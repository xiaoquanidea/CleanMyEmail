package imap

import (
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	poolCleanupInterval = 10 * time.Minute // 清理间隔
	poolMaxIdleTime     = 10 * time.Minute // 池最大空闲时间
)

// PoolManager 连接池管理器，为每个账号维护一个连接池
type PoolManager struct {
	mu    sync.RWMutex
	pools map[int64]*managedPool // key: accountID

	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

// managedPool 被管理的连接池
type managedPool struct {
	pool       *ConnectionPool
	config     *ConnectConfig
	lastAccess time.Time
}

// NewPoolManager 创建连接池管理器
func NewPoolManager() *PoolManager {
	pm := &PoolManager{
		pools:       make(map[int64]*managedPool),
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}
	go pm.cleanupLoop()
	return pm
}

// GetPool 获取或创建账号的连接池
func (pm *PoolManager) GetPool(accountID int64, config *ConnectConfig, opts *PoolOptions) *ConnectionPool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	logPrefix := fmt.Sprintf("[%s@%s]", config.Username, config.Server)

	if mp, ok := pm.pools[accountID]; ok {
		mp.lastAccess = time.Now()
		// 检查配置是否变化（简单比较服务器和用户名）
		if mp.config.Server == config.Server && mp.config.Username == config.Username {
			// 更新可能变化的字段（如 AccessToken、TokenRefresher）
			mp.config.AccessToken = config.AccessToken
			mp.config.TokenRefresher = config.TokenRefresher
			mp.pool.UpdateConfig(config)
			log.Printf("[DEBUG] %s 复用连接池", logPrefix)
			return mp.pool
		}
		// 配置变化，关闭旧池
		log.Printf("[DEBUG] %s 配置变化，重建连接池", logPrefix)
		mp.pool.Close()
	}

	// 创建新池
	pool := NewConnectionPool(config, opts)
	pm.pools[accountID] = &managedPool{
		pool:       pool,
		config:     config,
		lastAccess: time.Now(),
	}
	log.Printf("[DEBUG] %s 创建新连接池", logPrefix)
	return pool
}

// ClosePool 关闭指定账号的连接池
func (pm *PoolManager) ClosePool(accountID int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if mp, ok := pm.pools[accountID]; ok {
		mp.pool.Close()
		delete(pm.pools, accountID)
		log.Printf("[DEBUG] 连接池管理器: 关闭账号 %d 的连接池", accountID)
	}
}

// Close 关闭管理器和所有连接池
func (pm *PoolManager) Close() {
	close(pm.stopCleanup)
	<-pm.cleanupDone

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for accountID, mp := range pm.pools {
		mp.pool.Close()
		log.Printf("[DEBUG] 连接池管理器: 关闭账号 %d 的连接池", accountID)
	}
	pm.pools = nil
}

// cleanupLoop 定期清理空闲的连接池
func (pm *PoolManager) cleanupLoop() {
	defer close(pm.cleanupDone)

	ticker := time.NewTicker(poolCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopCleanup:
			return
		case <-ticker.C:
			pm.cleanupIdlePools()
		}
	}
}

// cleanupIdlePools 清理空闲的连接池
func (pm *PoolManager) cleanupIdlePools() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for accountID, mp := range pm.pools {
		if now.Sub(mp.lastAccess) > poolMaxIdleTime {
			// 检查池中是否有使用中的连接
			stats := mp.pool.Stats()
			if stats.InUse == 0 {
				mp.pool.Close()
				delete(pm.pools, accountID)
				log.Printf("[DEBUG] 连接池管理器: 清理空闲连接池 (账号 %d)", accountID)
			}
		}
	}
}

// Stats 获取所有连接池的统计信息
func (pm *PoolManager) Stats() map[int64]PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[int64]PoolStats)
	for accountID, mp := range pm.pools {
		result[accountID] = mp.pool.Stats()
	}
	return result
}

