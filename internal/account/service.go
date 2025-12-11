package account

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"CleanMyEmail/internal/db"
	"CleanMyEmail/internal/email/imap"
	"CleanMyEmail/internal/model"
	"CleanMyEmail/internal/oauth2"
)

// Service 账号服务
type Service struct {
	// tokenRefreshMu 用于防止同一账号的 Token 被多个 goroutine 同时刷新
	tokenRefreshMu sync.Map // map[int64]*sync.Mutex
}

// NewService 创建账号服务
func NewService() *Service {
	return &Service{}
}

// getAccountMutex 获取指定账号的互斥锁
func (s *Service) getAccountMutex(accountID int64) *sync.Mutex {
	mu, _ := s.tokenRefreshMu.LoadOrStore(accountID, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// Create 创建账号
func (s *Service) Create(req *model.AccountCreateRequest) (*model.EmailAccount, error) {
	// 检查邮箱是否已存在
	existing, _ := db.GetAccountByEmail(req.Email)
	if existing != nil {
		return nil, fmt.Errorf("邮箱 %s 已存在", req.Email)
	}

	// 设置默认IMAP服务器
	imapServer := req.IMAPServer
	if imapServer == "" {
		imapServer = req.Vendor.GetDefaultIMAPServer()
	}
	if imapServer == "" {
		return nil, fmt.Errorf("请指定IMAP服务器地址")
	}

	account := &model.EmailAccount{
		Email:      req.Email,
		Vendor:     req.Vendor,
		AuthType:   req.AuthType,
		IMAPServer: imapServer,
		Password:   req.Password,
		Status:     model.AccountStatusActive,
	}

	id, err := db.CreateAccount(account)
	if err != nil {
		return nil, fmt.Errorf("创建账号失败: %w", err)
	}

	account.ID = id
	return account, nil
}

// TestConnection 测试连接
func (s *Service) TestConnection(req *model.AccountCreateRequest) error {
	imapServer := req.IMAPServer
	if imapServer == "" {
		imapServer = req.Vendor.GetDefaultIMAPServer()
	}
	if imapServer == "" {
		return fmt.Errorf("请指定IMAP服务器地址")
	}

	cfg := &imap.ConnectConfig{
		Server:   imapServer,
		Username: req.Email,
		Password: req.Password,
		AuthType: req.AuthType,
	}

	return imap.TestConnection(cfg)
}

// TestConnectionByID 根据账号ID测试连接
func (s *Service) TestConnectionByID(accountID int64) error {
	account, err := db.GetAccountByID(accountID)
	if err != nil {
		return fmt.Errorf("账号不存在: %w", err)
	}

	cfg, err := s.buildConnectConfig(account)
	if err != nil {
		db.UpdateAccountStatus(accountID, model.AccountStatusDisconnected)
		return err
	}
	if err := imap.TestConnection(cfg); err != nil {
		db.UpdateAccountStatus(accountID, model.AccountStatusDisconnected)
		return err
	}

	db.UpdateAccountStatus(accountID, model.AccountStatusActive)
	db.UpdateAccountLastConnected(accountID)
	return nil
}

// List 获取账号列表
func (s *Service) List() ([]*model.AccountListItem, error) {
	accounts, err := db.ListAccounts()
	if err != nil {
		return nil, err
	}

	// 检查 OAuth2 账号的 token 状态
	for _, account := range accounts {
		if account.AuthType.IsOAuth2() {
			warning := s.checkTokenWarning(account)
			if warning != "" {
				account.TokenWarning = warning
			}
		}
	}

	return accounts, nil
}

// checkTokenWarning 检查 token 警告状态
func (s *Service) checkTokenWarning(account *model.AccountListItem) string {
	token, err := db.GetTokenByAccountID(account.ID)
	if err != nil || token == nil {
		return ""
	}

	// 检查 token 状态
	if token.AuthStatus == model.OAuth2StatusExpired {
		return "Token已过期，请重新授权"
	}

	// 检查 Outlook 的 refresh token 是否即将过期（90天有效期，提前7天警告）
	if account.Vendor == model.EmailVendorOutlook {
		lifetime := account.Vendor.GetRefreshTokenLifetime()
		if lifetime > 0 && token.CreatedAt.Add(lifetime).Before(time.Now().Add(7*24*time.Hour)) {
			daysLeft := int(time.Until(token.CreatedAt.Add(lifetime)).Hours() / 24)
			if daysLeft <= 0 {
				return "Refresh Token已过期，请重新授权"
			}
			return fmt.Sprintf("Refresh Token将在%d天后过期，建议重新授权", daysLeft)
		}
	}

	return ""
}

// Get 获取账号详情
func (s *Service) Get(id int64) (*model.EmailAccount, error) {
	return db.GetAccountByID(id)
}

// Update 更新账号
func (s *Service) Update(account *model.EmailAccount) error {
	return db.UpdateAccount(account)
}

// Delete 删除账号
func (s *Service) Delete(id int64) error {
	// 先删除关联的token
	db.DeleteTokenByAccountID(id)
	return db.DeleteAccount(id)
}

// buildConnectConfig 构建连接配置
func (s *Service) buildConnectConfig(account *model.EmailAccount) (*imap.ConnectConfig, error) {
	cfg := &imap.ConnectConfig{
		Server:   account.IMAPServer,
		Username: account.Email,
		Password: account.Password,
		AuthType: account.AuthType,
	}

	// 如果是OAuth2，需要获取并可能刷新access token
	if account.AuthType.IsOAuth2() {
		accessToken, err := s.getOrRefreshAccessToken(account)
		if err != nil {
			return nil, fmt.Errorf("获取OAuth2 Token失败: %w", err)
		}
		cfg.AccessToken = accessToken
		// 设置 token 刷新器，用于长时间任务中自动刷新 token
		cfg.TokenRefresher = func() (string, error) {
			return s.getOrRefreshAccessToken(account)
		}
	}

	return cfg, nil
}

// getOrRefreshAccessToken 获取或刷新 access token
// 使用互斥锁防止同一账号的 Token 被多个 goroutine 同时刷新
func (s *Service) getOrRefreshAccessToken(account *model.EmailAccount) (string, error) {
	// 先检查是否需要刷新（无锁快速路径）
	token, err := db.GetTokenByAccountID(account.ID)
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", fmt.Errorf("未找到OAuth2 Token，请重新授权")
	}

	// 如果 Token 未过期，直接返回（无需加锁）
	if token.ExpiresAt != nil && time.Until(*token.ExpiresAt) > 5*time.Minute {
		return token.AccessToken, nil
	}

	// 需要刷新，获取账号级别的互斥锁
	mu := s.getAccountMutex(account.ID)
	mu.Lock()
	defer mu.Unlock()

	// 双重检查：获取锁后再次检查，可能其他 goroutine 已经刷新了
	token, err = db.GetTokenByAccountID(account.ID)
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", fmt.Errorf("未找到OAuth2 Token，请重新授权")
	}

	// 再次检查是否需要刷新
	if token.ExpiresAt != nil && time.Until(*token.ExpiresAt) > 5*time.Minute {
		log.Printf("[DEBUG] Token 已被其他 goroutine 刷新, accountID: %d", account.ID)
		return token.AccessToken, nil
	}

	// 确实需要刷新
	if token.RefreshToken == "" {
		db.UpdateTokenStatus(account.ID, model.OAuth2StatusExpired, "Refresh token不存在，需要重新授权")
		return "", fmt.Errorf("Token已过期，请重新授权")
	}

	log.Printf("[DEBUG] 开始刷新 Token, accountID: %d, provider: %s", account.ID, token.Provider)

	// 获取OAuth2配置
	dbConfig, err := db.GetOAuth2Config(token.Provider)
	if err != nil || dbConfig == nil {
		return "", fmt.Errorf("OAuth2配置不存在")
	}

	// 根据厂商获取配置（刷新时不需要 PKCE）
	var cfg *oauth2.Config
	switch token.Provider {
	case "gmail":
		// Google 刷新 Token 需要 client_secret
		cfg = oauth2.GmailConfig(dbConfig.ClientID, dbConfig.ClientSecret, "")
	case "outlook":
		cfg = oauth2.OutlookConfig(dbConfig.ClientID, "")
	default:
		return "", fmt.Errorf("不支持的OAuth2厂商: %s", token.Provider)
	}

	// 刷新Token
	tokenResp, err := oauth2.RefreshToken(context.Background(), cfg, token.RefreshToken)
	if err != nil {
		db.UpdateTokenStatus(account.ID, model.OAuth2StatusExpired, err.Error())
		return "", fmt.Errorf("刷新Token失败: %w", err)
	}

	// 更新Token
	expiresAt := tokenResp.GetExpiresAt()
	token.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		token.RefreshToken = tokenResp.RefreshToken
	}
	token.ExpiresAt = &expiresAt
	token.AuthStatus = model.OAuth2StatusAuthorized
	token.ErrorMessage = ""

	if err := db.SaveToken(token); err != nil {
		return "", err
	}

	log.Printf("[INFO] Token 刷新成功, accountID: %d", account.ID)
	return token.AccessToken, nil
}

// GetConnectConfig 获取连接配置
func (s *Service) GetConnectConfig(accountID int64) (*imap.ConnectConfig, error) {
	account, err := db.GetAccountByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("账号不存在: %w", err)
	}
	return s.buildConnectConfig(account)
}
