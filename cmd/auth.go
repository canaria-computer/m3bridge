package cmd

import (
	"context"
	"fmt"

	"github.com/canaria-computer/m3bridge/internal/auth"
	"github.com/canaria-computer/m3bridge/internal/config"
	"github.com/canaria-computer/m3bridge/internal/graph"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Microsoft Graphで認証",
	Long:  `Microsoft Graph APIで認証し、アクセストークンを取得してキャッシュします。`,
	RunE:  runAuth,
}

var (
	testAuth bool
)

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.Flags().BoolVar(&testAuth, "test", false, "認証後にユーザー情報を取得してテスト")
}

func runAuth(cmd *cobra.Command, args []string) error {
	logger := GetLogger()
	logger.Info("認証を開始します")

	// 設定を読み込む
	cfg, err := config.NewManager(logger)
	if err != nil {
		return fmt.Errorf("設定読み込みエラー: %w", err)
	}

	graphConfig := cfg.GetGraphConfig()

	// 認証マネージャーを作成
	authenticator := auth.NewAuthenticator(
		graphConfig.ClientID,
		graphConfig.RedirectURI,
		graphConfig.AuthorityURL,
		graphConfig.TokenCache,
		logger,
	)

	// アクセストークンを取得
	accessToken, err := authenticator.GetAccessToken()
	if err != nil {
		return fmt.Errorf("トークン取得エラー: %w", err)
	}

	logger.Info("認証成功")

	// テストが有効な場合、ユーザー情報を取得
	if testAuth {
		logger.Info("ユーザー情報を取得します")
		graphClient, err := graph.NewClient(accessToken, logger)
		if err != nil {
			return fmt.Errorf("Graphクライアント作成エラー: %w", err)
		}

		if err := graphClient.GetUserInfo(context.Background()); err != nil {
			return fmt.Errorf("ユーザー情報取得エラー: %w", err)
		}
	}

	return nil
}
