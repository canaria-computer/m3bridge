package smtp

import (
	"fmt"
	"time"

	"github.com/canaria-computer/m3bridge/internal/graph"
	"github.com/charmbracelet/log"
	"github.com/emersion/go-smtp"
)

// Server SMTPサーバ
type Server struct {
	smtpServer  *smtp.Server
	graphClient *graph.Client
	logger      *log.Logger
}

// Config サーバ設定
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
}

// NewServer 新しいSMTPサーバを作成
func NewServer(config Config, graphClient *graph.Client, logger *log.Logger) *Server {
	backend := NewBackend(graphClient, config.Username, config.Password, logger)

	s := smtp.NewServer(backend)
	s.Addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
	s.Domain = "localhost"
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = 10 * 1024 * 1024 // 10MB
	s.MaxRecipients = 50
	s.AllowInsecureAuth = true

	logger.Info("SMTPサーバ作成完了",
		"addr", s.Addr,
		"auth_enabled", config.Username != "" && config.Password != "")

	return &Server{
		smtpServer:  s,
		graphClient: graphClient,
		logger:      logger,
	}
}

// Start サーバを起動
func (s *Server) Start() error {
	s.logger.Info("SMTPサーバ起動", "addr", s.smtpServer.Addr)
	return s.smtpServer.ListenAndServe()
}

// Stop サーバを停止
func (s *Server) Stop() error {
	s.logger.Info("SMTPサーバ停止")
	return s.smtpServer.Close()
}
