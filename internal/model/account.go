package model

import "time"

// EmailAccount 邮箱账号
type EmailAccount struct {
	ID            int64           `json:"id"`
	Email         string          `json:"email"`
	DisplayName   string          `json:"displayName"`
	Vendor        EmailVendorType `json:"vendor"`
	AuthType      EmailAuthType   `json:"authType"`
	IMAPServer    string          `json:"imapServer"`
	Password      string          `json:"-"` // 不序列化到JSON
	Status        AccountStatus   `json:"status"`
	LastConnected *time.Time      `json:"lastConnected"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

// OAuth2Token OAuth2令牌
type OAuth2Token struct {
	ID           int64            `json:"id"`
	AccountID    int64            `json:"accountId"`
	Provider     string           `json:"provider"`
	AccessToken  string           `json:"-"`
	RefreshToken string           `json:"-"`
	TokenType    string           `json:"tokenType"`
	ExpiresAt    *time.Time       `json:"expiresAt"`
	AuthStatus   OAuth2AuthStatus `json:"authStatus"`
	ErrorMessage string           `json:"errorMessage"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

// AccountCreateRequest 创建账号请求
type AccountCreateRequest struct {
	Email      string          `json:"email"`
	Vendor     EmailVendorType `json:"vendor"`
	AuthType   EmailAuthType   `json:"authType"`
	Password   string          `json:"password"`
	IMAPServer string          `json:"imapServer"`
}

// AccountListItem 账号列表项（用于前端展示）
type AccountListItem struct {
	ID            int64           `json:"id"`
	Email         string          `json:"email"`
	DisplayName   string          `json:"displayName"`
	Vendor        EmailVendorType `json:"vendor"`
	AuthType      EmailAuthType   `json:"authType"`
	Status        AccountStatus   `json:"status"`
	LastConnected *time.Time      `json:"lastConnected"`
}

// VendorInfo 厂商信息
type VendorInfo struct {
	Vendor        EmailVendorType `json:"vendor"`
	Name          string          `json:"name"`
	Icon          string          `json:"icon"`
	IMAPServer    string          `json:"imapServer"`
	SupportsOAuth bool            `json:"supportsOAuth"`
}

// GetVendorList 获取支持的厂商列表
func GetVendorList() []VendorInfo {
	return []VendorInfo{
		{EmailVendorNE163Enterprise, "网易163企业邮箱", "netease", "imaphz.qiye.163.com:993", false},
		{EmailVendorNE163Personal, "网易163个人邮箱", "netease", "imap.163.com:993", false},
		{EmailVendorNE126, "网易126邮箱", "netease", "imap.126.com:993", false},
		{EmailVendorQQ, "QQ邮箱", "qq", "imap.qq.com:993", false},
		{EmailVendorAliyun, "阿里邮箱", "aliyun", "imap.qiye.aliyun.com:993", false},
		{EmailVendorGmail, "Gmail", "gmail", "imap.gmail.com:993", true},
		{EmailVendorOutlook, "Outlook", "outlook", "outlook.office365.com:993", true},
		{EmailVendorOther, "其他邮箱", "other", "", false},
	}
}

// GetVendorIcon 根据厂商类型获取图标名称
func GetVendorIcon(vendor EmailVendorType) string {
	switch vendor {
	case EmailVendorGmail:
		return "gmail"
	case EmailVendorOutlook:
		return "outlook"
	case EmailVendorQQ:
		return "qq"
	case EmailVendorNE163Personal, EmailVendorNE163Enterprise, EmailVendorNE126:
		return "netease"
	case EmailVendorAliyun:
		return "aliyun"
	default:
		return "other"
	}
}
