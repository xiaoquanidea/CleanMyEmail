package service

import (
	"database/sql"
	"encoding/json"
	"time"

	"CleanMyEmail/internal/db"
	"CleanMyEmail/internal/model"
)

// HistoryService 历史记录服务
type HistoryService struct{}

// NewHistoryService 创建历史记录服务
func NewHistoryService() *HistoryService {
	return &HistoryService{}
}

// CreateHistory 创建历史记录
func (s *HistoryService) CreateHistory(req *model.CleanRequest, accountEmail string) (int64, error) {
	database, err := db.GetDB()
	if err != nil {
		return 0, err
	}

	foldersJSON, _ := json.Marshal(req.Folders)
	dateRange := ""
	if req.StartDate != "" {
		dateRange = req.StartDate + " ~ " + req.EndDate
	} else {
		dateRange = "~ " + req.EndDate
	}

	result, err := database.Exec(`
		INSERT INTO clean_history (
			account_id, account_email, folders, folder_count, date_range,
			filter_sender, filter_subject, filter_size, filter_read,
			preview_only, start_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.AccountID, accountEmail, string(foldersJSON), len(req.Folders), dateRange,
		req.FilterSender, req.FilterSubject, req.FilterSize, req.FilterRead,
		req.PreviewOnly, time.Now(), "running")
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// UpdateHistory 更新历史记录
func (s *HistoryService) UpdateHistory(id int64, matchedCount, deletedCount int, status, errorMsg string, duration float64) error {
	database, err := db.GetDB()
	if err != nil {
		return err
	}

	_, err = database.Exec(`
		UPDATE clean_history SET
			matched_count = ?, deleted_count = ?, status = ?, error_message = ?,
			duration = ?, end_time = ?
		WHERE id = ?
	`, matchedCount, deletedCount, status, errorMsg, duration, time.Now(), id)
	return err
}

// GetHistoryList 获取历史记录列表
func (s *HistoryService) GetHistoryList(limit, offset int) ([]model.CleanHistoryListItem, error) {
	database, err := db.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := database.Query(`
		SELECT id, account_email, folder_count, date_range, matched_count, deleted_count,
			   preview_only, duration, status, created_at
		FROM clean_history
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.CleanHistoryListItem
	for rows.Next() {
		var item model.CleanHistoryListItem
		var previewOnly int
		err := rows.Scan(
			&item.ID, &item.AccountEmail, &item.FolderCount, &item.DateRange,
			&item.MatchedCount, &item.DeletedCount, &previewOnly,
			&item.Duration, &item.Status, &item.CreatedAt,
		)
		if err != nil {
			continue
		}
		item.PreviewOnly = previewOnly == 1
		list = append(list, item)
	}
	return list, nil
}

// GetHistoryDetail 获取历史记录详情
func (s *HistoryService) GetHistoryDetail(id int64) (*model.CleanHistory, error) {
	database, err := db.GetDB()
	if err != nil {
		return nil, err
	}

	var h model.CleanHistory
	var previewOnly int
	var endTime sql.NullTime
	var errorMsg sql.NullString

	err = database.QueryRow(`
		SELECT id, account_id, account_email, folders, folder_count, date_range,
			   filter_sender, filter_subject, filter_size, filter_read,
			   matched_count, deleted_count, preview_only, start_time, end_time,
			   duration, status, error_message, created_at
		FROM clean_history WHERE id = ?
	`, id).Scan(
		&h.ID, &h.AccountID, &h.AccountEmail, &h.Folders, &h.FolderCount, &h.DateRange,
		&h.FilterSender, &h.FilterSubject, &h.FilterSize, &h.FilterRead,
		&h.MatchedCount, &h.DeletedCount, &previewOnly, &h.StartTime, &endTime,
		&h.Duration, &h.Status, &errorMsg, &h.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	h.PreviewOnly = previewOnly == 1
	if endTime.Valid {
		h.EndTime = endTime.Time
	}
	if errorMsg.Valid {
		h.ErrorMessage = errorMsg.String
	}
	return &h, nil
}

// DeleteHistory 删除历史记录
func (s *HistoryService) DeleteHistory(id int64) error {
	database, err := db.GetDB()
	if err != nil {
		return err
	}
	_, err = database.Exec(`DELETE FROM clean_history WHERE id = ?`, id)
	return err
}

// ClearAllHistory 清空所有历史记录
func (s *HistoryService) ClearAllHistory() error {
	database, err := db.GetDB()
	if err != nil {
		return err
	}
	_, err = database.Exec(`DELETE FROM clean_history`)
	return err
}

