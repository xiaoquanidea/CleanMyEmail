package db

import (
	"database/sql"
	"time"

	"CleanMyEmail/internal/model"
)

// SaveToken 保存或更新Token
func SaveToken(token *model.OAuth2Token) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	// 先尝试更新
	result, err := db.Exec(`
		UPDATE oauth2_tokens 
		SET access_token = ?, refresh_token = ?, token_type = ?, expires_at = ?, auth_status = ?, error_message = ?, updated_at = ?
		WHERE account_id = ?
	`, token.AccessToken, token.RefreshToken, token.TokenType, token.ExpiresAt,
		token.AuthStatus, token.ErrorMessage, time.Now(), token.AccountID)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		return nil
	}

	// 不存在则插入
	_, err = db.Exec(`
		INSERT INTO oauth2_tokens (account_id, provider, access_token, refresh_token, token_type, expires_at, auth_status, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, token.AccountID, token.Provider, token.AccessToken, token.RefreshToken,
		token.TokenType, token.ExpiresAt, token.AuthStatus, token.ErrorMessage)
	return err
}

// GetTokenByAccountID 根据账号ID获取Token
func GetTokenByAccountID(accountID int64) (*model.OAuth2Token, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	token := &model.OAuth2Token{}
	var expiresAt sql.NullTime

	err = db.QueryRow(`
		SELECT id, account_id, provider, access_token, refresh_token, token_type, expires_at, auth_status, error_message, created_at, updated_at
		FROM oauth2_tokens WHERE account_id = ?
	`, accountID).Scan(&token.ID, &token.AccountID, &token.Provider, &token.AccessToken,
		&token.RefreshToken, &token.TokenType, &expiresAt, &token.AuthStatus,
		&token.ErrorMessage, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if expiresAt.Valid {
		token.ExpiresAt = &expiresAt.Time
	}
	return token, nil
}

// DeleteTokenByAccountID 删除账号的Token
func DeleteTokenByAccountID(accountID int64) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM oauth2_tokens WHERE account_id = ?", accountID)
	return err
}

// UpdateTokenStatus 更新Token状态
func UpdateTokenStatus(accountID int64, status model.OAuth2AuthStatus, errorMsg string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		UPDATE oauth2_tokens SET auth_status = ?, error_message = ?, updated_at = ? WHERE account_id = ?
	`, status, errorMsg, time.Now(), accountID)
	return err
}

// IsTokenExpired 检查Token是否过期
func IsTokenExpired(accountID int64) (bool, error) {
	token, err := GetTokenByAccountID(accountID)
	if err != nil {
		return true, err
	}
	if token == nil {
		return true, nil
	}
	if token.ExpiresAt == nil {
		return false, nil
	}
	// 提前5分钟认为过期
	return time.Now().Add(5 * time.Minute).After(*token.ExpiresAt), nil
}

