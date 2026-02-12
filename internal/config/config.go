package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"
)

const (
	ConfigDirName  = ".msgraph-smtp"
	ConfigFileName = "config.json"
)

// Config SMTPサーバとMicrosoft Graphの設定
type Config struct {
	SMTP  SMTPConfig  `json:"smtp"`
	Graph GraphConfig `json:"graph"`
}

// SMTPConfig SMTP関連の設定
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// GraphConfig Microsoft Graph関連の設定
type GraphConfig struct {
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri"`
	AuthorityURL string `json:"authority_url"`
	TokenCache   string `json:"token_cache"`
}

// Manager 設定ファイルマネージャー
type Manager struct {
	configPath string
	config     *Config
	mu         sync.RWMutex
	logger     *log.Logger
}

// NewManager 新しい設定マネージャーを作成
func NewManager(logger *log.Logger) (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("ホームディレクトリ取得エラー: %w", err)
	}

	configDir := filepath.Join(home, ConfigDirName)
	configPath := filepath.Join(configDir, ConfigFileName)

	// 設定ディレクトリが存在しない場合は作成
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("設定ディレクトリ作成エラー: %w", err)
	}

	manager := &Manager{
		configPath: configPath,
		logger:     logger,
	}

	// 設定ファイルを読み込む（存在しない場合は初期化）
	if err := manager.load(); err != nil {
		if os.IsNotExist(err) {
			logger.Info("設定ファイルが存在しないため、新規作成します")
			if err := manager.initialize(); err != nil {
				return nil, fmt.Errorf("設定初期化エラー: %w", err)
			}
		} else {
			return nil, fmt.Errorf("設定読み込みエラー: %w", err)
		}
	}

	return manager, nil
}

// initialize 初期設定を作成
func (m *Manager) initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// ランダムなパスワードを生成
	password, err := generatePassword(32)
	if err != nil {
		return fmt.Errorf("パスワード生成エラー: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("ホームディレクトリ取得エラー: %w", err)
	}

	tokenCachePath := filepath.Join(home, ConfigDirName, "token_cache.json")

	m.config = &Config{
		SMTP: SMTPConfig{
			Host:     "localhost",
			Port:     2525,
			Username: "msgraph",
			Password: password,
		},
		Graph: GraphConfig{
			ClientID:     "b1fac4bf-c5c6-4170-89e0-7a7bb9ef35f2",
			RedirectURI:  "http://localhost:5225/callback",
			AuthorityURL: "https://login.microsoftonline.com/common",
			TokenCache:   tokenCachePath,
		},
	}

	return m.save()
}

// load 設定ファイルを読み込む
func (m *Manager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("JSON解析エラー: %w", err)
	}

	m.config = &config
	m.logger.Debug("設定ファイル読み込み成功", "path", m.configPath)
	return nil
}

// save 設定ファイルに保存
func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON作成エラー: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("ファイル書き込みエラー: %w", err)
	}

	m.logger.Debug("設定ファイル保存成功", "path", m.configPath)
	return nil
}

// GetConfig 設定を取得
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetSMTPConfig SMTP設定を取得
func (m *Manager) GetSMTPConfig() SMTPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.SMTP
}

// GetGraphConfig Graph設定を取得
func (m *Manager) GetGraphConfig() GraphConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Graph
}

// UpdateSMTPPort SMTPポートを更新
func (m *Manager) UpdateSMTPPort(port int) error {
	m.mu.Lock()
	m.config.SMTP.Port = port
	m.mu.Unlock()
	return m.save()
}

// GetConfigPath 設定ファイルのパスを取得
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// generatePassword ランダムなパスワードを生成
func generatePassword(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:length], nil
}
