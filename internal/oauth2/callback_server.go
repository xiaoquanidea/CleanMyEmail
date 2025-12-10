package oauth2

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// CallbackResult OAuth2回调结果
type CallbackResult struct {
	Code  string
	State string
	Error string
}

// CallbackServer 本地OAuth2回调服务器
type CallbackServer struct {
	server     *http.Server
	listener   net.Listener
	port       int
	resultChan chan CallbackResult
	mu         sync.Mutex
	running    bool
}

// NewCallbackServer 创建回调服务器
func NewCallbackServer() *CallbackServer {
	return &CallbackServer{
		resultChan: make(chan CallbackResult, 1),
	}
}

// Start 启动回调服务器
func (s *CallbackServer) Start() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return s.port, nil
	}

	// 使用 localhost（不是 127.0.0.1），Microsoft OAuth2 要求必须是 localhost
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, fmt.Errorf("无法启动回调服务器: %w", err)
	}

	s.listener = listener
	s.port = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	// 根路径处理回调（Microsoft 不带路径）
	mux.HandleFunc("/", s.handleCallback)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	s.running = true

	go func() {
		_ = s.server.Serve(listener)
	}()

	return s.port, nil
}

// Stop 停止回调服务器
func (s *CallbackServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.server.Shutdown(ctx)
	s.running = false
}

// GetRedirectURI 获取回调地址（使用 localhost，Microsoft 要求）
func (s *CallbackServer) GetRedirectURI() string {
	return fmt.Sprintf("http://localhost:%d/", s.port)
}

// WaitForCallback 等待回调结果
func (s *CallbackServer) WaitForCallback(timeout time.Duration) (*CallbackResult, error) {
	select {
	case result := <-s.resultChan:
		return &result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("等待授权超时")
	}
}

// handleCallback 处理OAuth2回调
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errMsg := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	result := CallbackResult{
		Code:  code,
		State: state,
	}

	if errMsg != "" {
		result.Error = fmt.Sprintf("%s: %s", errMsg, errDesc)
	}

	// 发送结果
	select {
	case s.resultChan <- result:
	default:
	}

	// 返回成功页面
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if result.Error != "" {
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>授权失败</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1 style="color: #e74c3c;">❌ 授权失败</h1>
<p>%s</p>
<p>请关闭此窗口并重试</p>
</body></html>`, result.Error)
	} else {
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权成功</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1 style="color: #27ae60;">✅ 授权成功</h1>
<p>您可以关闭此窗口并返回应用</p>
<script>setTimeout(function(){window.close();}, 2000);</script>
</body></html>`)
	}
}

