package db

import (
	"database/sql"
	"time"

	"CleanMyEmail/internal/model"
)

// CreateAccount 创建账号
func CreateAccount(account *model.EmailAccount) (int64, error) {
	db, err := GetDB()
	if err != nil {
		return 0, err
	}

	result, err := db.Exec(`
		INSERT INTO email_accounts (email, display_name, vendor, auth_type, imap_server, password, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, account.Email, account.DisplayName, account.Vendor, account.AuthType,
		account.IMAPServer, account.Password, account.Status)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// GetAccountByID 根据ID获取账号
func GetAccountByID(id int64) (*model.EmailAccount, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	account := &model.EmailAccount{}
	var lastConnected sql.NullTime

	err = db.QueryRow(`
		SELECT id, email, display_name, vendor, auth_type, imap_server, password, status, last_connected, created_at, updated_at
		FROM email_accounts WHERE id = ?
	`, id).Scan(&account.ID, &account.Email, &account.DisplayName, &account.Vendor,
		&account.AuthType, &account.IMAPServer, &account.Password,
		&account.Status, &lastConnected, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if lastConnected.Valid {
		account.LastConnected = &lastConnected.Time
	}
	return account, nil
}

// GetAccountByEmail 根据邮箱获取账号
func GetAccountByEmail(email string) (*model.EmailAccount, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	account := &model.EmailAccount{}
	var lastConnected sql.NullTime

	err = db.QueryRow(`
		SELECT id, email, display_name, vendor, auth_type, imap_server, password, status, last_connected, created_at, updated_at
		FROM email_accounts WHERE email = ?
	`, email).Scan(&account.ID, &account.Email, &account.DisplayName, &account.Vendor,
		&account.AuthType, &account.IMAPServer, &account.Password,
		&account.Status, &lastConnected, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if lastConnected.Valid {
		account.LastConnected = &lastConnected.Time
	}
	return account, nil
}

// ListAccounts 获取所有账号
func ListAccounts() ([]*model.AccountListItem, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT id, email, display_name, vendor, auth_type, status, last_connected
		FROM email_accounts ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*model.AccountListItem
	for rows.Next() {
		account := &model.AccountListItem{}
		var lastConnected sql.NullTime
		err := rows.Scan(&account.ID, &account.Email, &account.DisplayName, &account.Vendor,
			&account.AuthType, &account.Status, &lastConnected)
		if err != nil {
			return nil, err
		}
		if lastConnected.Valid {
			account.LastConnected = &lastConnected.Time
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// UpdateAccount 更新账号
func UpdateAccount(account *model.EmailAccount) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		UPDATE email_accounts
		SET email = ?, display_name = ?, vendor = ?, auth_type = ?, imap_server = ?, password = ?, status = ?, updated_at = ?
		WHERE id = ?
	`, account.Email, account.DisplayName, account.Vendor, account.AuthType,
		account.IMAPServer, account.Password, account.Status, time.Now(), account.ID)
	return err
}

// DeleteAccount 删除账号
func DeleteAccount(id int64) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM email_accounts WHERE id = ?", id)
	return err
}

// UpdateAccountStatus 更新账号状态
func UpdateAccountStatus(id int64, status model.AccountStatus) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("UPDATE email_accounts SET status = ?, updated_at = ? WHERE id = ?", status, time.Now(), id)
	return err
}

// UpdateAccountLastConnected 更新最后连接时间
func UpdateAccountLastConnected(id int64) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	now := time.Now()
	_, err = db.Exec("UPDATE email_accounts SET last_connected = ?, updated_at = ? WHERE id = ?", now, now, id)
	return err
}

