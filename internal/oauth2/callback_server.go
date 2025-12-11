package oauth2

import (
	"context"
	"fmt"
	"log"
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

// CallbackServer 本地OAuth2回调服务器（支持多个并发OAuth2会话）
type CallbackServer struct {
	server   *http.Server
	listener net.Listener
	port     int
	mu       sync.Mutex
	running  bool
	// 使用 state 作为 key 存储每个会话的结果通道
	sessions map[string]chan CallbackResult
}

// NewCallbackServer 创建回调服务器
func NewCallbackServer() *CallbackServer {
	return &CallbackServer{
		sessions: make(map[string]chan CallbackResult),
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

	log.Printf("[INFO] OAuth2 回调服务器已启动，端口: %d", s.port)
	return s.port, nil
}

// RegisterSession 注册一个新的 OAuth2 会话
func (s *CallbackServer) RegisterSession(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[state] = make(chan CallbackResult, 1)
	log.Printf("[DEBUG] 注册 OAuth2 会话: %s", state)
}

// UnregisterSession 注销一个 OAuth2 会话
func (s *CallbackServer) UnregisterSession(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.sessions[state]; ok {
		close(ch)
		delete(s.sessions, state)
		log.Printf("[DEBUG] 注销 OAuth2 会话: %s", state)
	}
}

// Stop 停止回调服务器（仅当没有活跃会话时）
func (s *CallbackServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	// 如果还有活跃会话，不停止服务器
	if len(s.sessions) > 0 {
		log.Printf("[DEBUG] 还有 %d 个活跃 OAuth2 会话，保持服务器运行", len(s.sessions))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.server.Shutdown(ctx)
	s.running = false
	log.Printf("[INFO] OAuth2 回调服务器已停止")
}

// ForceStop 强制停止回调服务器（清理所有会话）
func (s *CallbackServer) ForceStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 关闭所有会话通道
	for state, ch := range s.sessions {
		close(ch)
		delete(s.sessions, state)
	}

	if !s.running {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.server.Shutdown(ctx)
	s.running = false
	log.Printf("[INFO] OAuth2 回调服务器已强制停止")
}

// GetRedirectURI 获取回调地址（使用 localhost，Microsoft 要求）
func (s *CallbackServer) GetRedirectURI() string {
	return fmt.Sprintf("http://localhost:%d/", s.port)
}

// WaitForCallback 等待指定 state 的回调结果
func (s *CallbackServer) WaitForCallback(state string, timeout time.Duration) (*CallbackResult, error) {
	s.mu.Lock()
	ch, ok := s.sessions[state]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("未找到对应的 OAuth2 会话")
	}

	select {
	case result, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("OAuth2 会话已取消")
		}
		return &result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("等待授权超时")
	}
}

// handleCallback 处理OAuth2回调
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	// 忽略 favicon.ico 等非回调请求
	if r.URL.Path == "/favicon.ico" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errMsg := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	// 如果没有 code 和 state，也没有 error，说明不是有效的 OAuth2 回调
	if code == "" && state == "" && errMsg == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("[DEBUG] 收到 OAuth2 回调请求, URL: %s", r.URL.String())
	log.Printf("[DEBUG] 回调参数 - state: %s, hasCode: %v, error: %s", state, code != "", errMsg)

	result := CallbackResult{
		Code:  code,
		State: state,
	}

	if errMsg != "" {
		result.Error = fmt.Sprintf("%s: %s", errMsg, errDesc)
		log.Printf("[WARN] OAuth2 授权错误: %s", result.Error)
	}

	// 根据 state 找到对应的会话通道
	s.mu.Lock()
	ch, ok := s.sessions[state]
	log.Printf("[DEBUG] 查找会话 - state: %s, 找到: %v, 当前会话数: %d", state, ok, len(s.sessions))
	s.mu.Unlock()

	if ok {
		// 发送结果到对应的会话
		select {
		case ch <- result:
			log.Printf("[INFO] 已发送回调结果到会话: %s, code长度: %d", state, len(code))
		default:
			log.Printf("[ERROR] 会话通道已满或已关闭: %s", state)
		}
	} else {
		log.Printf("[ERROR] 未找到匹配的 OAuth2 会话, state: %s", state)
	}

	// 返回成功页面
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if result.Error != "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>授权失败</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1 style="color: #e74c3c;">❌ 授权失败</h1>
<p>%s</p>
<p>请关闭此窗口并重试</p>
</body></html>`, result.Error)
	} else if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1 style="color: #e74c3c;">❌ 授权失败</h1>
<p>未找到对应的授权会话，请重新发起授权</p>
</body></html>`)
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权成功</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1 style="color: #27ae60;">✅ 授权成功</h1>
<p>您可以关闭此窗口并返回应用</p>
<script>setTimeout(function(){window.close();}, 2000);</script>
</body></html>`)
	}
}

