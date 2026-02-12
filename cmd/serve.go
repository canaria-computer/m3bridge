package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/canaria-computer/m3bridge/internal/auth"
	"github.com/canaria-computer/m3bridge/internal/config"
	"github.com/canaria-computer/m3bridge/internal/graph"
	"github.com/canaria-computer/m3bridge/internal/smtp"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "SMTPサーバを起動",
	Long:  `ローカルホストでSMTPサーバを起動し、受信したメールをMicrosoft Graph経由で送信します。`,
	RunE:  runServe,
}

var (
	port int
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVarP(&port, "port", "p", 2525, "SMTPサーバのポート番号")
}

func runServe(cmd *cobra.Command, args []string) error {
	logger := GetLogger()
	logger.Info("SMTPサーバを起動します")

	// 設定を読み込む
	cfg, err := config.NewManager(logger)
	if err != nil {
		return fmt.Errorf("設定読み込みエラー: %w", err)
	}

	smtpConfig := cfg.GetSMTPConfig()
	graphConfig := cfg.GetGraphConfig()

	// ポートが指定された場合は更新
	if port != 2525 {
		smtpConfig.Port = port
		if err := cfg.UpdateSMTPPort(port); err != nil {
			logger.Warn("ポート設定更新失敗", "error", err)
		}
	}

	// SMTP接続情報を表示
	logger.Info("SMTP設定情報",
		"host", smtpConfig.Host,
		"port", smtpConfig.Port,
		"username", smtpConfig.Username,
		"password", smtpConfig.Password)

	fmt.Println("\n=== SMTP接続情報 ===")
	fmt.Printf("サーバ: %s:%d\n", smtpConfig.Host, smtpConfig.Port)
	fmt.Printf("ユーザー名: %s\n", smtpConfig.Username)
	fmt.Printf("パスワード: %s\n", smtpConfig.Password)
	fmt.Printf("セキュリティ: なし（平文）\n")
	fmt.Printf("設定ファイル: %s\n", cfg.GetConfigPath())
	fmt.Println("=====================\n")

	// 認証マネージャーを作成
	authenticator := auth.NewAuthenticator(
		graphConfig.ClientID,
		graphConfig.RedirectURI,
		graphConfig.AuthorityURL,
		graphConfig.TokenCache,
		logger,
	)

	// アクセストークンを取得
	logger.Info("Microsoft Graphで認証します")
	accessToken, err := authenticator.GetAccessToken()
	if err != nil {
		return fmt.Errorf("トークン取得エラー: %w", err)
	}

	logger.Info("認証成功")

	// Graphクライアントを作成
	graphClient, err := graph.NewClient(accessToken, logger)
	if err != nil {
		return fmt.Errorf("Graphクライアント作成エラー: %w", err)
	}

	// ユーザー情報を取得して確認
	if err := graphClient.GetUserInfo(context.Background()); err != nil {
		return fmt.Errorf("ユーザー情報取得エラー: %w", err)
	}

	// SMTPサーバを作成
	server := smtp.NewServer(smtp.Config{
		Host:     smtpConfig.Host,
		Port:     smtpConfig.Port,
		Username: smtpConfig.Username,
		Password: smtpConfig.Password,
	}, graphClient, logger)

	// シグナルハンドリング
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)

	// サーバをゴルーチンで起動
	go func() {
		errChan <- server.Start()
	}()

	// シグナルまたはエラーを待機
	select {
	case sig := <-sigChan:
		logger.Info("シグナル受信、サーバを停止します", "signal", sig)
		if err := server.Stop(); err != nil {
			logger.Error("サーバ停止エラー", "error", err)
		}
		return nil
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("サーバエラー: %w", err)
		}
		return nil
	}
}
