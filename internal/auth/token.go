package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

const (
	tokenBufferSecs = 300 // トークン有効期限切れまで5分の余裕を持たせる
)

// TokenResponse トークンレスポンス
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token"`
	Scope        string    `json:"scope"`
	CachedAt     time.Time `json:"cached_at"`
}

// IsExpired トークンが有効期限切れか判定
func (tr *TokenResponse) IsExpired() bool {
	if tr.CachedAt.IsZero() {
		return true
	}

	expirationTime := tr.CachedAt.Add(time.Duration(tr.ExpiresIn) * time.Second)
	// 5分の余裕を持たせる
	return time.Now().Add(time.Duration(tokenBufferSecs) * time.Second).After(expirationTime)
}

// RemainingValidity トークンの残り有効期限を取得
func (tr *TokenResponse) RemainingValidity() time.Duration {
	if tr.CachedAt.IsZero() {
		return 0
	}

	expirationTime := tr.CachedAt.Add(time.Duration(tr.ExpiresIn) * time.Second)
	remaining := time.Until(expirationTime)

	if remaining < 0 {
		return 0
	}

	return remaining
}

// TokenCacheManager トークンキャッシュマネージャー
type TokenCacheManager struct {
	filePath string
	mu       sync.RWMutex
	logger   *log.Logger
}

// NewTokenCacheManager 新しいトークンキャッシュマネージャーを作成
func NewTokenCacheManager(filePath string, logger *log.Logger) *TokenCacheManager {
	return &TokenCacheManager{
		filePath: filePath,
		logger:   logger,
	}
}

// LoadToken トークンをキャッシュから読み込む
func (tcm *TokenCacheManager) LoadToken() (*TokenResponse, error) {
	tcm.mu.RLock()
	defer tcm.mu.RUnlock()

	data, err := os.ReadFile(tcm.filePath)
	if err != nil {
		tcm.logger.Debug("キャッシュファイル読み込み失敗", "error", err)
		return nil, err
	}

	var token TokenResponse
	if err := json.Unmarshal(data, &token); err != nil {
		tcm.logger.Error("キャッシュJSON解析失敗", "error", err)
		return nil, err
	}

	// トークンの有効期限をチェック
	if token.IsExpired() {
		tcm.logger.Debug("キャッシュトークンは期限切れです")
		return nil, fmt.Errorf("token expired")
	}

	tcm.logger.Debug("キャッシュトークン読み込み成功")
	return &token, nil
}

// SaveToken トークンをキャッシュに保存
func (tcm *TokenCacheManager) SaveToken(token *TokenResponse) error {
	tcm.mu.Lock()
	defer tcm.mu.Unlock()

	// トークンの取得時刻を記録
	token.CachedAt = time.Now()

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		tcm.logger.Error("キャッシュJSON作成失敗", "error", err)
		return err
	}

	// ファイルを安全に書き込む（0600: 所有者のみ読み書き可能）
	if err := os.WriteFile(tcm.filePath, data, 0600); err != nil {
		tcm.logger.Error("キャッシュファイル書き込み失敗", "error", err)
		return err
	}

	tcm.logger.Debug("トークンをキャッシュに保存しました")
	return nil
}

// ClearCache トークンキャッシュをクリア
func (tcm *TokenCacheManager) ClearCache() error {
	tcm.mu.Lock()
	defer tcm.mu.Unlock()

	if err := os.Remove(tcm.filePath); err != nil && !os.IsNotExist(err) {
		tcm.logger.Error("キャッシュ削除失敗", "error", err)
		return err
	}

	tcm.logger.Debug("トークンキャッシュをクリアしました")
	return nil
}
