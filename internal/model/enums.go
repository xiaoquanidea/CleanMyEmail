package model

import "time"

// EmailVendorType 邮箱厂商类型
type EmailVendorType string

const (
	EmailVendorOther           EmailVendorType = "other"
	EmailVendorNE163Personal   EmailVendorType = "163-personal"
	EmailVendorNE163Enterprise EmailVendorType = "163-enterprise"
	EmailVendorNE126           EmailVendorType = "126"
	EmailVendorQQ              EmailVendorType = "qq"
	EmailVendorGmail           EmailVendorType = "gmail"
	EmailVendorOutlook         EmailVendorType = "outlook"
	EmailVendorAliyun          EmailVendorType = "aliyun"
)

func (e EmailVendorType) String() string {
	return string(e)
}

// GetDefaultIMAPServer 获取默认IMAP服务器
func (e EmailVendorType) GetDefaultIMAPServer() string {
	switch e {
	case EmailVendorNE163Personal:
		return "imap.163.com:993"
	case EmailVendorNE163Enterprise:
		return "imaphz.qiye.163.com:993"
	case EmailVendorNE126:
		return "imap.126.com:993"
	case EmailVendorQQ:
		return "imap.qq.com:993"
	case EmailVendorGmail:
		return "imap.gmail.com:993"
	case EmailVendorOutlook:
		return "outlook.office365.com:993"
	default:
		return ""
	}
}

// SupportsOAuth2 是否支持OAuth2
func (e EmailVendorType) SupportsOAuth2() bool {
	switch e {
	case EmailVendorGmail, EmailVendorOutlook:
		return true
	default:
		return false
	}
}

// GetRefreshTokenLifetime 获取RefreshToken有效期
func (e EmailVendorType) GetRefreshTokenLifetime() time.Duration {
	switch e {
	case EmailVendorOutlook:
		return 90 * 24 * time.Hour
	case EmailVendorGmail:
		return 0 // 永不过期
	default:
		return 90 * 24 * time.Hour
	}
}

// EmailAuthType 认证类型
type EmailAuthType string

const (
	EmailAuthTypePassword          EmailAuthType = "password"
	EmailAuthTypeOAuth2            EmailAuthType = "oauth2"
	EmailAuthTypeOAuth2AuthCode    EmailAuthType = "oauth2-auth-code"
	EmailAuthTypeOAuth2ClientCreds EmailAuthType = "oauth2-client-creds"
)

func (e EmailAuthType) String() string {
	return string(e)
}

func (e EmailAuthType) IsOAuth2() bool {
	return e == EmailAuthTypeOAuth2 || e == EmailAuthTypeOAuth2AuthCode || e == EmailAuthTypeOAuth2ClientCreds
}

// AccountStatus 账号状态
type AccountStatus string

const (
	AccountStatusActive       AccountStatus = "active"
	AccountStatusDisconnected AccountStatus = "disconnected"
	AccountStatusAuthRequired AccountStatus = "auth_required"
)

// OAuth2AuthStatus OAuth2认证状态
type OAuth2AuthStatus string

const (
	OAuth2StatusAuthorized  OAuth2AuthStatus = "authorized"
	OAuth2AuthStatusActive  OAuth2AuthStatus = "active"
	OAuth2StatusExpired     OAuth2AuthStatus = "expired"
	OAuth2AuthStatusExpired OAuth2AuthStatus = "expired"
	OAuth2AuthStatusError   OAuth2AuthStatus = "error"
)
