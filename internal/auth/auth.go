package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/charmbracelet/log"
	abstractions "github.com/microsoft/kiota-abstractions-go"
)

// Authenticator OAuth認証を管理
type Authenticator struct {
	clientID     string
	redirectURI  string
	authorityURL string
	tokenCache   *TokenCacheManager
	logger       *log.Logger

	codeVerifier  string
	codeChallenge string
	authCode      chan string
	server        *http.Server
}

// NewAuthenticator 新しい認証マネージャーを作成
func NewAuthenticator(clientID, redirectURI, authorityURL, tokenCachePath string, logger *log.Logger) *Authenticator {
	return &Authenticator{
		clientID:     clientID,
		redirectURI:  redirectURI,
		authorityURL: authorityURL,
		tokenCache:   NewTokenCacheManager(tokenCachePath, logger),
		logger:       logger,
		authCode:     make(chan string),
	}
}

// GetAccessToken アクセストークンを取得（キャッシュまたは新規取得）
func (a *Authenticator) GetAccessToken() (string, error) {
	// キャッシュからトークンを読み込む
	cachedToken, err := a.tokenCache.LoadToken()
	if err == nil && cachedToken != nil && !cachedToken.IsExpired() {
		a.logger.Info("キャッシュからトークンを読み込みました", "remaining", cachedToken.RemainingValidity())
		return cachedToken.AccessToken, nil
	}

	a.logger.Debug("新しいトークンを取得します")

	// 新しいトークンを取得
	token, err := a.acquireNewToken()
	if err != nil {
		return "", fmt.Errorf("トークン取得エラー: %w", err)
	}

	// キャッシュに保存
	if err := a.tokenCache.SaveToken(token); err != nil {
		a.logger.Warn("トークンキャッシュ保存失敗", "error", err)
	}

	return token.AccessToken, nil
}

// acquireNewToken 新しいトークンを取得
func (a *Authenticator) acquireNewToken() (*TokenResponse, error) {
	a.generatePKCE()

	authURL, err := a.buildAuthorizationURL()
	if err != nil {
		return nil, fmt.Errorf("認証URL生成エラー: %w", err)
	}

	a.logger.Info("ブラウザで以下のURLを開いてください")
	fmt.Println(authURL)

	// コールバックサーバーを起動
	if err := a.startCallbackServer(); err != nil {
		return nil, fmt.Errorf("コールバックサーバー起動エラー: %w", err)
	}
	defer a.stopCallbackServer()

	// 認証コードを待機（タイムアウト5分）
	select {
	case code := <-a.authCode:
		a.logger.Debug("認証コード受け取り", "code_length", len(code))
		token, err := a.exchangeCodeForToken(code)
		if err != nil {
			return nil, fmt.Errorf("トークン交換エラー: %w", err)
		}
		a.logger.Info("アクセストークン取得成功")
		return token, nil
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("認証タイムアウト")
	}
}

// generatePKCE PKCE用のコード生成
func (a *Authenticator) generatePKCE() {
	b := make([]byte, 32)
	rand.Read(b)
	a.codeVerifier = base64.RawURLEncoding.EncodeToString(b)

	h := sha256.New()
	h.Write([]byte(a.codeVerifier))
	a.codeChallenge = base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	a.logger.Debug("PKCE生成完了")
}

// buildAuthorizationURL 認証URLを構築
func (a *Authenticator) buildAuthorizationURL() (string, error) {
	baseURL := fmt.Sprintf("%s/oauth2/v2.0/authorize", a.authorityURL)

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("client_id", a.clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", a.redirectURI)
	q.Set("scope", "User.Read Mail.Send Mail.ReadWrite offline_access")
	q.Set("code_challenge", a.codeChallenge)
	q.Set("code_challenge_method", "S256")
	q.Set("response_mode", "query")
	q.Set("prompt", "select_account")

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// startCallbackServer コールバックサーバーを起動
func (a *Authenticator) startCallbackServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", a.callbackHandler)

	a.server = &http.Server{
		Addr:    "localhost:5225",
		Handler: mux,
	}

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("コールバックサーバーエラー", "error", err)
		}
	}()

	a.logger.Debug("コールバックサーバー起動", "addr", a.server.Addr)
	return nil
}

// stopCallbackServer コールバックサーバーを停止
func (a *Authenticator) stopCallbackServer() {
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.server.Shutdown(ctx)
		a.logger.Debug("コールバックサーバー停止")
	}
}

// callbackHandler 認証コールバックハンドラ
func (a *Authenticator) callbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := r.URL.Query().Get("error")
		errDesc := r.URL.Query().Get("error_description")
		a.logger.Error("認証エラー", "error", errMsg, "description", errDesc)
		http.Error(w, fmt.Sprintf("認証エラー: %s - %s", errMsg, errDesc), http.StatusBadRequest)
		return
	}

	a.logger.Debug("認証コード取得", "code_length", len(code))
	a.authCode <- code
	fmt.Fprintf(w, "認証が完了しました。このウィンドウを閉じてください。")
}

// exchangeCodeForToken 認証コードをトークンに交換
func (a *Authenticator) exchangeCodeForToken(code string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/oauth2/v2.0/token", a.authorityURL)
	a.logger.Debug("トークン交換開始", "url", tokenURL)

	data := url.Values{}
	data.Set("client_id", a.clientID)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", a.redirectURI)
	data.Set("code_verifier", a.codeVerifier)

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		a.logger.Error("トークン取得失敗", "status", resp.StatusCode, "response", string(body))
		return nil, fmt.Errorf("トークン取得失敗 (status: %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("JSONパースエラー: %w", err)
	}

	a.logger.Info("トークン取得成功", "scope", tokenResp.Scope)
	return &tokenResp, nil
}

// BearerTokenAuthenticationProvider Bearer トークン認証プロバイダー
type BearerTokenAuthenticationProvider struct {
	accessToken string
	logger      *log.Logger
}

// NewBearerTokenAuthenticationProvider 新しい認証プロバイダーを作成
func NewBearerTokenAuthenticationProvider(accessToken string, logger *log.Logger) *BearerTokenAuthenticationProvider {
	return &BearerTokenAuthenticationProvider{
		accessToken: accessToken,
		logger:      logger,
	}
}

// AuthenticateRequest リクエストに認証ヘッダーを追加
func (p *BearerTokenAuthenticationProvider) AuthenticateRequest(ctx context.Context, request *abstractions.RequestInformation, additionalAuthenticationContext map[string]interface{}) error {
	if request == nil {
		return fmt.Errorf("request cannot be nil")
	}

	request.Headers.Add("Authorization", "Bearer "+p.accessToken)
	return nil
}

// StaticTokenCredential 静的トークン認証情報
type StaticTokenCredential struct {
	token     string
	expiresOn time.Time
}

// NewStaticTokenCredential 新しい静的トークン認証情報を作成
func NewStaticTokenCredential(token string) *StaticTokenCredential {
	return &StaticTokenCredential{
		token:     token,
		expiresOn: time.Now().Add(1 * time.Hour),
	}
}

// GetToken トークンを取得
func (c *StaticTokenCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     c.token,
		ExpiresOn: c.expiresOn,
	}, nil
}
