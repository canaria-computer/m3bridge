package graph

import (
	"context"
	"fmt"

	"github.com/canaria-computer/m3bridge/internal/auth"
	"github.com/charmbracelet/log"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// Client Microsoft Graph APIクライアント
type Client struct {
	graphClient *msgraphsdk.GraphServiceClient
	logger      *log.Logger
}

// NewClient 新しいGraphクライアントを作成
func NewClient(accessToken string, logger *log.Logger) (*Client, error) {
	authProvider := auth.NewBearerTokenAuthenticationProvider(accessToken, logger)

	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		return nil, fmt.Errorf("アダプター作成失敗: %w", err)
	}

	graphClient := msgraphsdk.NewGraphServiceClient(adapter)

	return &Client{
		graphClient: graphClient,
		logger:      logger,
	}, nil
}

// GetUserInfo ユーザー情報を取得
func (c *Client) GetUserInfo(ctx context.Context) error {
	c.logger.Debug("ユーザー情報取得開始")

	user, err := c.graphClient.Me().Get(ctx, nil)
	if err != nil {
		c.logger.Error("ユーザー情報取得失敗", "error", err)
		return err
	}

	displayName := ""
	if user.GetDisplayName() != nil {
		displayName = *user.GetDisplayName()
	}
	upn := ""
	if user.GetUserPrincipalName() != nil {
		upn = *user.GetUserPrincipalName()
	}
	mail := ""
	if user.GetMail() != nil {
		mail = *user.GetMail()
	}

	c.logger.Info("ユーザー情報取得成功",
		"displayName", displayName,
		"userPrincipalName", upn,
		"mail", mail)

	return nil
}

// SendMail メールを送信
func (c *Client) SendMail(ctx context.Context, to, subject, body string, isHTML bool) error {
	c.logger.Debug("メール送信開始", "to", to, "subject", subject)

	// メッセージの作成
	message := models.NewMessage()
	message.SetSubject(&subject)

	// ボディの設定
	messageBody := models.NewItemBody()
	var contentType models.BodyType
	if isHTML {
		contentType = models.HTML_BODYTYPE
	} else {
		contentType = models.TEXT_BODYTYPE
	}
	messageBody.SetContentType(&contentType)
	messageBody.SetContent(&body)
	message.SetBody(messageBody)

	// 受信者の設定
	recipient := models.NewRecipient()
	emailAddress := models.NewEmailAddress()
	emailAddress.SetAddress(&to)
	recipient.SetEmailAddress(emailAddress)
	message.SetToRecipients([]models.Recipientable{recipient})

	// メール送信リクエストボディの作成
	sendMailBody := users.NewItemSendMailPostRequestBody()
	sendMailBody.SetMessage(message)
	saveToSentItems := true
	sendMailBody.SetSaveToSentItems(&saveToSentItems)

	c.logger.Debug("メール送信リクエスト送信中")
	err := c.graphClient.Me().SendMail().Post(ctx, sendMailBody, nil)
	if err != nil {
		c.logger.Error("メール送信失敗", "error", err)
		return err
	}

	c.logger.Info("メール送信成功", "to", to)
	return nil
}

// SendMailWithMultipleRecipients 複数の受信者にメールを送信
func (c *Client) SendMailWithMultipleRecipients(ctx context.Context, to []string, cc []string, subject, body string, isHTML bool) error {
	c.logger.Debug("メール送信開始", "to_count", len(to), "cc_count", len(cc), "subject", subject)

	// メッセージの作成
	message := models.NewMessage()
	message.SetSubject(&subject)

	// ボディの設定
	messageBody := models.NewItemBody()
	var contentType models.BodyType
	if isHTML {
		contentType = models.HTML_BODYTYPE
	} else {
		contentType = models.TEXT_BODYTYPE
	}
	messageBody.SetContentType(&contentType)
	messageBody.SetContent(&body)
	message.SetBody(messageBody)

	// To受信者の設定
	if len(to) > 0 {
		toRecipients := make([]models.Recipientable, 0, len(to))
		for _, addr := range to {
			recipient := models.NewRecipient()
			emailAddress := models.NewEmailAddress()
			emailAddress.SetAddress(&addr)
			recipient.SetEmailAddress(emailAddress)
			toRecipients = append(toRecipients, recipient)
		}
		message.SetToRecipients(toRecipients)
	}

	// CC受信者の設定
	if len(cc) > 0 {
		ccRecipients := make([]models.Recipientable, 0, len(cc))
		for _, addr := range cc {
			recipient := models.NewRecipient()
			emailAddress := models.NewEmailAddress()
			emailAddress.SetAddress(&addr)
			recipient.SetEmailAddress(emailAddress)
			ccRecipients = append(ccRecipients, recipient)
		}
		message.SetCcRecipients(ccRecipients)
	}

	// メール送信リクエストボディの作成
	sendMailBody := users.NewItemSendMailPostRequestBody()
	sendMailBody.SetMessage(message)
	saveToSentItems := true
	sendMailBody.SetSaveToSentItems(&saveToSentItems)

	c.logger.Debug("メール送信リクエスト送信中")
	err := c.graphClient.Me().SendMail().Post(ctx, sendMailBody, nil)
	if err != nil {
		c.logger.Error("メール送信失敗", "error", err)
		return err
	}

	c.logger.Info("メール送信成功", "to_count", len(to), "cc_count", len(cc))
	return nil
}
