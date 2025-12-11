package imap

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"

	"CleanMyEmail/internal/model"
	"CleanMyEmail/internal/proxy"
)

// ConnectConfig IMAP连接配置
type ConnectConfig struct {
	Server      string
	Username    string
	Password    string
	AuthType    model.EmailAuthType
	AccessToken string
	// TokenRefresher 用于在 token 过期时刷新，返回新的 access token
	// 如果为 nil，则不支持自动刷新
	TokenRefresher func() (string, error)
}

// Connect 连接到IMAP服务器（带重试）
func Connect(cfg *ConnectConfig) (*imapclient.Client, error) {
	var lastErr error
	maxRetries := 3
	logPrefix := fmt.Sprintf("[%s@%s]", cfg.Username, cfg.Server)

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			// 重试前等待
			waitTime := time.Duration(i) * 2 * time.Second
			log.Printf("[DEBUG] %s 等待 %v 后重试...", logPrefix, waitTime)
			time.Sleep(waitTime)
		}

		client, err := connectOnce(cfg, logPrefix)
		if err == nil {
			return client, nil
		}
		lastErr = err
		log.Printf("[DEBUG] %s 连接尝试 %d/%d 失败: %v", logPrefix, i+1, maxRetries, err)
	}

	return nil, lastErr
}

// connectOnce 单次连接尝试
func connectOnce(cfg *ConnectConfig, logPrefix string) (*imapclient.Client, error) {
	host, port := parseServer(cfg.Server)

	// 创建TLS配置
	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	// 连接服务器
	address := fmt.Sprintf("%s:%s", host, port)

	// 检查代理设置
	if proxy.IsEnabled() {
		log.Printf("[DEBUG] %s 连接 (通过代理 %s)", logPrefix, proxy.GetProxyURL())
	} else {
		log.Printf("[DEBUG] %s 连接 (直连)", logPrefix)
	}

	// 使用全局代理设置建立 TCP 连接
	tcpConn, err := proxy.Dial("tcp", address, 30*time.Second)
	if err != nil {
		log.Printf("[DEBUG] %s TCP连接失败: %v", logPrefix, err)
		return nil, fmt.Errorf("TCP连接失败: %w", err)
	}

	// TLS 握手
	conn := tls.Client(tcpConn, tlsConfig)
	if err := conn.Handshake(); err != nil {
		log.Printf("[DEBUG] %s TLS握手失败: %v", logPrefix, err)
		tcpConn.Close()
		return nil, fmt.Errorf("TLS握手失败: %w", err)
	}

	// 创建IMAP客户端
	client := imapclient.New(conn, nil)

	// 等待服务器的 greeting 响应
	if err := client.WaitGreeting(); err != nil {
		client.Close()
		return nil, fmt.Errorf("等待服务器响应失败: %w", err)
	}

	// 认证
	if err := authenticate(client, cfg, logPrefix); err != nil {
		log.Printf("[DEBUG] %s 认证失败: %v", logPrefix, err)
		client.Close()
		return nil, err
	}
	log.Printf("[DEBUG] %s 认证成功", logPrefix)

	return client, nil
}

// authenticate 认证
func authenticate(client *imapclient.Client, cfg *ConnectConfig, logPrefix string) error {
	if cfg.AuthType.IsOAuth2() {
		return authenticateOAuth2(client, cfg.Username, cfg.AccessToken, logPrefix)
	}
	return authenticatePassword(client, cfg.Username, cfg.Password, logPrefix)
}

// authenticatePassword 密码认证
func authenticatePassword(client *imapclient.Client, username, password, logPrefix string) error {
	loginCmd := client.Login(username, password)

	// 使用 goroutine + channel 实现超时
	type loginResult struct {
		err error
	}
	resultCh := make(chan loginResult, 1)
	go func() {
		err := loginCmd.Wait()
		resultCh <- loginResult{err: err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			return fmt.Errorf("登录失败: %w", result.err)
		}
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("登录超时（30秒）")
	}
}

// authenticateOAuth2 OAuth2认证
func authenticateOAuth2(client *imapclient.Client, username, accessToken, logPrefix string) error {
	saslClient := NewXOAuth2Client(username, accessToken)

	// 使用 goroutine + channel 实现超时
	type authResult struct {
		err error
	}
	resultCh := make(chan authResult, 1)
	go func() {
		err := client.Authenticate(saslClient)
		resultCh <- authResult{err: err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			return fmt.Errorf("OAuth2认证失败: %w", result.err)
		}
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("OAuth2认证超时（30秒）")
	}
}

// parseServer 解析服务器地址
func parseServer(server string) (host, port string) {
	if strings.Contains(server, ":") {
		parts := strings.SplitN(server, ":", 2)
		return parts[0], parts[1]
	}
	return server, "993"
}

// XOAuth2Client XOAUTH2 SASL客户端
type XOAuth2Client struct {
	username    string
	accessToken string
}

// NewXOAuth2Client 创建XOAUTH2客户端
func NewXOAuth2Client(username, accessToken string) sasl.Client {
	return &XOAuth2Client{
		username:    username,
		accessToken: accessToken,
	}
}

func (c *XOAuth2Client) Start() (mech string, ir []byte, err error) {
	// XOAUTH2格式: user=<username>\x01auth=Bearer <token>\x01\x01
	ir = []byte(fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.username, c.accessToken))
	return "XOAUTH2", ir, nil
}

func (c *XOAuth2Client) Next(challenge []byte) (response []byte, err error) {
	// XOAUTH2不需要额外的挑战响应
	return nil, nil
}

// TestConnection 测试连接
func TestConnection(cfg *ConnectConfig) error {
	client, err := Connect(cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	return nil
}

// SupportsListStatus 检查服务器是否支持 LIST-STATUS 扩展
func SupportsListStatus(client *imapclient.Client) bool {
	return client.Caps().Has(imap.CapListStatus)
}

// ListMailboxes 列出所有邮箱文件夹
func ListMailboxes(client *imapclient.Client) ([]*model.MailFolder, error) {
	startTime := time.Now()
	log.Printf("[DEBUG] 开始获取文件夹列表...")

	// 检查服务器是否支持 LIST-STATUS 扩展
	caps := client.Caps()
	supportsListStatus := caps.Has(imap.CapListStatus)
	log.Printf("[DEBUG] 服务器支持 LIST-STATUS: %v", supportsListStatus)

	// 构建 LIST 命令选项
	var listOpts *imap.ListOptions
	if supportsListStatus {
		// 如果支持 LIST-STATUS，请求邮件数量
		listOpts = &imap.ListOptions{
			ReturnStatus: &imap.StatusOptions{
				NumMessages: true,
				NumUnseen:   true,
			},
		}
		log.Printf("[DEBUG] 发送 LIST-STATUS 命令...")
	} else {
		log.Printf("[DEBUG] 发送 LIST 命令...")
	}
	listCmd := client.List("", "*", listOpts)

	var folders []*model.MailFolder
	log.Printf("[DEBUG] 开始读取文件夹...")
	folderCount := 0
	for {
		mbox := listCmd.Next()
		if mbox == nil {
			break
		}

		folderCount++
		// 减少日志输出，只在每10个文件夹时输出一次
		if folderCount <= 5 || folderCount%10 == 0 {
			log.Printf("[DEBUG] 发现文件夹 #%d: %s", folderCount, mbox.Mailbox)
		}

		folder := &model.MailFolder{
			Name:         mbox.Mailbox,
			FullPath:     mbox.Mailbox,
			Delimiter:    string(mbox.Delim),
			Attributes:   make([]string, 0, len(mbox.Attrs)),
			IsSelectable: true,
		}

		for _, attr := range mbox.Attrs {
			folder.Attributes = append(folder.Attributes, string(attr))
			if attr == imap.MailboxAttrNoSelect {
				folder.IsSelectable = false
			}
		}

		if mbox.Status != nil {
			if mbox.Status.NumMessages != nil {
				folder.MessageCount = *mbox.Status.NumMessages
			}
			if mbox.Status.NumUnseen != nil {
				folder.UnseenCount = *mbox.Status.NumUnseen
			}
		}

		// 调试：记录 LIST-STATUS 返回的数量
		if supportsListStatus && folderCount <= 10 {
			hasStatus := mbox.Status != nil
			log.Printf("[DEBUG] 文件夹 %s: hasStatus=%v, messageCount=%d", mbox.Mailbox, hasStatus, folder.MessageCount)
		}

		folders = append(folders, folder)
	}

	elapsed := time.Since(startTime)
	log.Printf("[DEBUG] 文件夹读取完成，共 %d 个，耗时 %.2fs", len(folders), elapsed.Seconds())

	if err := listCmd.Close(); err != nil {
		log.Printf("[DEBUG] LIST 命令关闭失败: %v", err)
		return nil, fmt.Errorf("获取文件夹列表失败: %w", err)
	}

	log.Printf("[DEBUG] 文件夹列表获取成功，总耗时 %.2fs", time.Since(startTime).Seconds())
	return folders, nil
}

// FolderStatusUpdate 文件夹状态更新
type FolderStatusUpdate struct {
	FolderPath   string `json:"folderPath"`
	MessageCount uint32 `json:"messageCount"`
	UnseenCount  uint32 `json:"unseenCount"`
}

// FetchFolderStatus 异步获取文件夹邮件数量，通过回调返回
// 策略：先用 STATUS 命令（快速），如果返回 0 则用 EXAMINE 回退（更可靠）
func FetchFolderStatus(client *imapclient.Client, folders []*model.MailFolder, onUpdate func(FolderStatusUpdate)) {
	log.Printf("[DEBUG] 开始异步获取 %d 个文件夹的邮件数量...", len(folders))
	successCount := 0
	fallbackCount := 0
	for i, folder := range folders {
		if !folder.IsSelectable {
			continue
		}

		var messageCount uint32 = 0

		// 先尝试 STATUS 命令（快速）
		statusCmd := client.Status(folder.FullPath, &imap.StatusOptions{
			NumMessages: true,
		})
		statusData, err := statusCmd.Wait()
		if err == nil && statusData.NumMessages != nil {
			messageCount = *statusData.NumMessages
		}

		// 如果 STATUS 返回 0，使用 EXAMINE 回退
		if messageCount == 0 {
			selectCmd := client.Select(folder.FullPath, &imap.SelectOptions{
				ReadOnly: true,
			})
			selectData, err := selectCmd.Wait()
			if err == nil {
				messageCount = selectData.NumMessages
				if messageCount > 0 {
					fallbackCount++
				}
			}
		}

		update := FolderStatusUpdate{
			FolderPath:   folder.FullPath,
			MessageCount: messageCount,
		}
		onUpdate(update)
		successCount++

		// 每10个文件夹输出一次进度
		if (i+1)%10 == 0 || i == len(folders)-1 {
			log.Printf("[DEBUG] 文件夹状态获取进度: %d/%d", i+1, len(folders))
		}
	}
	log.Printf("[DEBUG] 文件夹状态获取完成，成功 %d 个，EXAMINE回退 %d 个", successCount, fallbackCount)
}
