package proxy

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
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
	globalSettings = settings
	if settings != nil && settings.Enabled {
		log.Printf("[INFO] 代理设置已更新: %s", settings.GetURL())
	}
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

// Dial 使用全局代理设置建立 TCP 连接（用于 IMAP 等非 HTTP 协议）
// 支持 SOCKS5 和 HTTP CONNECT 代理
func Dial(network, address string, timeout time.Duration) (net.Conn, error) {
	settings := GetGlobalProxy()

	// 如果没有启用代理，直接连接
	if settings == nil || !settings.Enabled || settings.Type == model.ProxyTypeNone {
		return net.DialTimeout(network, address, timeout)
	}

	proxyAddr := fmt.Sprintf("%s:%d", settings.Host, settings.Port)

	switch settings.Type {
	case model.ProxyTypeSocks5:
		return dialSocks5(proxyAddr, network, address, timeout)

	case model.ProxyTypeHTTP:
		return dialHTTPConnect(proxyAddr, network, address, timeout)

	default:
		return net.DialTimeout(network, address, timeout)
	}
}

// dialSocks5 通过 SOCKS5 代理建立连接
func dialSocks5(proxyAddr, network, address string, timeout time.Duration) (net.Conn, error) {
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, &net.Dialer{Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("创建SOCKS5代理失败: %w", err)
	}
	return dialer.Dial(network, address)
}

// dialHTTPConnect 通过 HTTP CONNECT 代理建立隧道连接
// 用于 IMAP 等非 HTTP 协议通过 HTTP 代理
func dialHTTPConnect(proxyAddr, network, address string, timeout time.Duration) (net.Conn, error) {
	// 连接到代理服务器
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("连接HTTP代理失败: %w", err)
	}

	// 发送 CONNECT 请求
	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}
	req.Header.Set("Proxy-Connection", "Keep-Alive")

	// 设置写入超时
	conn.SetDeadline(time.Now().Add(timeout))
	defer conn.SetDeadline(time.Time{}) // 清除超时

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("发送CONNECT请求失败: %w", err)
	}

	// 读取响应
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("读取代理响应失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("HTTP代理返回错误: %s", resp.Status)
	}

	return conn, nil
}

// GetHTTPProxyURL 获取 HTTP 代理 URL（用于 http.Transport）
// 返回 nil 表示不使用代理
func GetHTTPProxyURL() *url.URL {
	settings := GetGlobalProxy()
	if settings == nil || !settings.Enabled || settings.Type == model.ProxyTypeNone {
		return nil
	}

	var scheme string
	switch settings.Type {
	case model.ProxyTypeSocks5:
		scheme = "socks5"
	case model.ProxyTypeHTTP:
		scheme = "http"
	default:
		return nil
	}

	return &url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%d", settings.Host, settings.Port),
	}
}

// GetHTTPProxyFunc 返回用于 http.Transport.Proxy 的函数
func GetHTTPProxyFunc() func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		proxyURL := GetHTTPProxyURL()
		return proxyURL, nil
	}
}

// IsEnabled 检查代理是否启用
func IsEnabled() bool {
	settings := GetGlobalProxy()
	return settings != nil && settings.Enabled && settings.Type != model.ProxyTypeNone
}

// GetProxyURL 获取代理 URL 字符串（用于日志等）
func GetProxyURL() string {
	settings := GetGlobalProxy()
	if settings == nil || !settings.Enabled {
		return ""
	}
	return settings.GetURL()
}

// basicAuth 生成 Basic 认证头（预留，当前未使用）
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

