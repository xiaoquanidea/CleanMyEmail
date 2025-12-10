package db

import (
	"database/sql"
	"encoding/json"

	"CleanMyEmail/internal/model"
)

// GetAppSettings 获取应用设置
func GetAppSettings() (*model.AppSettings, error) {
	database, err := GetDB()
	if err != nil {
		return nil, err
	}

	var settingsJSON string
	err = database.QueryRow(`SELECT value FROM app_settings WHERE key = 'global'`).Scan(&settingsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			// 返回默认设置
			return model.DefaultAppSettings(), nil
		}
		return nil, err
	}

	var settings model.AppSettings
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return model.DefaultAppSettings(), nil
	}

	return &settings, nil
}

// SaveAppSettings 保存应用设置
func SaveAppSettings(settings *model.AppSettings) error {
	database, err := GetDB()
	if err != nil {
		return err
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	// 先尝试更新
	result, err := database.Exec(`
		UPDATE app_settings SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE key = 'global'
	`, string(settingsJSON))
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		return nil
	}

	// 不存在则插入
	_, err = database.Exec(`
		INSERT INTO app_settings (key, value) VALUES ('global', ?)
	`, string(settingsJSON))
	return err
}

// GetProxySettings 获取代理设置
func GetProxySettings() (*model.ProxySettings, error) {
	settings, err := GetAppSettings()
	if err != nil {
		return nil, err
	}
	return &settings.Proxy, nil
}

// SaveProxySettings 保存代理设置
func SaveProxySettings(proxy *model.ProxySettings) error {
	settings, err := GetAppSettings()
	if err != nil {
		settings = model.DefaultAppSettings()
	}
	settings.Proxy = *proxy
	return SaveAppSettings(settings)
}

