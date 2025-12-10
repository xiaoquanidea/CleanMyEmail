package cleaner

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	imapClient "CleanMyEmail/internal/email/imap"
	"CleanMyEmail/internal/model"
)

const (
	defaultBatchSize      = 500 // 默认每批处理的邮件数量
	defaultMaxConcurrency = 3   // 默认最大并发文件夹数（与连接池大小匹配）
)

// Cleaner 邮件清理器
type Cleaner struct {
	pool       *imapClient.ConnectionPool
	ownsPool   bool // 是否拥有连接池（需要在清理完成后关闭）
	ctx        context.Context
	cancel     context.CancelFunc
	progressCh chan *model.CleanProgress
	mu         sync.Mutex
	running    bool
}

// NewCleaner 创建清理器（使用外部连接池）
func NewCleaner(pool *imapClient.ConnectionPool) *Cleaner {
	return &Cleaner{
		pool:       pool,
		ownsPool:   false,
		progressCh: make(chan *model.CleanProgress, 100),
	}
}

// NewCleanerWithConfig 创建清理器（自动创建连接池）
func NewCleanerWithConfig(config *imapClient.ConnectConfig, concurrency int) *Cleaner {
	if concurrency <= 0 {
		concurrency = defaultMaxConcurrency
	}
	pool := imapClient.NewConnectionPool(config, &imapClient.PoolOptions{
		MaxSize:     concurrency,
		IdleTimeout: 5 * time.Minute,
	})
	return &Cleaner{
		pool:       pool,
		ownsPool:   true,
		progressCh: make(chan *model.CleanProgress, 100),
	}
}

// ProgressChan 获取进度通道
func (c *Cleaner) ProgressChan() <-chan *model.CleanProgress {
	return c.progressCh
}

// Clean 执行清理
func (c *Cleaner) Clean(req *model.CleanRequest) (*model.CleanResult, error) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil, fmt.Errorf("清理任务正在进行中")
	}
	c.running = true
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.mu.Unlock()

	startTime := time.Now()
	result := &model.CleanResult{
		AccountID:   req.AccountID,
		FolderStats: make([]model.FolderCleanStat, 0, len(req.Folders)),
		Status:      "completed",
	}

	concurrency := req.GetMaxConcurrency()

	defer func() {
		// 只有拥有连接池时才关闭
		if c.ownsPool && c.pool != nil {
			stats := c.pool.Stats()
			log.Printf("[DEBUG] 清理完成，连接池统计: 创建=%d, 复用=%d, 健康检查失败=%d",
				stats.Created, stats.Reused, stats.HealthErr)
			c.pool.Close()
			c.pool = nil
		}
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		close(c.progressCh)
	}()

	// 解析日期
	var startDate time.Time
	var err error
	if req.StartDate != "" {
		startDate, err = time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			return nil, fmt.Errorf("开始日期格式错误: %w", err)
		}
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("结束日期格式错误: %w", err)
	}
	// 结束日期加一天（包含当天）
	endDate = endDate.Add(24 * time.Hour)

	var totalDeleted int64
	var wg sync.WaitGroup
	batchSize := req.GetBatchSize()
	sem := make(chan struct{}, concurrency)
	statsCh := make(chan model.FolderCleanStat, len(req.Folders))

	for i, folder := range req.Folders {
		select {
		case <-c.ctx.Done():
			result.Status = "cancelled"
			goto done
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, folderName string, bs int) {
			defer wg.Done()
			defer func() { <-sem }()

			stat := c.cleanFolder(folderName, startDate, endDate, req, idx, len(req.Folders), bs)
			atomic.AddInt64(&totalDeleted, int64(stat.DeletedCount))
			statsCh <- stat
		}(i, folder, batchSize)
	}

	wg.Wait()

done:
	close(statsCh)
	for stat := range statsCh {
		result.FolderStats = append(result.FolderStats, stat)
	}

	result.TotalDeleted = int(totalDeleted)
	result.Duration = time.Since(startTime).Seconds()

	// 发送完成进度
	c.sendProgress(&model.CleanProgress{
		AccountID:      req.AccountID,
		Status:         result.Status,
		DeletedCount:   result.TotalDeleted,
		ElapsedSeconds: result.Duration,
		Message:        fmt.Sprintf("清理完成，共删除 %d 封邮件", result.TotalDeleted),
	})

	return result, nil
}

// Cancel 取消清理
func (c *Cleaner) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
}

// sendProgress 发送进度
func (c *Cleaner) sendProgress(progress *model.CleanProgress) {
	select {
	case c.progressCh <- progress:
	default:
		// 通道满了就丢弃
	}
}

