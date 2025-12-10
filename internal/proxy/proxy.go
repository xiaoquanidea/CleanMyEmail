package proxy

import (
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"CleanMyEmail/internal/model"
)

var (
	globalSettings *model.ProxySettings
	mu             sync.RWMutex
)

// SetGlobalProxy 设置全局代理
func SetGlobalProxy(settings *model.ProxySettings) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("[DEBUG] SetGlobalProxy: 设置代理 %+v\n", settings)
	globalSettings = settings
}

// GetGlobalProxy 获取全局代理设置
func GetGlobalProxy() *model.ProxySettings {
	mu.RLock()
	defer mu.RUnlock()
	if globalSettings == nil {
		return &model.ProxySettings{
			Type:    model.ProxyTypeNone,
			Enabled: false,
		}
	}
	return globalSettings
}

// Dial 使用全局代理设置建立 TCP 连接
func Dial(network, address string, timeout time.Duration) (net.Conn, error) {
	settings := GetGlobalProxy()

	fmt.Printf("[DEBUG] Dial: settings=%+v\n", settings)

	// 如果没有启用代理，直接连接
	if settings == nil || !settings.Enabled || settings.Type == model.ProxyTypeNone {
		fmt.Printf("[DEBUG] Dial: 使用直连\n")
		return net.DialTimeout(network, address, timeout)
	}

	proxyAddr := fmt.Sprintf("%s:%d", settings.Host, settings.Port)
	fmt.Printf("[DEBUG] Dial: 使用代理 %s\n", proxyAddr)

	switch settings.Type {
	case model.ProxyTypeSocks5:
		dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, &net.Dialer{Timeout: timeout})
		if err != nil {
			return nil, fmt.Errorf("创建SOCKS5代理失败: %w", err)
		}
		return dialer.Dial(network, address)

	case model.ProxyTypeHTTP:
		// HTTP 代理需要使用 CONNECT 方法，这里简化处理，建议使用 SOCKS5
		// 对于 IMAP 等非 HTTP 协议，SOCKS5 更合适
		return nil, fmt.Errorf("HTTP代理暂不支持IMAP连接，请使用SOCKS5代理")

	default:
		return net.DialTimeout(network, address, timeout)
	}
}

// IsEnabled 检查代理是否启用
func IsEnabled() bool {
	settings := GetGlobalProxy()
	return settings != nil && settings.Enabled && settings.Type != model.ProxyTypeNone
}

// GetProxyURL 获取代理 URL（用于日志等）
func GetProxyURL() string {
	settings := GetGlobalProxy()
	if settings == nil || !settings.Enabled {
		return ""
	}
	return settings.GetURL()
}

