package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	logger  *log.Logger
)

var rootCmd = &cobra.Command{
	Use:   "msgraph-smtp",
	Short: "Microsoft Graph経由でメールを送信するSMTPサーバ",
	Long: `Microsoft Graph APIを使用してメールを送信するローカルSMTPサーバ。
OAuth 2.0認証を使用し、SMTPプロトコル経由で受信したメールをMicrosoft Graphで送信します。`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "設定ファイルパス (デフォルト: $HOME/.msgraph-smtp.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "ログレベル (debug, info, warn, error)")

	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))

	// ロガーの初期化
	logger = log.New(os.Stderr)
	logger.SetLevel(log.InfoLevel)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".msgraph-smtp")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		logger.Debug("設定ファイル使用", "file", viper.ConfigFileUsed())
	}

	// ログレベルの設定
	logLevel := viper.GetString("log-level")
	switch logLevel {
	case "debug":
		logger.SetLevel(log.DebugLevel)
	case "info":
		logger.SetLevel(log.InfoLevel)
	case "warn":
		logger.SetLevel(log.WarnLevel)
	case "error":
		logger.SetLevel(log.ErrorLevel)
	}
}

func GetLogger() *log.Logger {
	return logger
}
