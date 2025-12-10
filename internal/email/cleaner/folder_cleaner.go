package cleaner

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	imapClient "CleanMyEmail/internal/email/imap"
	"CleanMyEmail/internal/model"
)

const (
	maxRetries    = 3               // 最大重试次数
	retryInterval = 2 * time.Second // 重试间隔
)

// retryResult 重试操作的结果
type retryResult struct {
	conn   *imapClient.PooledConn
	client *imapclient.Client
}

// retryWithReconnect 带重连的重试操作
// operation: 执行的操作，返回 error
// onReconnect: 重连后需要执行的操作（如重新选择文件夹）
func (c *Cleaner) retryWithReconnect(
	conn *imapClient.PooledConn,
	folderName string,
	operation func(client *imapclient.Client) error,
) (*retryResult, error) {
	client := conn.Client()
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		select {
		case <-c.ctx.Done():
			return nil, fmt.Errorf("操作已取消")
		default:
		}

		if err := operation(client); err == nil {
			return &retryResult{conn: conn, client: client}, nil
		} else {
			lastErr = err
		}

		if retry < maxRetries-1 {
			log.Printf("[DEBUG] 操作失败，%v 后重试 (%d/%d): %v", retryInterval, retry+1, maxRetries, lastErr)
			time.Sleep(retryInterval)

			// 标记当前连接为不可用，获取新连接
			conn.MarkBad()
			var err error
			conn, err = c.getConnection()
			if err != nil {
				return nil, fmt.Errorf("重新获取连接失败: %w", err)
			}
			client = conn.Client()

			// 重新选择文件夹
			if folderName != "" {
				selectCmd := client.Select(folderName, nil)
				if _, err := selectCmd.Wait(); err != nil {
					return nil, fmt.Errorf("重新选择文件夹失败: %w", err)
				}
			}
		}
	}

	return nil, fmt.Errorf("操作失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// getConnection 从连接池获取连接（带重试）
func (c *Cleaner) getConnection() (*imapClient.PooledConn, error) {
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		select {
		case <-c.ctx.Done():
			return nil, fmt.Errorf("操作已取消")
		default:
		}

		conn, err := c.pool.Get(c.ctx)
		if err == nil {
			return conn, nil
		}

		lastErr = err
		if retry < maxRetries-1 {
			log.Printf("[DEBUG] 获取连接失败，%v 后重试 (%d/%d): %v", retryInterval, retry+1, maxRetries, err)
			time.Sleep(retryInterval)
		}
	}
	return nil, fmt.Errorf("获取连接失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// parseSize 解析大小筛选条件，返回字节数和比较符号
func parseSize(sizeFilter string) (int64, string) {
	if sizeFilter == "" {
		return 0, ""
	}
	var op string
	var sizeStr string
	if strings.HasPrefix(sizeFilter, ">") {
		op = ">"
		sizeStr = sizeFilter[1:]
	} else if strings.HasPrefix(sizeFilter, "<") {
		op = "<"
		sizeStr = sizeFilter[1:]
	} else {
		return 0, ""
	}

	sizeStr = strings.ToUpper(sizeStr)
	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		sizeStr = sizeStr[:len(sizeStr)-1]
	} else if strings.HasSuffix(sizeStr, "K") {
		multiplier = 1024
		sizeStr = sizeStr[:len(sizeStr)-1]
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, ""
	}
	return size * multiplier, op
}

// buildOrChain 构建发件人 OR 条件链
// IMAP OR 语法: OR <criteria1> <criteria2>
// 多个发件人需要嵌套: OR (FROM a) (OR (FROM b) (FROM c))
// 例如 4 个发件人: OR (FROM a) (OR (FROM b) (OR (FROM c) (FROM d)))
func buildOrChain(senders []string) [][2]imap.SearchCriteria {
	if len(senders) < 2 {
		return nil
	}

	// 从后往前构建嵌套的 OR 链
	// 最后两个发件人组成最内层的 OR
	n := len(senders)
	innerCriteria := imap.SearchCriteria{
		Header: []imap.SearchCriteriaHeaderField{{Key: "From", Value: senders[n-1]}},
	}

	// 从倒数第二个开始，逐层包装
	for i := n - 2; i >= 0; i-- {
		currentCriteria := imap.SearchCriteria{
			Header: []imap.SearchCriteriaHeaderField{{Key: "From", Value: senders[i]}},
		}
		// 创建 OR(current, inner)
		innerCriteria = imap.SearchCriteria{
			Or: [][2]imap.SearchCriteria{{currentCriteria, innerCriteria}},
		}
	}

	// 返回最外层的 Or 字段
	return innerCriteria.Or
}

// cleanFolder 清理单个文件夹
func (c *Cleaner) cleanFolder(folderName string, startDate, endDate time.Time, req *model.CleanRequest, folderIdx, totalFolders, batchSize int) model.FolderCleanStat {
	stat := model.FolderCleanStat{
		Folder: folderName,
		Status: "completed",
	}

	// 从连接池获取连接
	conn, err := c.getConnection()
	if err != nil {
		stat.Status = "failed"
		stat.Error = fmt.Sprintf("获取连接失败: %v", err)
		return stat
	}
	// 使用完毕后归还连接（不是关闭）
	defer conn.Release()

	client := conn.Client()

	// 选择文件夹
	selectCmd := client.Select(folderName, nil)
	mbox, err := selectCmd.Wait()
	if err != nil {
		stat.Status = "failed"
		stat.Error = fmt.Sprintf("选择文件夹失败: %v", err)
		return stat
	}

	if mbox.NumMessages == 0 {
		c.sendProgress(&model.CleanProgress{
			CurrentFolder: folderName,
			FolderIndex:   folderIdx + 1,
			TotalFolders:  totalFolders,
			Status:        "running",
			Message:       fmt.Sprintf("文件夹 %s 为空", folderName),
		})
		return stat
	}

	// 构建基础搜索条件
	buildBaseCriteria := func() *imap.SearchCriteria {
		criteria := &imap.SearchCriteria{
			SentBefore: endDate,
		}
		if !startDate.IsZero() {
			criteria.SentSince = startDate
		}
		// 主题筛选
		if req.FilterSubject != "" {
			criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
				Key:   "Subject",
				Value: req.FilterSubject,
			})
		}
		// 大小筛选
		if req.FilterSize != "" {
			size, op := parseSize(req.FilterSize)
			if size > 0 {
				if op == ">" {
					criteria.Larger = size
				} else if op == "<" {
					criteria.Smaller = size
				}
			}
		}
		// 已读/未读筛选
		if req.FilterRead == "seen" {
			criteria.Flag = append(criteria.Flag, imap.FlagSeen)
		} else if req.FilterRead == "unseen" {
			criteria.NotFlag = append(criteria.NotFlag, imap.FlagSeen)
		}
		return criteria
	}

	// 解析发件人列表
	senders := []string{}
	if req.FilterSender != "" {
		for _, sender := range strings.Split(req.FilterSender, ",") {
			sender = strings.TrimSpace(sender)
			if sender != "" {
				senders = append(senders, sender)
			}
		}
	}

	// 构建完整的搜索条件（包含发件人 OR 逻辑）
	buildFullCriteria := func() *imap.SearchCriteria {
		criteria := buildBaseCriteria()

		if len(senders) == 1 {
			// 单个发件人，直接添加到 Header
			criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
				Key:   "From",
				Value: senders[0],
			})
		} else if len(senders) > 1 {
			// 多个发件人，使用 IMAP 原生 OR 条件
			// Or 是 [][2]SearchCriteria，需要构建 OR 链
			// 例如: (OR (FROM a) (OR (FROM b) (FROM c)))
			orCriteria := buildOrChain(senders)
			criteria.Or = orCriteria
		}

		return criteria
	}

	// 搜索符合条件的邮件（带重试）
	var uids []imap.UID
	result, err := c.retryWithReconnect(conn, folderName, func(cli *imapclient.Client) error {
		criteria := buildFullCriteria()
		searchCmd := cli.UIDSearch(criteria, nil)
		searchData, searchErr := searchCmd.Wait()
		if searchErr != nil {
			return searchErr
		}
		uids = searchData.AllUIDs()
		return nil
	})
	if err != nil {
		stat.Status = "failed"
		stat.Error = fmt.Sprintf("搜索邮件失败: %v", err)
		return stat
	}
	conn = result.conn
	client = result.client

	stat.MatchedCount = len(uids)

	if len(uids) == 0 {
		c.sendProgress(&model.CleanProgress{
			CurrentFolder: folderName,
			FolderIndex:   folderIdx + 1,
			TotalFolders:  totalFolders,
			MatchedCount:  0,
			Status:        "running",
			Message:       fmt.Sprintf("文件夹 %s 没有符合条件的邮件", folderName),
		})
		return stat
	}

	if req.PreviewOnly {
		c.sendProgress(&model.CleanProgress{
			CurrentFolder: folderName,
			FolderIndex:   folderIdx + 1,
			TotalFolders:  totalFolders,
			MatchedCount:  stat.MatchedCount,
			Status:        "running",
			Message:       fmt.Sprintf("预览: 文件夹 %s 有 %d 封邮件符合条件", folderName, stat.MatchedCount),
		})
		return stat
	}

	// 分批删除
	totalBatches := (len(uids) + batchSize - 1) / batchSize
	for batch := 0; batch < totalBatches; batch++ {
		select {
		case <-c.ctx.Done():
			stat.Status = "cancelled"
			return stat
		default:
		}

		start := batch * batchSize
		end := start + batchSize
		if end > len(uids) {
			end = len(uids)
		}

		batchUIDs := uids[start:end]

		// 删除批次（带重试）
		var deleted int
		result, err := c.retryWithReconnect(conn, folderName, func(cli *imapclient.Client) error {
			var deleteErr error
			deleted, deleteErr = c.deleteBatch(cli, batchUIDs)
			return deleteErr
		})
		if err != nil {
			stat.Status = "failed"
			stat.Error = fmt.Sprintf("删除失败: %v", err)
			return stat
		}
		conn = result.conn
		client = result.client

		stat.DeletedCount += deleted

		c.sendProgress(&model.CleanProgress{
			CurrentFolder: folderName,
			FolderIndex:   folderIdx + 1,
			TotalFolders:  totalFolders,
			CurrentBatch:  batch + 1,
			TotalBatches:  totalBatches,
			DeletedCount:  stat.DeletedCount,
			MatchedCount:  stat.MatchedCount,
			Status:        "running",
			Message:       fmt.Sprintf("文件夹 %s: 批次 %d/%d 完成，已删除 %d 封", folderName, batch+1, totalBatches, stat.DeletedCount),
		})
	}

	return stat
}

// deleteBatch 删除一批邮件
func (c *Cleaner) deleteBatch(client *imapclient.Client, uids []imap.UID) (int, error) {
	if len(uids) == 0 {
		return 0, nil
	}

	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	// 标记为删除
	storeCmd := client.Store(uidSet, &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}, nil)
	if err := storeCmd.Close(); err != nil {
		return 0, fmt.Errorf("标记删除失败: %w", err)
	}

	// 执行删除
	if err := client.Expunge().Close(); err != nil {
		return 0, fmt.Errorf("执行删除失败: %w", err)
	}

	return len(uids), nil
}
