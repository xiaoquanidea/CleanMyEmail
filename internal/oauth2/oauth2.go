package oauth2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// VendorType OAuth2厂商类型
type VendorType string

const (
	VendorGoogle    VendorType = "google"
	VendorMicrosoft VendorType = "microsoft"
)

// Config OAuth2配置
type Config struct {
	Vendor       VendorType // 厂商类型
	ClientID     string
	ClientSecret string // Google 桌面应用需要 client_secret
	AuthURL      string
	TokenURL     string
	Scopes       []string
	RedirectURI  string
	// PKCE 相关
	CodeVerifier  string
	CodeChallenge string
}

// TokenResponse Token响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// GetExpiresAt 计算过期时间
func (t *TokenResponse) GetExpiresAt() time.Time {
	if t.ExpiresIn > 0 {
		return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	}
	return time.Now().Add(time.Hour)
}

// generatePKCE 生成 PKCE code_verifier 和 code_challenge
func generatePKCE() (verifier, challenge string) {
	// 生成 43-128 字符的随机字符串作为 code_verifier
	b := make([]byte, 32)
	rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)

	// 计算 code_challenge = BASE64URL(SHA256(code_verifier))
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

// GmailConfig 获取Gmail OAuth2配置
// Google 桌面应用需要 client_secret（与 Web 应用不同，桌面应用的 secret 是公开的）
func GmailConfig(clientID, clientSecret, redirectURI string) *Config {
	verifier, challenge := generatePKCE()
	return &Config{
		Vendor:        VendorGoogle,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		AuthURL:       "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:      "https://oauth2.googleapis.com/token",
		Scopes:        []string{"https://mail.google.com/", "openid", "email"},
		RedirectURI:   redirectURI,
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
	}
}

// OutlookConfig 获取Outlook OAuth2配置（使用PKCE，无需Client Secret）
func OutlookConfig(clientID, redirectURI string) *Config {
	verifier, challenge := generatePKCE()
	return &Config{
		Vendor:        VendorMicrosoft,
		ClientID:      clientID,
		AuthURL:       "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
		TokenURL:      "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
		Scopes:        []string{"https://outlook.office.com/IMAP.AccessAsUser.All", "offline_access", "openid", "email"},
		RedirectURI:   redirectURI,
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
	}
}

// GenerateState 生成随机state
func GenerateState() string {
	return uuid.New().String()
}

// BuildAuthURL 构建授权URL（使用PKCE）
func BuildAuthURL(cfg *Config, state string) string {
	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", cfg.RedirectURI)
	params.Set("scope", strings.Join(cfg.Scopes, " "))
	params.Set("state", state)
	params.Set("code_challenge", cfg.CodeChallenge)
	params.Set("code_challenge_method", "S256")

	// 根据厂商类型设置特定参数
	cfg.setVendorSpecificParams(params)

	return cfg.AuthURL + "?" + params.Encode()
}

// setVendorSpecificParams 设置厂商特定的授权参数
func (cfg *Config) setVendorSpecificParams(params url.Values) {
	switch cfg.Vendor {
	case VendorMicrosoft:
		// Microsoft: 强制显示账号选择页面，避免自动使用已登录账号
		params.Set("prompt", "select_account")
	case VendorGoogle:
		// Google: 需要 access_type=offline 和 prompt=consent 才能获取 refresh_token
		params.Set("access_type", "offline")
		params.Set("prompt", "consent")
	}
}

// ExchangeToken 用授权码换取Token
func ExchangeToken(ctx context.Context, cfg *Config, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", cfg.ClientID)
	data.Set("code", code)
	data.Set("redirect_uri", cfg.RedirectURI)
	// PKCE: 提供 code_verifier
	data.Set("code_verifier", cfg.CodeVerifier)
	// Google 桌面应用需要 client_secret
	if cfg.ClientSecret != "" {
		data.Set("client_secret", cfg.ClientSecret)
	}

	return requestToken(ctx, cfg.TokenURL, data)
}

// RefreshToken 刷新Token
func RefreshToken(ctx context.Context, cfg *Config, refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", cfg.ClientID)
	data.Set("refresh_token", refreshToken)
	// Google 需要 client_secret
	if cfg.ClientSecret != "" {
		data.Set("client_secret", cfg.ClientSecret)
	}

	return requestToken(ctx, cfg.TokenURL, data)
}

// httpClient 禁用连接复用，避免 "Unsolicited response" 警告
var httpClient = &http.Client{
	Transport: &http.Transport{
		DisableKeepAlives: true,
	},
	Timeout: 30 * time.Second,
}

func requestToken(ctx context.Context, tokenURL string, data url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求Token失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析Token响应失败: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("OAuth2错误: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("未获取到access_token")
	}

	if tokenResp.TokenType == "" {
		tokenResp.TokenType = "Bearer"
	}

	return &tokenResp, nil
}

