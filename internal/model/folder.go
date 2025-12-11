package model

// MailFolder 邮箱文件夹
type MailFolder struct {
	Name        string        `json:"name"`
	FullPath    string        `json:"fullPath"`
	Delimiter   string        `json:"delimiter"`
	MessageCount uint32       `json:"messageCount"`
	UnseenCount  uint32       `json:"unseenCount"`
	Attributes  []string      `json:"attributes"`
	Children    []*MailFolder `json:"children,omitempty"`
	IsSelectable bool         `json:"isSelectable"`
}

// FolderTreeNode 文件夹树节点（用于前端展示）
type FolderTreeNode struct {
	Key          string            `json:"key"`
	Label        string            `json:"label"`
	FullPath     string            `json:"fullPath"`
	MessageCount uint32            `json:"messageCount"`
	IsLeaf       bool              `json:"isLeaf"`
	Disabled     bool              `json:"disabled"`
	Children     []*FolderTreeNode `json:"children,omitempty"`
}

// CleanRequest 清理请求
type CleanRequest struct {
	AccountID      int64    `json:"accountId"`
	Folders        []string `json:"folders"`
	StartDate      string   `json:"startDate"` // YYYY-MM-DD
	EndDate        string   `json:"endDate"`   // YYYY-MM-DD
	PreviewOnly    bool     `json:"previewOnly"`
	BatchSize      int      `json:"batchSize"`      // 每批处理的邮件数量，默认500
	MaxConcurrency int      `json:"maxConcurrency"` // 最大并发文件夹数，默认5
	// 筛选条件
	FilterSender  string `json:"filterSender"`  // 发件人筛选（支持多个，逗号分隔）
	FilterSubject string `json:"filterSubject"` // 主题关键词筛选
	FilterSize    string `json:"filterSize"`    // 大小筛选：">1M", "<100K" 等
	FilterRead    string `json:"filterRead"`    // 已读/未读：seen, unseen, all
	// 高级选项
	EnableClientFallback bool `json:"enableClientFallback"` // 启用客户端回退（当服务端不支持发件人/主题搜索时）
}

// GetBatchSize 获取批处理大小，使用默认值如果未设置
func (r *CleanRequest) GetBatchSize() int {
	if r.BatchSize <= 0 {
		return 500
	}
	return r.BatchSize
}

// GetMaxConcurrency 获取最大并发数，使用默认值如果未设置
func (r *CleanRequest) GetMaxConcurrency() int {
	if r.MaxConcurrency <= 0 {
		return 3 // 与连接池默认大小匹配，对邮件服务器友好
	}
	return r.MaxConcurrency
}

// CleanProgress 清理进度
type CleanProgress struct {
	AccountID      int64   `json:"accountId"`
	CurrentFolder  string  `json:"currentFolder"`
	FolderIndex    int     `json:"folderIndex"`
	TotalFolders   int     `json:"totalFolders"`
	CurrentBatch   int     `json:"currentBatch"`
	TotalBatches   int     `json:"totalBatches"`
	DeletedCount   int     `json:"deletedCount"`
	MatchedCount   int     `json:"matchedCount"`
	Status         string  `json:"status"` // running, completed, failed, cancelled
	Message        string  `json:"message"`
	ElapsedSeconds float64 `json:"elapsedSeconds"`
}

// CleanResult 清理结果
type CleanResult struct {
	AccountID    int64           `json:"accountId"`
	TotalDeleted int             `json:"totalDeleted"`
	FolderStats  []FolderCleanStat `json:"folderStats"`
	Duration     float64         `json:"duration"`
	Status       string          `json:"status"`
	Error        string          `json:"error,omitempty"`
}

// FolderCleanStat 文件夹清理统计
type FolderCleanStat struct {
	Folder       string `json:"folder"`
	MatchedCount int    `json:"matchedCount"`
	DeletedCount int    `json:"deletedCount"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
}

