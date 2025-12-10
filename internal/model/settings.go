package model

// ProxyType 代理类型
type ProxyType string

const (
	ProxyTypeNone   ProxyType = "none"   // 无代理（直连）
	ProxyTypeSocks5 ProxyType = "socks5" // SOCKS5 代理
	ProxyTypeHTTP   ProxyType = "http"   // HTTP 代理
)

// ProxySettings 代理设置
type ProxySettings struct {
	Type    ProxyType `json:"type"`    // 代理类型
	Host    string    `json:"host"`    // 代理主机
	Port    int       `json:"port"`    // 代理端口
	Enabled bool      `json:"enabled"` // 是否启用
}

// GetAddress 获取代理地址
func (p *ProxySettings) GetAddress() string {
	if p == nil || !p.Enabled || p.Type == ProxyTypeNone {
		return ""
	}
	if p.Host == "" || p.Port == 0 {
		return ""
	}
	return p.Host + ":" + string(rune(p.Port+'0'))
}

// GetURL 获取代理 URL
func (p *ProxySettings) GetURL() string {
	if p == nil || !p.Enabled || p.Type == ProxyTypeNone {
		return ""
	}
	if p.Host == "" || p.Port == 0 {
		return ""
	}
	switch p.Type {
	case ProxyTypeSocks5:
		return "socks5://" + p.Host + ":" + itoa(p.Port)
	case ProxyTypeHTTP:
		return "http://" + p.Host + ":" + itoa(p.Port)
	default:
		return ""
	}
}

// itoa 简单的整数转字符串
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// AppSettings 应用全局设置
type AppSettings struct {
	Proxy ProxySettings `json:"proxy"`
}

// DefaultAppSettings 默认设置
func DefaultAppSettings() *AppSettings {
	return &AppSettings{
		Proxy: ProxySettings{
			Type:    ProxyTypeNone,
			Host:    "127.0.0.1",
			Port:    7891,
			Enabled: false,
		},
	}
}

