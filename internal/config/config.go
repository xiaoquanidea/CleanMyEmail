package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var (
	cfg     *AppConfig
	cfgOnce sync.Once
	cfgMu   sync.RWMutex
)

// AppConfig 应用配置
type AppConfig struct {
	DataDir       string         `json:"dataDir"`
	LogLevel      string         `json:"logLevel"`
	OAuth2Configs map[string]OAuth2ProviderConfig `json:"oauth2Configs"`
}

// OAuth2ProviderConfig OAuth2提供商配置
type OAuth2ProviderConfig struct {
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	AuthURL      string   `json:"authUrl"`
	TokenURL     string   `json:"tokenUrl"`
	Scopes       []string `json:"scopes"`
}

// GetDataDir 获取数据目录
func GetDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".CleanMyEmail"
	}
	return filepath.Join(homeDir, ".CleanMyEmail")
}

// GetDBPath 获取数据库路径
func GetDBPath() string {
	return filepath.Join(GetDataDir(), "cleanmyemail.db")
}

// GetConfigPath 获取配置文件路径
func GetConfigPath() string {
	return filepath.Join(GetDataDir(), "config.json")
}

// EnsureDataDir 确保数据目录存在
func EnsureDataDir() error {
	dataDir := GetDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	// 创建日志目录
	logDir := filepath.Join(dataDir, "logs")
	return os.MkdirAll(logDir, 0755)
}

// GetConfig 获取配置
func GetConfig() *AppConfig {
	cfgOnce.Do(func() {
		cfg = loadConfig()
	})
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

// loadConfig 加载配置
func loadConfig() *AppConfig {
	config := &AppConfig{
		DataDir:       GetDataDir(),
		LogLevel:      "info",
		OAuth2Configs: make(map[string]OAuth2ProviderConfig),
	}

	configPath := GetConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config
	}

	_ = json.Unmarshal(data, config)
	return config
}

// SaveConfig 保存配置
func SaveConfig(config *AppConfig) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	if err := EnsureDataDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	cfg = config
	return os.WriteFile(GetConfigPath(), data, 0644)
}

// GetOAuth2Config 获取OAuth2配置
func GetOAuth2Config(provider string) (OAuth2ProviderConfig, bool) {
	config := GetConfig()
	if config.OAuth2Configs == nil {
		return OAuth2ProviderConfig{}, false
	}
	cfg, ok := config.OAuth2Configs[provider]
	return cfg, ok
}

