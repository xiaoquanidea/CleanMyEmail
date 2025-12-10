package db

import (
	"database/sql"
	"sync"

	"CleanMyEmail/internal/config"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db     *sql.DB
	dbOnce sync.Once
)

// GetDB 获取数据库连接
func GetDB() (*sql.DB, error) {
	var err error
	dbOnce.Do(func() {
		if err = config.EnsureDataDir(); err != nil {
			return
		}
		db, err = sql.Open("sqlite3", config.GetDBPath())
		if err != nil {
			return
		}
		err = initTables()
	})
	return db, err
}

// initTables 初始化数据库表
func initTables() error {
	createTableSQL := `
	-- 邮箱账号表
	CREATE TABLE IF NOT EXISTS email_accounts (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		email           TEXT NOT NULL UNIQUE,
		display_name    TEXT,
		vendor          TEXT NOT NULL,
		auth_type       TEXT NOT NULL,
		imap_server     TEXT NOT NULL,
		password        TEXT,
		status          TEXT DEFAULT 'active',
		last_connected  DATETIME,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- OAuth2 Token 表
	CREATE TABLE IF NOT EXISTS oauth2_tokens (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id          INTEGER NOT NULL,
		provider            TEXT NOT NULL,
		access_token        TEXT NOT NULL,
		refresh_token       TEXT,
		token_type          TEXT DEFAULT 'Bearer',
		expires_at          DATETIME,
		auth_status         TEXT DEFAULT 'active',
		error_message       TEXT,
		created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (account_id) REFERENCES email_accounts(id) ON DELETE CASCADE
	);

	-- 清理历史记录表
	CREATE TABLE IF NOT EXISTS clean_history (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id      INTEGER NOT NULL,
		account_email   TEXT NOT NULL,
		folders         TEXT NOT NULL,
		folder_count    INTEGER DEFAULT 0,
		date_range      TEXT,
		filter_sender   TEXT,
		filter_subject  TEXT,
		filter_size     TEXT,
		filter_read     TEXT,
		matched_count   INTEGER DEFAULT 0,
		deleted_count   INTEGER DEFAULT 0,
		preview_only    INTEGER DEFAULT 0,
		start_time      DATETIME NOT NULL,
		end_time        DATETIME,
		duration        REAL DEFAULT 0,
		status          TEXT DEFAULT 'running',
		error_message   TEXT,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (account_id) REFERENCES email_accounts(id) ON DELETE CASCADE
	);

	-- OAuth2 配置表（存储 ClientID/ClientSecret）
	CREATE TABLE IF NOT EXISTS oauth2_configs (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		vendor          TEXT NOT NULL UNIQUE,
		client_id       TEXT NOT NULL,
		client_secret   TEXT NOT NULL,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 应用设置表
	CREATE TABLE IF NOT EXISTS app_settings (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		key             TEXT NOT NULL UNIQUE,
		value           TEXT NOT NULL,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_oauth2_tokens_account_id ON oauth2_tokens(account_id);
	CREATE INDEX IF NOT EXISTS idx_clean_history_account_id ON clean_history(account_id);
	`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	// 初始化默认 OAuth2 配置（如果不存在）
	initDefaultOAuth2Configs()
	return nil
}

// initDefaultOAuth2Configs 初始化默认 OAuth2 配置
func initDefaultOAuth2Configs() {
	defaultConfigs := []struct {
		Vendor       string
		ClientID     string
		ClientSecret string
	}{
		{
			Vendor:       "outlook",
			ClientID:     "851db9a2-1d8e-4647-b91d-f6ba047022c6",
			ClientSecret: "7NY8Q~pWlfFjHrMhpOt3Yj03eqFPXtp4NojVca2x",
		},
		{
			Vendor:       "gmail",
			ClientID:     "39994161098-q5304h08f7maltchcs024kfl7v2o0r0l.apps.googleusercontent.com",
			ClientSecret: "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",
		},
	}

	for _, cfg := range defaultConfigs {
		// 检查是否已存在
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM oauth2_configs WHERE vendor = ?`, cfg.Vendor).Scan(&count)
		if count == 0 {
			db.Exec(`INSERT INTO oauth2_configs (vendor, client_id, client_secret) VALUES (?, ?, ?)`,
				cfg.Vendor, cfg.ClientID, cfg.ClientSecret)
		}
	}
}

// Close 关闭数据库连接
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// OAuth2ConfigRecord OAuth2配置记录
type OAuth2ConfigRecord struct {
	Vendor       string
	ClientID     string
	ClientSecret string
}

// GetOAuth2Config 获取OAuth2配置
func GetOAuth2Config(vendor string) (*OAuth2ConfigRecord, error) {
	database, err := GetDB()
	if err != nil {
		return nil, err
	}

	var config OAuth2ConfigRecord
	err = database.QueryRow(`
		SELECT vendor, client_id, client_secret FROM oauth2_configs WHERE vendor = ?
	`, vendor).Scan(&config.Vendor, &config.ClientID, &config.ClientSecret)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// SaveOAuth2Config 保存OAuth2配置
func SaveOAuth2Config(vendor, clientID, clientSecret string) error {
	database, err := GetDB()
	if err != nil {
		return err
	}

	// 先尝试更新
	result, err := database.Exec(`
		UPDATE oauth2_configs SET client_id = ?, client_secret = ?, updated_at = CURRENT_TIMESTAMP WHERE vendor = ?
	`, clientID, clientSecret, vendor)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		return nil
	}

	// 不存在则插入
	_, err = database.Exec(`
		INSERT INTO oauth2_configs (vendor, client_id, client_secret) VALUES (?, ?, ?)
	`, vendor, clientID, clientSecret)
	return err
}

