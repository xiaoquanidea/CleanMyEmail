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
	maxRetries       = 3               // 最大重试次数
	retryInterval    = 2 * time.Second // 重试间隔
	fetchBatchSize   = 100             // 获取邮件头的批次大小
)

// retryResult 重试操作的结果
type retryResult struct {
	conn   *imapClient.PooledConn
	client *imapclient.Client
}

// searchResult 搜索结果
type searchResult struct {
	uids             []imap.UID
	needClientFilter bool // 是否需要客户端过滤发件人
}

// retryWithReconnect 带重连的重试操作
func (c *Cleaner) retryWithReconnect(
	conn *imapClient.PooledConn,
	folderName string,
	operation func(client *imapclient.Client) error,
) (*retryResult, error) {
	client := conn.Client()
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		if c.ctx.Err() != nil {
			return nil, fmt.Errorf("操作已取消")
		}

		if err := operation(client); err == nil {
			return &retryResult{conn: conn, client: client}, nil
		} else {
			lastErr = err
		}

		if retry < maxRetries-1 {
			log.Printf("[DEBUG] 操作失败，%v 后重试 (%d/%d): %v", retryInterval, retry+1, maxRetries, lastErr)
			time.Sleep(retryInterval)

			conn.MarkBad()
			var err error
			if conn, err = c.getConnection(); err != nil {
				return nil, fmt.Errorf("重新获取连接失败: %w", err)
			}
			client = conn.Client()

			if folderName != "" {
				if _, err := client.Select(folderName, nil).Wait(); err != nil {
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
		if c.ctx.Err() != nil {
			return nil, fmt.Errorf("操作已取消")
		}

		if conn, err := c.pool.Get(c.ctx); err == nil {
			return conn, nil
		} else {
			lastErr = err
		}

		if retry < maxRetries-1 {
			log.Printf("[DEBUG] 获取连接失败，%v 后重试 (%d/%d): %v", retryInterval, retry+1, maxRetries, lastErr)
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

	var op, sizeStr string
	switch {
	case strings.HasPrefix(sizeFilter, ">"):
		op, sizeStr = ">", sizeFilter[1:]
	case strings.HasPrefix(sizeFilter, "<"):
		op, sizeStr = "<", sizeFilter[1:]
	default:
		return 0, ""
	}

	sizeStr = strings.ToUpper(sizeStr)
	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(sizeStr, "M"):
		multiplier, sizeStr = 1024*1024, sizeStr[:len(sizeStr)-1]
	case strings.HasSuffix(sizeStr, "K"):
		multiplier, sizeStr = 1024, sizeStr[:len(sizeStr)-1]
	}

	if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
		return size * multiplier, op
	}
	return 0, ""
}

// parseSenders 解析发件人列表
func parseSenders(filterSender string) []string {
	if filterSender == "" {
		return nil
	}
	var senders []string
	for _, s := range strings.Split(filterSender, ",") {
		if s = strings.TrimSpace(s); s != "" {
			senders = append(senders, s)
		}
	}
	return senders
}

// buildOrChain 构建发件人 OR 条件链
func buildOrChain(senders []string) [][2]imap.SearchCriteria {
	if len(senders) < 2 {
		return nil
	}

	n := len(senders)
	inner := imap.SearchCriteria{
		Header: []imap.SearchCriteriaHeaderField{{Key: "From", Value: senders[n-1]}},
	}

	for i := n - 2; i >= 0; i-- {
		current := imap.SearchCriteria{
			Header: []imap.SearchCriteriaHeaderField{{Key: "From", Value: senders[i]}},
		}
		inner = imap.SearchCriteria{Or: [][2]imap.SearchCriteria{{current, inner}}}
	}

	return inner.Or
}

// formatDate 格式化日期用于日志
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02")
}

// cleanFolderContext 清理文件夹的上下文
type cleanFolderContext struct {
	folderName   string
	folderIdx    int
	totalFolders int
	batchSize    int
	startDate    time.Time
	endDate      time.Time
	req          *model.CleanRequest
	senders      []string
	subject      string // 主题关键词
}

// buildBaseCriteria 构建基础搜索条件（仅日期、大小、已读状态）
func (c *Cleaner) buildBaseCriteria(ctx *cleanFolderContext) *imap.SearchCriteria {
	criteria := &imap.SearchCriteria{
		Before: ctx.endDate.AddDate(0, 0, 1), // BEFORE 是"严格早于"，需要 +1 天
	}
	if !ctx.startDate.IsZero() {
		criteria.Since = ctx.startDate
	}

	// 大小筛选
	if size, op := parseSize(ctx.req.FilterSize); size > 0 {
		if op == ">" {
			criteria.Larger = size
		} else {
			criteria.Smaller = size
		}
	}

	// 已读/未读筛选
	switch ctx.req.FilterRead {
	case "seen":
		criteria.Flag = append(criteria.Flag, imap.FlagSeen)
	case "unseen":
		criteria.NotFlag = append(criteria.NotFlag, imap.FlagSeen)
	}

	return criteria
}

// buildFullCriteria 构建完整搜索条件（含发件人和主题）
func (c *Cleaner) buildFullCriteria(ctx *cleanFolderContext) *imap.SearchCriteria {
	criteria := c.buildBaseCriteria(ctx)

	// 主题筛选
	if ctx.subject != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key: "Subject", Value: ctx.subject,
		})
	}

	// 发件人筛选
	switch len(ctx.senders) {
	case 1:
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key: "From", Value: ctx.senders[0],
		})
	default:
		if len(ctx.senders) > 1 {
			criteria.Or = buildOrChain(ctx.senders)
		}
	}

	return criteria
}

// searchEmails 搜索邮件，返回 UID 列表和是否需要客户端过滤
func (c *Cleaner) searchEmails(conn *imapClient.PooledConn, ctx *cleanFolderContext) (*searchResult, *retryResult, error) {
	var result searchResult
	hasFilters := len(ctx.senders) > 0 || ctx.subject != ""

	retryRes, err := c.retryWithReconnect(conn, ctx.folderName, func(cli *imapclient.Client) error {
		// 检查是否已取消
		if c.ctx.Err() != nil {
			return fmt.Errorf("操作已取消")
		}

		// 先尝试完整的服务端搜索
		criteria := c.buildFullCriteria(ctx)
		log.Printf("[DEBUG] [%s] 搜索条件: Since=%s, Before=%s, Header=%+v, Or=%v, senders=%v, subject=%s",
			ctx.folderName, formatDate(criteria.Since), formatDate(criteria.Before),
			criteria.Header, criteria.Or != nil, ctx.senders, ctx.subject)

		searchData, err := cli.UIDSearch(criteria, nil).Wait()
		if err != nil {
			return err
		}
		result.uids = searchData.AllUIDs()
		log.Printf("[DEBUG] [%s] 服务端搜索结果: 找到 %d 封邮件", ctx.folderName, len(result.uids))

		// 如果启用了客户端回退，且有筛选条件但服务端返回 0，可能是服务器不支持某些搜索
		if ctx.req.EnableClientFallback && len(result.uids) == 0 && hasFilters {
			// 检查是否已取消
			if c.ctx.Err() != nil {
				return fmt.Errorf("操作已取消")
			}

			baseCriteria := c.buildBaseCriteria(ctx)
			baseData, baseErr := cli.UIDSearch(baseCriteria, nil).Wait()
			if baseErr != nil {
				return baseErr
			}
			if baseUIDs := baseData.AllUIDs(); len(baseUIDs) > 0 {
				var filterDesc string
				if len(ctx.senders) > 0 && ctx.subject != "" {
					filterDesc = "发件人/主题"
				} else if len(ctx.senders) > 0 {
					filterDesc = "发件人"
				} else {
					filterDesc = "主题"
				}
				log.Printf("[DEBUG] [%s] 服务端不支持 %s 搜索，回退到客户端过滤 (%d 封)", ctx.folderName, filterDesc, len(baseUIDs))
				result.uids = baseUIDs
				result.needClientFilter = true
				// 通知前端正在进行客户端过滤
				c.sendProgress(&model.CleanProgress{
					CurrentFolder: ctx.folderName,
					FolderIndex:   ctx.folderIdx + 1,
					TotalFolders:  ctx.totalFolders,
					Status:        "running",
					Message:       fmt.Sprintf("文件夹 %s: 服务端不支持%s搜索，正在客户端过滤 %d 封邮件...", ctx.folderName, filterDesc, len(baseUIDs)),
				})
			}
		}
		return nil
	})

	return &result, retryRes, err
}

// deleteEmailBatches 分批删除邮件
func (c *Cleaner) deleteEmailBatches(conn *imapClient.PooledConn, ctx *cleanFolderContext, uids []imap.UID, stat *model.FolderCleanStat) {
	totalBatches := (len(uids) + ctx.batchSize - 1) / ctx.batchSize

	for batch := 0; batch < totalBatches; batch++ {
		if c.ctx.Err() != nil {
			stat.Status = "cancelled"
			return
		}

		start := batch * ctx.batchSize
		end := min(start+ctx.batchSize, len(uids))
		batchUIDs := uids[start:end]

		var deleted int
		result, err := c.retryWithReconnect(conn, ctx.folderName, func(cli *imapclient.Client) error {
			var deleteErr error
			deleted, deleteErr = c.deleteBatch(cli, batchUIDs)
			return deleteErr
		})
		if err != nil {
			stat.Status = "failed"
			stat.Error = fmt.Sprintf("删除失败: %v", err)
			return
		}
		conn = result.conn

		stat.DeletedCount += deleted
		c.sendProgress(&model.CleanProgress{
			CurrentFolder: ctx.folderName,
			FolderIndex:   ctx.folderIdx + 1,
			TotalFolders:  ctx.totalFolders,
			CurrentBatch:  batch + 1,
			TotalBatches:  totalBatches,
			DeletedCount:  stat.DeletedCount,
			MatchedCount:  stat.MatchedCount,
			Status:        "running",
			Message:       fmt.Sprintf("文件夹 %s: 批次 %d/%d 完成，已删除 %d 封", ctx.folderName, batch+1, totalBatches, stat.DeletedCount),
		})
	}
}

// sendNoMatchProgress 发送无匹配邮件的进度
func (c *Cleaner) sendNoMatchProgress(ctx *cleanFolderContext, message string) {
	c.sendProgress(&model.CleanProgress{
		CurrentFolder: ctx.folderName,
		FolderIndex:   ctx.folderIdx + 1,
		TotalFolders:  ctx.totalFolders,
		MatchedCount:  0,
		Status:        "running",
		Message:       message,
	})
}

// cleanFolder 清理单个文件夹
func (c *Cleaner) cleanFolder(folderName string, startDate, endDate time.Time, req *model.CleanRequest, folderIdx, totalFolders, batchSize int) model.FolderCleanStat {
	ctx := &cleanFolderContext{
		folderName:   folderName,
		folderIdx:    folderIdx,
		totalFolders: totalFolders,
		batchSize:    batchSize,
		startDate:    startDate,
		endDate:      endDate,
		req:          req,
		senders:      parseSenders(req.FilterSender),
		subject:      strings.TrimSpace(req.FilterSubject),
	}

	stat := model.FolderCleanStat{Folder: folderName, Status: "completed"}

	// 获取连接
	conn, err := c.getConnection()
	if err != nil {
		stat.Status, stat.Error = "failed", fmt.Sprintf("获取连接失败: %v", err)
		return stat
	}
	defer conn.Release()

	// 选择文件夹
	mbox, err := conn.Client().Select(folderName, nil).Wait()
	if err != nil {
		stat.Status, stat.Error = "failed", fmt.Sprintf("选择文件夹失败: %v", err)
		return stat
	}

	if mbox.NumMessages == 0 {
		c.sendNoMatchProgress(ctx, fmt.Sprintf("文件夹 %s 为空", folderName))
		return stat
	}

	// 搜索邮件
	searchRes, retryRes, err := c.searchEmails(conn, ctx)
	if err != nil {
		stat.Status, stat.Error = "failed", fmt.Sprintf("搜索邮件失败: %v", err)
		return stat
	}
	conn = retryRes.conn
	uids := searchRes.uids

	if len(uids) == 0 {
		c.sendNoMatchProgress(ctx, fmt.Sprintf("文件夹 %s 没有符合条件的邮件", folderName))
		return stat
	}

	// 客户端过滤（如果服务端不支持发件人/主题搜索）
	if searchRes.needClientFilter {
		if filteredUIDs, err := c.filterByEnvelope(conn, ctx, uids); err != nil {
			// 如果是取消操作，直接返回
			if c.ctx.Err() != nil {
				stat.Status = "cancelled"
				return stat
			}
			log.Printf("[WARN] [%s] 客户端过滤失败: %v，跳过筛选", folderName, err)
		} else {
			log.Printf("[DEBUG] [%s] 客户端过滤: %d -> %d 封邮件", folderName, len(uids), len(filteredUIDs))
			uids = filteredUIDs
		}
	}

	stat.MatchedCount = len(uids)

	if len(uids) == 0 {
		c.sendNoMatchProgress(ctx, fmt.Sprintf("文件夹 %s 没有符合条件的邮件", folderName))
		return stat
	}

	// 预览模式
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
	c.deleteEmailBatches(conn, ctx, uids, &stat)
	return stat
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

	if err := client.Store(uidSet, &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}, nil).Close(); err != nil {
		return 0, fmt.Errorf("标记删除失败: %w", err)
	}

	if err := client.Expunge().Close(); err != nil {
		return 0, fmt.Errorf("执行删除失败: %w", err)
	}

	return len(uids), nil
}

// filterByEnvelope 根据发件人和主题过滤邮件（客户端过滤）
func (c *Cleaner) filterByEnvelope(conn *imapClient.PooledConn, ctx *cleanFolderContext, uids []imap.UID) ([]imap.UID, error) {
	if len(uids) == 0 {
		return uids, nil
	}
	// 如果没有需要过滤的条件，直接返回
	if len(ctx.senders) == 0 && ctx.subject == "" {
		return uids, nil
	}

	client := conn.Client()
	var filteredUIDs []imap.UID
	totalBatches := (len(uids) + fetchBatchSize - 1) / fetchBatchSize

	// 构建过滤描述
	var filterDesc string
	if len(ctx.senders) > 0 && ctx.subject != "" {
		filterDesc = "发件人/主题"
	} else if len(ctx.senders) > 0 {
		filterDesc = "发件人"
	} else {
		filterDesc = "主题"
	}

	for i := 0; i < len(uids); i += fetchBatchSize {
		if c.ctx.Err() != nil {
			return nil, fmt.Errorf("操作已取消")
		}

		batchNum := i/fetchBatchSize + 1
		end := min(i+fetchBatchSize, len(uids))
		batchUIDs := uids[i:end]

		// 每 10 批或最后一批发送进度
		if batchNum%10 == 0 || batchNum == totalBatches {
			c.sendProgress(&model.CleanProgress{
				CurrentFolder: ctx.folderName,
				FolderIndex:   ctx.folderIdx + 1,
				TotalFolders:  ctx.totalFolders,
				Status:        "running",
				Message:       fmt.Sprintf("文件夹 %s: 过滤%s %d/%d (已匹配 %d 封)", ctx.folderName, filterDesc, end, len(uids), len(filteredUIDs)),
			})
		}

		uidSet := imap.UIDSet{}
		for _, uid := range batchUIDs {
			uidSet.AddNum(uid)
		}

		fetchCmd := client.Fetch(uidSet, &imap.FetchOptions{Envelope: true})
		for msg := fetchCmd.Next(); msg != nil; msg = fetchCmd.Next() {
			var msgUID imap.UID
			for item := msg.Next(); item != nil; item = msg.Next() {
				switch data := item.(type) {
				case imapclient.FetchItemDataUID:
					msgUID = data.UID
				case imapclient.FetchItemDataEnvelope:
					if c.matchEnvelope(data.Envelope, ctx) {
						filteredUIDs = append(filteredUIDs, msgUID)
					}
				}
			}
		}

		if err := fetchCmd.Close(); err != nil {
			return nil, fmt.Errorf("获取邮件头失败: %w", err)
		}
	}

	return filteredUIDs, nil
}

// matchEnvelope 检查邮件信封是否匹配筛选条件
func (c *Cleaner) matchEnvelope(envelope *imap.Envelope, ctx *cleanFolderContext) bool {
	if envelope == nil {
		return false
	}

	// 检查发件人（如果有筛选条件）
	if len(ctx.senders) > 0 {
		if len(envelope.From) == 0 {
			return false
		}
		fromAddr := strings.ToLower(envelope.From[0].Addr())
		matched := false
		for _, sender := range ctx.senders {
			if strings.Contains(fromAddr, strings.ToLower(strings.TrimSpace(sender))) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 检查主题（如果有筛选条件）
	if ctx.subject != "" {
		subject := strings.ToLower(envelope.Subject)
		keyword := strings.ToLower(strings.TrimSpace(ctx.subject))
		if !strings.Contains(subject, keyword) {
			return false
		}
	}

	return true
}