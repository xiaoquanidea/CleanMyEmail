package imap

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"
)

const (
	defaultMaxSize     = 3               // 默认最大连接数
	defaultIdleTimeout = 5 * time.Minute // 默认空闲超时
	healthCheckTimeout = 5 * time.Second // 健康检查超时
	waitTimeout        = 30 * time.Second // 等待连接超时
)

// PoolOptions 连接池配置选项
type PoolOptions struct {
	MaxSize     int           // 最大连接数
	IdleTimeout time.Duration // 空闲超时时间
}

// PoolStats 连接池统计信息
type PoolStats struct {
	Total     int // 总连接数
	InUse     int // 使用中的连接数
	Idle      int // 空闲连接数
	Created   int // 累计创建的连接数
	Reused    int // 累计复用次数
	HealthErr int // 健康检查失败次数
}

// ConnectionPool IMAP 连接池
type ConnectionPool struct {
	config      *ConnectConfig
	maxSize     int
	idleTimeout time.Duration
	logPrefix   string        // 日志前缀，包含账号信息

	mu          sync.Mutex
	cond        *sync.Cond    // 条件变量，用于等待连接释放
	connections []*PooledConn
	creating    int           // 正在创建中的连接数
	closed      bool

	// 统计信息
	stats struct {
		created   int
		reused    int
		healthErr int
	}
}

// PooledConn 池化的连接
type PooledConn struct {
	client    *imapclient.Client
	pool      *ConnectionPool
	inUse     bool
	lastUsed  time.Time
	createdAt time.Time
}

// NewConnectionPool 创建连接池
func NewConnectionPool(config *ConnectConfig, opts *PoolOptions) *ConnectionPool {
	maxSize := defaultMaxSize
	idleTimeout := defaultIdleTimeout

	if opts != nil {
		if opts.MaxSize > 0 {
			maxSize = opts.MaxSize
		}
		if opts.IdleTimeout > 0 {
			idleTimeout = opts.IdleTimeout
		}
	}

	// 生成日志前缀
	logPrefix := fmt.Sprintf("[%s@%s]", config.Username, config.Server)

	p := &ConnectionPool{
		config:      config,
		maxSize:     maxSize,
		idleTimeout: idleTimeout,
		logPrefix:   logPrefix,
		connections: make([]*PooledConn, 0, maxSize),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// Get 获取一个连接（带等待和超时）
func (p *ConnectionPool) Get(ctx context.Context) (*PooledConn, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("连接池已关闭")
	}

	deadline := time.Now().Add(waitTimeout)

	for {
		// 检查 context 是否已取消
		select {
		case <-ctx.Done():
			p.mu.Unlock()
			return nil, ctx.Err()
		default:
		}

		// 1. 尝试获取空闲连接
		now := time.Now()
		for i := 0; i < len(p.connections); i++ {
			conn := p.connections[i]
			if conn.inUse {
				continue
			}

			// 检查是否超过空闲超时
			if now.Sub(conn.lastUsed) > p.idleTimeout {
				log.Printf("[DEBUG] %s 连接 #%d 空闲超时，关闭", p.logPrefix, i)
				conn.client.Close()
				p.removeConnLocked(i)
				i--
				continue
			}

			// 先标记为使用中，防止其他 goroutine 获取同一个连接
			conn.inUse = true
			connIndex := i

			// 健康检查（临时释放锁）
			p.mu.Unlock()
			healthy := p.isHealthy(conn)
			p.mu.Lock()

			if p.closed {
				conn.client.Close()
				p.mu.Unlock()
				return nil, fmt.Errorf("连接池已关闭")
			}

			if !healthy {
				log.Printf("[DEBUG] %s 连接 #%d 健康检查失败，关闭", p.logPrefix, connIndex)
				p.stats.healthErr++
				conn.client.Close()
				// 从池中移除（需要重新查找索引，因为可能已变化）
				for j, c := range p.connections {
					if c == conn {
						p.removeConnLocked(j)
						break
					}
				}
				p.cond.Signal() // 通知其他等待者
				// 继续尝试下一个连接，需要重新开始循环
				i = -1
				continue
			}

			// 找到可用连接
			conn.lastUsed = now
			p.stats.reused++
			log.Printf("[DEBUG] %s 复用连接 #%d (总复用 %d 次)", p.logPrefix, connIndex, p.stats.reused)
			p.mu.Unlock()
			return conn, nil
		}

		// 2. 检查是否可以创建新连接（包括正在创建中的）
		totalPending := len(p.connections) + p.creating
		if totalPending < p.maxSize {
			p.creating++
			p.mu.Unlock()

			// 如果有 token 刷新器，先尝试刷新 token
			if p.config.AuthType.IsOAuth2() && p.config.TokenRefresher != nil {
				if newToken, err := p.config.TokenRefresher(); err == nil && newToken != "" {
					p.config.AccessToken = newToken
					log.Printf("[DEBUG] %s Token 已刷新", p.logPrefix)
				}
			}

			// 创建新连接（不持有锁）
			client, err := Connect(p.config)

			p.mu.Lock()
			p.creating--

			if err != nil {
				p.cond.Signal() // 通知其他等待者
				p.mu.Unlock()
				return nil, fmt.Errorf("创建连接失败: %w", err)
			}

			if p.closed {
				client.Close()
				p.cond.Signal()
				p.mu.Unlock()
				return nil, fmt.Errorf("连接池已关闭")
			}

			conn := &PooledConn{
				client:    client,
				pool:      p,
				inUse:     true,
				lastUsed:  time.Now(),
				createdAt: time.Now(),
			}
			p.connections = append(p.connections, conn)
			p.stats.created++
			log.Printf("[DEBUG] %s 创建新连接 #%d (总创建 %d 个)", p.logPrefix, len(p.connections)-1, p.stats.created)
			p.mu.Unlock()
			return conn, nil
		}

		// 3. 池已满，等待连接释放
		if time.Now().After(deadline) {
			p.mu.Unlock()
			return nil, fmt.Errorf("等待连接超时 (%v)", waitTimeout)
		}

		// 使用条件变量等待，设置超时
		go func() {
			time.Sleep(100 * time.Millisecond)
			p.cond.Signal()
		}()
		p.cond.Wait()

		if p.closed {
			p.mu.Unlock()
			return nil, fmt.Errorf("连接池已关闭")
		}
	}
}

// Put 归还连接到池
func (p *ConnectionPool) Put(conn *PooledConn) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		conn.client.Close()
		return
	}

	// 检查连接是否还在池中（可能已被 MarkBad 移除）
	found := false
	for _, c := range p.connections {
		if c == conn {
			found = true
			break
		}
	}
	if !found {
		// 连接已被移除，不需要归还
		return
	}

	// 标记为空闲
	conn.inUse = false
	conn.lastUsed = time.Now()

	// 通知等待的 goroutine
	p.cond.Signal()
}

// Close 关闭连接池
func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	for _, conn := range p.connections {
		conn.client.Close()
	}
	p.connections = nil

	// 唤醒所有等待的 goroutine
	p.cond.Broadcast()

	log.Printf("[DEBUG] %s 连接池已关闭，共创建 %d 个连接，复用 %d 次", p.logPrefix, p.stats.created, p.stats.reused)
}

// Stats 获取统计信息
func (p *ConnectionPool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := PoolStats{
		Total:     len(p.connections),
		Created:   p.stats.created,
		Reused:    p.stats.reused,
		HealthErr: p.stats.healthErr,
	}

	for _, conn := range p.connections {
		if conn.inUse {
			stats.InUse++
		} else {
			stats.Idle++
		}
	}

	return stats
}

// removeConnLocked 移除连接（需要持有锁）
func (p *ConnectionPool) removeConnLocked(index int) {
	if index < 0 || index >= len(p.connections) {
		return
	}
	p.connections = append(p.connections[:index], p.connections[index+1:]...)
}

// isHealthy 检查连接健康状态（不持有锁）
func (p *ConnectionPool) isHealthy(conn *PooledConn) bool {
	// 使用 channel 实现超时
	done := make(chan error, 1)
	go func() {
		err := conn.client.Noop().Wait()
		done <- err
	}()

	select {
	case err := <-done:
		return err == nil
	case <-time.After(healthCheckTimeout):
		return false
	}
}

// MarkBad 标记连接为不可用并关闭
func (p *ConnectionPool) MarkBad(conn *PooledConn) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 找到并移除连接
	for i, c := range p.connections {
		if c == conn {
			c.client.Close()
			p.removeConnLocked(i)
			log.Printf("[DEBUG] %s 标记连接 #%d 为不可用并移除", p.logPrefix, i)
			// 通知等待的 goroutine 可以创建新连接了
			p.cond.Signal()
			return
		}
	}
}

// --- PooledConn 方法 ---

// Client 获取底层 IMAP 客户端
func (c *PooledConn) Client() *imapclient.Client {
	return c.client
}

// Release 归还连接到池
func (c *PooledConn) Release() {
	if c.pool != nil {
		c.pool.Put(c)
	}
}

// MarkBad 标记连接为不可用
func (c *PooledConn) MarkBad() {
	if c.pool != nil {
		c.pool.MarkBad(c)
	}
}

