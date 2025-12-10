package model

import "time"

// CleanHistory 清理历史记录
type CleanHistory struct {
	ID            int64     `json:"id"`
	AccountID     int64     `json:"accountId"`
	AccountEmail  string    `json:"accountEmail"`
	Folders       string    `json:"folders"`       // JSON 数组
	FolderCount   int       `json:"folderCount"`
	DateRange     string    `json:"dateRange"`     // 如 "2024-01-01 ~ 2024-06-01"
	FilterSender  string    `json:"filterSender"`  // 发件人筛选
	FilterSubject string    `json:"filterSubject"` // 主题筛选
	FilterSize    string    `json:"filterSize"`    // 大小筛选
	FilterRead    string    `json:"filterRead"`    // 已读/未读筛选
	MatchedCount  int       `json:"matchedCount"`
	DeletedCount  int       `json:"deletedCount"`
	PreviewOnly   bool      `json:"previewOnly"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	Duration      float64   `json:"duration"` // 秒
	Status        string    `json:"status"`   // running, completed, failed, cancelled
	ErrorMessage  string    `json:"errorMessage,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// CleanHistoryListItem 历史记录列表项（简化版）
type CleanHistoryListItem struct {
	ID           int64     `json:"id"`
	AccountEmail string    `json:"accountEmail"`
	FolderCount  int       `json:"folderCount"`
	DateRange    string    `json:"dateRange"`
	MatchedCount int       `json:"matchedCount"`
	DeletedCount int       `json:"deletedCount"`
	PreviewOnly  bool      `json:"previewOnly"`
	Duration     float64   `json:"duration"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

