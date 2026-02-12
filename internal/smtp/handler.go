package smtp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"

	"github.com/canaria-computer/m3bridge/internal/graph"
	"github.com/charmbracelet/log"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// Backend SMTPバックエンド
type Backend struct {
	graphClient *graph.Client
	username    string
	password    string
	logger      *log.Logger
}

// NewBackend 新しいバックエンドを作成
func NewBackend(graphClient *graph.Client, username, password string, logger *log.Logger) *Backend {
	return &Backend{
		graphClient: graphClient,
		username:    username,
		password:    password,
		logger:      logger,
	}
}

// NewSession 新しいSMTPセッションを作成
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	b.logger.Debug("新しいSMTPセッション開始")
	return &Session{
		backend: b,
		logger:  b.logger,
	}, nil
}

// Session SMTPセッション
type Session struct {
	backend       *Backend
	from          string
	to            []string
	logger        *log.Logger
	authenticated bool
}

// Reset セッションをリセット
func (s *Session) Reset() {
	s.from = ""
	s.to = nil
	s.logger.Debug("セッションリセット")
}

// Logout セッションを終了
func (s *Session) Logout() error {
	s.logger.Debug("セッション終了")
	return nil
}

// AuthMechanisms サポートする認証メカニズムを返す
func (s *Session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

// Auth 認証を実行
func (s *Session) Auth(mech string) (sasl.Server, error) {
	if mech != sasl.Plain {
		return nil, fmt.Errorf("unsupported auth mechanism")
	}
	
	return sasl.NewPlainServer(func(identity, username, password string) error {
		if username != s.backend.username || password != s.backend.password {
			s.logger.Warn("認証失敗", "username", username)
			return fmt.Errorf("invalid credentials")
		}
		s.logger.Debug("認証成功", "username", username)
		s.authenticated = true
		return nil
	}), nil
}

// Mail 送信者を設定
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	s.logger.Debug("送信者設定", "from", from)
	return nil
}

// Rcpt 受信者を追加
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	s.logger.Debug("受信者追加", "to", to)
	return nil
}

// Data メールデータを受信して送信
func (s *Session) Data(r io.Reader) error {
	s.logger.Debug("メールデータ受信開始")

	// メッセージをパース
	msg, err := mail.ReadMessage(r)
	if err != nil {
		s.logger.Error("メッセージパースエラー", "error", err)
		return fmt.Errorf("メッセージパースエラー: %w", err)
	}

	// ヘッダーを解析
	subject := decodeHeader(msg.Header.Get("Subject"))
	s.logger.Debug("メッセージ解析", "subject", subject, "from", s.from, "to_count", len(s.to))

	// CC受信者を取得
	var ccAddresses []string
	if ccHeader := msg.Header.Get("Cc"); ccHeader != "" {
		ccList, err := mail.ParseAddressList(ccHeader)
		if err == nil {
			for _, addr := range ccList {
				ccAddresses = append(ccAddresses, addr.Address)
			}
		}
	}

	// メール本文を抽出
	body, isHTML, err := extractBody(msg)
	if err != nil {
		s.logger.Warn("本文抽出エラー、デフォルトテキストで送信", "error", err)
		body = "（本文を抽出できませんでした）"
		isHTML = false
	}

	s.logger.Debug("本文抽出完了", "length", len(body), "isHTML", isHTML)

	// Microsoft Graphで送信
	ctx := context.Background()
	if len(ccAddresses) > 0 {
		err = s.backend.graphClient.SendMailWithMultipleRecipients(ctx, s.to, ccAddresses, subject, body, isHTML)
	} else {
		if len(s.to) == 0 {
			return fmt.Errorf("受信者が指定されていません")
		}
		// 単一受信者の場合（後方互換性）
		err = s.backend.graphClient.SendMail(ctx, s.to[0], subject, body, isHTML)
	}

	if err != nil {
		s.logger.Error("メール送信失敗", "error", err)
		return fmt.Errorf("メール送信失敗: %w", err)
	}

	s.logger.Info("メール送信成功", "subject", subject, "to_count", len(s.to), "cc_count", len(ccAddresses))
	return nil
}

// decodeHeader MIMEエンコードされたヘッダーをデコード
func decodeHeader(header string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(header)
	if err != nil {
		return header
	}
	return decoded
}

// extractBody メール本文を抽出
func extractBody(msg *mail.Message) (string, bool, error) {
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		// Content-Typeがない場合、本文全体を読み取る
		bodyBytes, err := io.ReadAll(msg.Body)
		if err != nil {
			return "", false, err
		}
		return string(bodyBytes), false, nil
	}

	// マルチパートの場合
	if strings.HasPrefix(mediaType, "multipart/") {
		return extractMultipartBody(msg.Body, params["boundary"])
	}

	// シングルパートの場合
	bodyBytes, err := io.ReadAll(msg.Body)
	if err != nil {
		return "", false, err
	}

	// Content-Transfer-Encodingを処理
	encoding := msg.Header.Get("Content-Transfer-Encoding")
	bodyText := string(bodyBytes)

	if strings.EqualFold(encoding, "base64") {
		decoded, err := base64.StdEncoding.DecodeString(bodyText)
		if err == nil {
			bodyText = string(decoded)
		}
	} else if strings.EqualFold(encoding, "quoted-printable") {
		bodyText = decodeQuotedPrintable(bodyText)
	}

	isHTML := strings.HasPrefix(mediaType, "text/html")
	return bodyText, isHTML, nil
}

// extractMultipartBody マルチパート本文を抽出
func extractMultipartBody(body io.Reader, boundary string) (string, bool, error) {
	mr := multipart.NewReader(body, boundary)

	var textPart, htmlPart string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false, err
		}

		contentType := part.Header.Get("Content-Type")
		mediaType, _, _ := mime.ParseMediaType(contentType)

		partBytes, err := io.ReadAll(part)
		if err != nil {
			continue
		}

		// Content-Transfer-Encodingを処理
		encoding := part.Header.Get("Content-Transfer-Encoding")
		partText := string(partBytes)

		if strings.EqualFold(encoding, "base64") {
			decoded, err := base64.StdEncoding.DecodeString(partText)
			if err == nil {
				partText = string(decoded)
			}
		} else if strings.EqualFold(encoding, "quoted-printable") {
			partText = decodeQuotedPrintable(partText)
		}

		// パートタイプに応じて保存
		if strings.HasPrefix(mediaType, "text/plain") {
			textPart = partText
		} else if strings.HasPrefix(mediaType, "text/html") {
			htmlPart = partText
		} else if strings.HasPrefix(mediaType, "multipart/") {
			// ネストされたマルチパート（再帰的に処理可能だが、ここでは簡略化）
			continue
		}
	}

	// HTMLが優先、なければテキスト
	if htmlPart != "" {
		return htmlPart, true, nil
	}
	if textPart != "" {
		return textPart, false, nil
	}

	return "", false, fmt.Errorf("本文が見つかりません")
}

// decodeQuotedPrintable Quoted-Printableデコード（簡易版）
func decodeQuotedPrintable(s string) string {
	var buf bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(s))

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			break
		}

		// 行末の=を削除（ソフト改行）
		line = strings.TrimRight(line, "\r\n")
		if strings.HasSuffix(line, "=") {
			line = strings.TrimSuffix(line, "=")
		} else {
			line += "\n"
		}

		// =XX形式をデコード
		i := 0
		for i < len(line) {
			if line[i] == '=' && i+2 < len(line) {
				var b byte
				fmt.Sscanf(line[i+1:i+3], "%02X", &b)
				buf.WriteByte(b)
				i += 3
			} else {
				buf.WriteByte(line[i])
				i++
			}
		}

		if err == io.EOF {
			break
		}
	}

	return buf.String()
}
