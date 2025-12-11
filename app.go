package main

import (
	"context"
	"fmt"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"CleanMyEmail/internal/account"
	"CleanMyEmail/internal/db"
	"CleanMyEmail/internal/email/cleaner"
	"CleanMyEmail/internal/email/folder"
	"CleanMyEmail/internal/email/imap"
	"CleanMyEmail/internal/model"
	"CleanMyEmail/internal/oauth2"
	"CleanMyEmail/internal/proxy"
	"CleanMyEmail/internal/service"
)

// App struct
type App struct {
	ctx            context.Context
	accountService *account.Service
	historyService *service.HistoryService
	poolManager    *imap.PoolManager // 连接池管理器
	currentCleaner *cleaner.Cleaner
	currentHistoryID int64 // 当前清理任务的历史记录ID
	// OAuth2 回调服务器
	callbackServer    *oauth2.CallbackServer
	pendingVendor     string
	pendingOAuth2Cfg  *oauth2.Config // 保存 PKCE 的 code_verifier
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		accountService: account.NewService(),
		historyService: service.NewHistoryService(),
		poolManager:    imap.NewPoolManager(),
		callbackServer: oauth2.NewCallbackServer(),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 初始化数据库
	if _, err := db.GetDB(); err != nil {
		wailsRuntime.LogError(ctx, fmt.Sprintf("初始化数据库失败: %v", err))
	}
	// 加载代理设置
	if proxySettings, err := db.GetProxySettings(); err == nil && proxySettings != nil {
		proxy.SetGlobalProxy(proxySettings)
		if proxySettings.Enabled {
			fmt.Printf("[INFO] 已加载代理设置: %s\n", proxySettings.GetURL())
		}
	}
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	// 关闭连接池管理器
	if a.poolManager != nil {
		a.poolManager.Close()
	}
	db.Close()
}

// ==================== 账号管理 ====================

// GetVendorList 获取支持的邮箱厂商列表
func (a *App) GetVendorList() []model.VendorInfo {
	return model.GetVendorList()
}

// CreateAccount 创建账号
func (a *App) CreateAccount(req model.AccountCreateRequest) (*model.EmailAccount, error) {
	return a.accountService.Create(&req)
}

// TestConnection 测试连接
func (a *App) TestConnection(req model.AccountCreateRequest) error {
	return a.accountService.TestConnection(&req)
}

// ListAccounts 获取账号列表
func (a *App) ListAccounts() ([]*model.AccountListItem, error) {
	return a.accountService.List()
}

// GetAccount 获取账号详情
func (a *App) GetAccount(id int64) (*model.EmailAccount, error) {
	return a.accountService.Get(id)
}

// DeleteAccount 删除账号
func (a *App) DeleteAccount(id int64) error {
	return a.accountService.Delete(id)
}

// ==================== 文件夹管理 ====================

// GetFolderTree 获取文件夹树
func (a *App) GetFolderTree(accountID int64) ([]*model.FolderTreeNode, error) {
	cfg, err := a.accountService.GetConnectConfig(accountID)
	if err != nil {
		return nil, err
	}

	// 使用连接池管理器获取连接池
	pool := a.poolManager.GetPool(accountID, cfg, nil)
	conn, err := pool.Get(context.Background())
	if err != nil {
		return nil, fmt.Errorf("连接邮箱失败: %w", err)
	}

	// 检查是否支持 LIST-STATUS
	supportsListStatus := imap.SupportsListStatus(conn.Client())

	folders, err := imap.ListMailboxes(conn.Client())
	if err != nil {
		conn.MarkBad()
		conn.Release()
		return nil, err
	}

	// 更新最后连接时间
	db.UpdateAccountLastConnected(accountID)
	db.UpdateAccountStatus(accountID, model.AccountStatusActive)

	tree := folder.BuildFolderTree(folders)

	// 如果不支持 LIST-STATUS，异步获取邮件数量
	if !supportsListStatus {
		go func() {
			defer conn.Release()
			imap.FetchFolderStatus(conn.Client(), folders, func(update imap.FolderStatusUpdate) {
				wailsRuntime.EventsEmit(a.ctx, "folder:status", update)
			})
		}()
	} else {
		conn.Release()
	}

	return tree, nil
}

// ==================== 邮件清理 ====================

// StartClean 开始清理
func (a *App) StartClean(req model.CleanRequest) error {
	cfg, err := a.accountService.GetConnectConfig(req.AccountID)
	if err != nil {
		return err
	}

	// 获取账号邮箱
	acc, err := a.accountService.Get(req.AccountID)
	if err != nil {
		return err
	}

	// 创建历史记录
	historyID, err := a.historyService.CreateHistory(&req, acc.Email)
	if err != nil {
		fmt.Printf("[WARN] 创建历史记录失败: %v\n", err)
	}
	a.currentHistoryID = historyID

	// 使用连接池管理器获取连接池
	concurrency := req.GetMaxConcurrency()
	pool := a.poolManager.GetPool(req.AccountID, cfg, &imap.PoolOptions{
		MaxSize:     concurrency,
		IdleTimeout: 5 * time.Minute,
	})
	a.currentCleaner = cleaner.NewCleaner(pool)

	// 启动进度监听
	go func() {
		for progress := range a.currentCleaner.ProgressChan() {
			wailsRuntime.EventsEmit(a.ctx, "clean:progress", progress)
		}
	}()

	// 异步执行清理
	go func() {
		result, err := a.currentCleaner.Clean(&req)
		if err != nil {
			// 更新历史记录为失败
			if a.currentHistoryID > 0 {
				a.historyService.UpdateHistory(a.currentHistoryID, 0, 0, "failed", err.Error(), 0)
			}
			wailsRuntime.EventsEmit(a.ctx, "clean:error", err.Error())
			return
		}
		// 更新历史记录为完成
		if a.currentHistoryID > 0 {
			matchedCount := 0
			for _, stat := range result.FolderStats {
				matchedCount += stat.MatchedCount
			}
			a.historyService.UpdateHistory(a.currentHistoryID, matchedCount, result.TotalDeleted, result.Status, "", result.Duration)
		}
		wailsRuntime.EventsEmit(a.ctx, "clean:complete", result)
	}()

	return nil
}

// CancelClean 取消清理
func (a *App) CancelClean() {
	if a.currentCleaner != nil {
		a.currentCleaner.Cancel()
	}
	// 更新历史记录为取消
	if a.currentHistoryID > 0 {
		a.historyService.UpdateHistory(a.currentHistoryID, 0, 0, "cancelled", "用户取消", 0)
	}
}

// ==================== OAuth2 ====================

// OAuth2AuthResult OAuth2授权结果
type OAuth2AuthResult struct {
	AuthURL string `json:"authUrl"`
	State   string `json:"state"`
	Port    int    `json:"port"`
}

// StartOAuth2Auth 开始OAuth2授权流程
func (a *App) StartOAuth2Auth(vendor string) (*OAuth2AuthResult, error) {
	// 获取OAuth2配置（只需要 ClientID）
	dbConfig, err := db.GetOAuth2Config(vendor)
	if err != nil || dbConfig == nil {
		return nil, fmt.Errorf("请先配置 %s 的 OAuth2 Client ID", vendor)
	}

	// 启动回调服务器
	port, err := a.callbackServer.Start()
	if err != nil {
		return nil, err
	}

	redirectURI := a.callbackServer.GetRedirectURI()

	// 根据厂商获取OAuth2配置
	var cfg *oauth2.Config
	switch vendor {
	case "gmail":
		// Google 桌面应用需要 client_secret
		cfg = oauth2.GmailConfig(dbConfig.ClientID, dbConfig.ClientSecret, redirectURI)
	case "outlook":
		// Microsoft 使用 PKCE，不需要 client_secret
		cfg = oauth2.OutlookConfig(dbConfig.ClientID, redirectURI)
	default:
		a.callbackServer.Stop()
		return nil, fmt.Errorf("不支持的OAuth2厂商: %s", vendor)
	}

	// 保存配置（包含 PKCE code_verifier，用于后续 ExchangeToken）
	a.pendingOAuth2Cfg = cfg
	a.pendingVendor = vendor

	// 生成state并构建授权URL
	state := oauth2.GenerateState()
	authURL := oauth2.BuildAuthURL(cfg, state)

	// 打开浏览器
	wailsRuntime.BrowserOpenURL(a.ctx, authURL)

	return &OAuth2AuthResult{
		AuthURL: authURL,
		State:   state,
		Port:    port,
	}, nil
}

// WaitOAuth2Callback 等待OAuth2回调并完成授权
func (a *App) WaitOAuth2Callback(vendor, email string) (*model.EmailAccount, error) {
	defer func() {
		a.callbackServer.Stop()
		a.pendingOAuth2Cfg = nil
		a.pendingVendor = ""
	}()

	// 检查是否有保存的配置（包含 PKCE code_verifier）
	if a.pendingOAuth2Cfg == nil {
		return nil, fmt.Errorf("OAuth2 授权流程未正确启动")
	}

	// 等待回调
	result, err := a.callbackServer.WaitForCallback(5 * time.Minute)
	if err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf("授权失败: %s", result.Error)
	}

	// 用授权码换取Token（使用保存的配置，包含 PKCE code_verifier）
	tokenResp, err := oauth2.ExchangeToken(context.Background(), a.pendingOAuth2Cfg, result.Code)
	if err != nil {
		return nil, err
	}

	// 创建账号
	vendorType := model.EmailVendorType(vendor)
	acct := &model.EmailAccount{
		Email:      email,
		Vendor:     vendorType,
		AuthType:   model.EmailAuthTypeOAuth2,
		IMAPServer: vendorType.GetDefaultIMAPServer(),
		Status:     model.AccountStatusActive,
	}

	accountID, err := db.CreateAccount(acct)
	if err != nil {
		return nil, fmt.Errorf("创建账号失败: %w", err)
	}
	acct.ID = accountID

	// 保存Token
	expiresAt := tokenResp.GetExpiresAt()
	token := &model.OAuth2Token{
		AccountID:    accountID,
		Provider:     vendor,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    &expiresAt,
		AuthStatus:   model.OAuth2StatusAuthorized,
	}
	if err := db.SaveToken(token); err != nil {
		return nil, fmt.Errorf("保存Token失败: %w", err)
	}

	return acct, nil
}

// CancelOAuth2Auth 取消OAuth2授权
func (a *App) CancelOAuth2Auth() {
	a.callbackServer.Stop()
	a.pendingOAuth2Cfg = nil
	a.pendingVendor = ""
}

// OAuth2Config OAuth2配置
type OAuth2Config struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// SaveOAuth2Config 保存OAuth2配置
func (a *App) SaveOAuth2Config(vendor, clientID, clientSecret string) error {
	return db.SaveOAuth2Config(vendor, clientID, clientSecret)
}

// GetOAuth2Config 获取OAuth2配置（前端用）
func (a *App) GetOAuth2Config(vendor string) (*OAuth2Config, error) {
	config, err := db.GetOAuth2Config(vendor)
	if err != nil || config == nil {
		return nil, nil // 返回nil表示未配置
	}
	return &OAuth2Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
	}, nil
}

// ==================== 工具方法 ====================

// AppVersionInfo 应用版本信息
type AppVersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
}

// GetAppVersion 获取应用版本
func (a *App) GetAppVersion() *AppVersionInfo {
	return &AppVersionInfo{
		Version:   Version,
		BuildTime: BuildTime,
	}
}

// GetCurrentTime 获取当前时间（用于前端默认值）
func (a *App) GetCurrentTime() time.Time {
	return time.Now()
}

// ==================== 代理设置 ====================

// GetProxySettings 获取代理设置
func (a *App) GetProxySettings() (*model.ProxySettings, error) {
	return db.GetProxySettings()
}

// SaveProxySettings 保存代理设置
func (a *App) SaveProxySettings(settings *model.ProxySettings) error {
	if err := db.SaveProxySettings(settings); err != nil {
		return err
	}
	// 更新全局代理设置
	proxy.SetGlobalProxy(settings)
	if settings.Enabled {
		fmt.Printf("[INFO] 代理设置已更新: %s\n", settings.GetURL())
	} else {
		fmt.Printf("[INFO] 代理已禁用\n")
	}
	return nil
}

// TestProxy 测试代理连接
func (a *App) TestProxy(settings *model.ProxySettings) error {
	// 临时设置代理
	oldSettings := proxy.GetGlobalProxy()
	proxy.SetGlobalProxy(settings)
	defer proxy.SetGlobalProxy(oldSettings)

	// 尝试连接 Google
	conn, err := proxy.Dial("tcp", "www.google.com:443", 10*time.Second)
	if err != nil {
		return fmt.Errorf("代理连接测试失败: %w", err)
	}
	conn.Close()
	return nil
}

// ==================== 历史记录 ====================

// GetCleanHistoryList 获取清理历史列表
func (a *App) GetCleanHistoryList(limit, offset int) ([]model.CleanHistoryListItem, error) {
	if limit <= 0 {
		limit = 20
	}
	return a.historyService.GetHistoryList(limit, offset)
}

// GetCleanHistoryDetail 获取清理历史详情
func (a *App) GetCleanHistoryDetail(id int64) (*model.CleanHistory, error) {
	return a.historyService.GetHistoryDetail(id)
}

// DeleteCleanHistory 删除清理历史
func (a *App) DeleteCleanHistory(id int64) error {
	return a.historyService.DeleteHistory(id)
}

// ClearAllCleanHistory 清空所有清理历史
func (a *App) ClearAllCleanHistory() error {
	return a.historyService.ClearAllHistory()
}
