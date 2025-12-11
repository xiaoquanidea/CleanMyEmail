package main

import (
	"context"
	"fmt"
	"log"
	"sync"
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

// OAuth2Session 存储单个 OAuth2 会话的状态
type OAuth2Session struct {
	Vendor    string
	Config    *oauth2.Config
	State     string
	AccountID int64  // 如果是重新授权，存储账号ID；新建账号时为0
	Email     string // 重新授权时的邮箱
}

// App struct
type App struct {
	ctx            context.Context
	accountService *account.Service
	historyService *service.HistoryService
	poolManager    *imap.PoolManager // 连接池管理器
	currentCleaner *cleaner.Cleaner
	// OAuth2 回调服务器（共享，支持多会话）
	callbackServer *oauth2.CallbackServer
	// OAuth2 会话管理（使用 state 作为 key）
	oauth2Sessions   map[string]*OAuth2Session
	oauth2SessionsMu sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		accountService: account.NewService(),
		historyService: service.NewHistoryService(),
		poolManager:    imap.NewPoolManager(),
		callbackServer: oauth2.NewCallbackServer(),
		oauth2Sessions: make(map[string]*OAuth2Session),
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
			log.Printf("[INFO] 已加载代理设置: %s", proxySettings.GetURL())
		}
	}
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	// 强制停止 OAuth2 回调服务器
	if a.callbackServer != nil {
		a.callbackServer.ForceStop()
	}
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

// GetAccountPassword 获取账号密码（仅密码认证类型）
func (a *App) GetAccountPassword(id int64) (string, error) {
	account, err := a.accountService.Get(id)
	if err != nil {
		return "", fmt.Errorf("获取账号失败: %w", err)
	}
	if account.AuthType.IsOAuth2() {
		return "", fmt.Errorf("OAuth2 账号没有密码")
	}
	return account.Password, nil
}

// DeleteAccount 删除账号
func (a *App) DeleteAccount(id int64) error {
	return a.accountService.Delete(id)
}

// UpdateAccountPassword 更新账号密码/授权码
func (a *App) UpdateAccountPassword(accountID int64, password string) error {
	account, err := a.accountService.Get(accountID)
	if err != nil {
		return fmt.Errorf("获取账号失败: %w", err)
	}

	// 先测试新密码是否有效
	testReq := &model.AccountCreateRequest{
		Email:      account.Email,
		Vendor:     account.Vendor,
		AuthType:   account.AuthType,
		Password:   password,
		IMAPServer: account.IMAPServer,
	}
	if err := a.accountService.TestConnection(testReq); err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}

	// 更新密码
	account.Password = password
	account.Status = model.AccountStatusActive
	if err := a.accountService.Update(account); err != nil {
		return fmt.Errorf("更新账号失败: %w", err)
	}

	return nil
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
		log.Printf("[WARN] 创建历史记录失败: %v", err)
	}

	// 使用连接池管理器获取连接池
	concurrency := req.GetMaxConcurrency()
	pool := a.poolManager.GetPool(req.AccountID, cfg, &imap.PoolOptions{
		MaxSize:     concurrency,
		IdleTimeout: 5 * time.Minute,
	})
	currentCleaner := cleaner.NewCleaner(pool)
	a.currentCleaner = currentCleaner

	// 启动进度监听
	go func() {
		for progress := range currentCleaner.ProgressChan() {
			wailsRuntime.EventsEmit(a.ctx, "clean:progress", progress)
		}
	}()

	// 异步执行清理（使用局部变量避免竞态）
	go func(hID int64, c *cleaner.Cleaner) {
		result, err := c.Clean(&req)
		if err != nil {
			// 更新历史记录为失败
			if hID > 0 {
				a.historyService.UpdateHistory(hID, 0, 0, "failed", err.Error(), 0)
			}
			wailsRuntime.EventsEmit(a.ctx, "clean:error", err.Error())
			return
		}
		// 更新历史记录为完成
		if hID > 0 {
			matchedCount := 0
			for _, stat := range result.FolderStats {
				matchedCount += stat.MatchedCount
			}
			a.historyService.UpdateHistory(hID, matchedCount, result.TotalDeleted, result.Status, "", result.Duration)
		}
		wailsRuntime.EventsEmit(a.ctx, "clean:complete", result)
	}(historyID, currentCleaner)

	return nil
}

// CancelClean 取消清理
func (a *App) CancelClean() {
	if a.currentCleaner != nil {
		a.currentCleaner.Cancel()
	}
}

// ==================== OAuth2 ====================

// OAuth2AuthResult OAuth2授权结果
type OAuth2AuthResult struct {
	AuthURL string `json:"authUrl"`
	State   string `json:"state"`
	Port    int    `json:"port"`
}

// startOAuth2Flow 内部方法：启动 OAuth2 流程（新建或重新授权共用）
func (a *App) startOAuth2Flow(vendor string, accountID int64, email string) (*OAuth2AuthResult, error) {
	// 获取OAuth2配置（只需要 ClientID）
	dbConfig, err := db.GetOAuth2Config(vendor)
	if err != nil || dbConfig == nil {
		return nil, fmt.Errorf("请先配置 %s 的 OAuth2 Client ID", vendor)
	}

	// 启动回调服务器（如果已运行则复用）
	port, err := a.callbackServer.Start()
	if err != nil {
		return nil, err
	}

	redirectURI := a.callbackServer.GetRedirectURI()
	log.Printf("[DEBUG] 回调服务器已启动, 端口: %d, redirectURI: %s", port, redirectURI)

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
		return nil, fmt.Errorf("不支持的OAuth2厂商: %s", vendor)
	}

	// 生成 state 并注册会话
	state := oauth2.GenerateState()
	log.Printf("[DEBUG] 生成 OAuth2 state: %s", state)

	// 保存会话（使用 state 作为 key）
	a.oauth2SessionsMu.Lock()
	a.oauth2Sessions[state] = &OAuth2Session{
		Vendor:    vendor,
		Config:    cfg,
		State:     state,
		AccountID: accountID, // 0 表示新建账号，>0 表示重新授权
		Email:     email,
	}
	a.oauth2SessionsMu.Unlock()

	// 在回调服务器中注册此会话
	a.callbackServer.RegisterSession(state)

	// 构建授权URL
	authURL := oauth2.BuildAuthURL(cfg, state)
	log.Printf("[DEBUG] 构建授权 URL, vendor: %s, authURL长度: %d", vendor, len(authURL))

	// 打开浏览器
	wailsRuntime.BrowserOpenURL(a.ctx, authURL)

	if accountID > 0 {
		log.Printf("[INFO] 开始 OAuth2 重新授权流程, vendor: %s, accountID: %d, state: %s, redirectURI: %s", vendor, accountID, state, redirectURI)
	} else {
		log.Printf("[INFO] 开始 OAuth2 授权流程, vendor: %s, state: %s, redirectURI: %s", vendor, state, redirectURI)
	}

	return &OAuth2AuthResult{
		AuthURL: authURL,
		State:   state,
		Port:    port,
	}, nil
}

// StartOAuth2Auth 开始OAuth2授权流程（新建账号）
func (a *App) StartOAuth2Auth(vendor string) (*OAuth2AuthResult, error) {
	return a.startOAuth2Flow(vendor, 0, "")
}

// StartOAuth2Reauth 开始OAuth2重新授权流程（更新现有账号的token）
func (a *App) StartOAuth2Reauth(accountID int64) (*OAuth2AuthResult, error) {
	// 获取账号信息
	account, err := db.GetAccountByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("获取账号失败: %w", err)
	}
	if account == nil {
		return nil, fmt.Errorf("账号不存在")
	}

	// 确定 vendor
	vendor := string(account.Vendor)
	if vendor != "gmail" && vendor != "outlook" {
		return nil, fmt.Errorf("该账号不支持 OAuth2 授权")
	}

	return a.startOAuth2Flow(vendor, accountID, account.Email)
}

// WaitOAuth2Callback 等待OAuth2回调并完成授权（需要传入 state 来匹配会话）
// 对于新建账号，需要传入 email；对于重新授权，email 参数会被忽略（使用 session 中保存的）
func (a *App) WaitOAuth2Callback(state, email string) (*model.EmailAccount, error) {
	log.Printf("[INFO] 开始等待 OAuth2 回调, state: %s, email: %s", state, email)

	// 清理函数
	defer func() {
		a.oauth2SessionsMu.Lock()
		delete(a.oauth2Sessions, state)
		a.oauth2SessionsMu.Unlock()
		a.callbackServer.UnregisterSession(state)
		a.callbackServer.Stop() // 如果没有其他会话，会停止服务器
		log.Printf("[DEBUG] 已清理 OAuth2 会话: %s", state)
	}()

	// 获取会话
	a.oauth2SessionsMu.RLock()
	session, ok := a.oauth2Sessions[state]
	a.oauth2SessionsMu.RUnlock()

	if !ok {
		log.Printf("[ERROR] OAuth2 会话不存在或已过期, state: %s", state)
		return nil, fmt.Errorf("OAuth2 授权流程未正确启动或已过期")
	}

	log.Printf("[DEBUG] 找到 OAuth2 会话, vendor: %s, state: %s", session.Vendor, state)

	// 等待回调（使用 state 匹配）
	log.Printf("[DEBUG] 等待回调结果, state: %s, 超时: 5分钟", state)
	result, err := a.callbackServer.WaitForCallback(state, 5*time.Minute)
	if err != nil {
		log.Printf("[ERROR] 等待回调失败: %v, state: %s", err, state)
		return nil, err
	}

	log.Printf("[DEBUG] 收到回调结果, state: %s, code长度: %d, error: %s", result.State, len(result.Code), result.Error)

	if result.Error != "" {
		log.Printf("[ERROR] OAuth2 授权错误: %s", result.Error)
		return nil, fmt.Errorf("授权失败: %s", result.Error)
	}

	// 验证 state 匹配
	if result.State != state {
		log.Printf("[ERROR] State 不匹配, 期望: %s, 实际: %s", state, result.State)
		return nil, fmt.Errorf("授权状态不匹配，可能存在安全问题")
	}

	// 用授权码换取Token（使用保存的配置，包含 PKCE code_verifier）
	log.Printf("[DEBUG] 开始用授权码交换 Token, vendor: %s, redirectURI: %s", session.Vendor, session.Config.RedirectURI)
	tokenResp, err := oauth2.ExchangeToken(context.Background(), session.Config, result.Code)
	if err != nil {
		log.Printf("[ERROR] Token 交换失败: %v, vendor: %s", err, session.Vendor)
		return nil, err
	}
	log.Printf("[DEBUG] Token 交换成功, accessToken长度: %d, refreshToken长度: %d", len(tokenResp.AccessToken), len(tokenResp.RefreshToken))

	var acct *model.EmailAccount
	var accountID int64

	// 判断是新建账号还是重新授权
	if session.AccountID > 0 {
		// 重新授权：更新现有账号的 token
		accountID = session.AccountID
		acct, err = db.GetAccountByID(accountID)
		if err != nil {
			return nil, fmt.Errorf("获取账号失败: %w", err)
		}
		if acct == nil {
			return nil, fmt.Errorf("账号不存在")
		}
		// 更新账号状态为活跃
		acct.Status = model.AccountStatusActive
		if err := db.UpdateAccountStatus(accountID, model.AccountStatusActive); err != nil {
			log.Printf("[WARN] 更新账号状态失败: %v", err)
		}
		log.Printf("[INFO] OAuth2 重新授权成功, vendor: %s, email: %s", session.Vendor, acct.Email)
	} else {
		// 新建账号：先检查邮箱是否已存在
		existingAccount, _ := db.GetAccountByEmail(email)
		if existingAccount != nil {
			return nil, fmt.Errorf("该邮箱账号已存在，如需重新授权请在首页点击重新授权按钮")
		}

		vendorType := model.EmailVendorType(session.Vendor)
		acct = &model.EmailAccount{
			Email:      email,
			Vendor:     vendorType,
			AuthType:   model.EmailAuthTypeOAuth2,
			IMAPServer: vendorType.GetDefaultIMAPServer(),
			Status:     model.AccountStatusActive,
		}

		accountID, err = db.CreateAccount(acct)
		if err != nil {
			return nil, fmt.Errorf("创建账号失败: %w", err)
		}
		acct.ID = accountID
		log.Printf("[INFO] OAuth2 授权成功, vendor: %s, email: %s", session.Vendor, email)
	}

	// 保存/更新 Token
	expiresAt := tokenResp.GetExpiresAt()
	token := &model.OAuth2Token{
		AccountID:    accountID,
		Provider:     session.Vendor,
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

// CancelOAuth2Auth 取消指定的OAuth2授权
func (a *App) CancelOAuth2Auth(state string) {
	a.oauth2SessionsMu.Lock()
	delete(a.oauth2Sessions, state)
	a.oauth2SessionsMu.Unlock()
	a.callbackServer.UnregisterSession(state)
	a.callbackServer.Stop()
	log.Printf("[INFO] 取消 OAuth2 授权, state: %s", state)
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

// GetVersion 获取版本号字符串
func (a *App) GetVersion() string {
	return Version
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
		log.Printf("[INFO] 代理设置已更新: %s", settings.GetURL())
	} else {
		log.Printf("[INFO] 代理已禁用")
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
