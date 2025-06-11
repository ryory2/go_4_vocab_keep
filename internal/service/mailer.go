package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"

	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/middleware"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// --- LogMailer ---
type LogMailer struct{}

func (m *LogMailer) Send(ctx context.Context, to, subject, body string) error {
	logger := middleware.GetLogger(ctx)
	logger.Info("--- Sending Email (LogMailer) ---", "to", to, "subject", subject, "body", body)
	return nil
}

// --- SmtpMailer ---
type SmtpMailer struct {
	cfg *config.SMTPConfig
}

func (m *SmtpMailer) Send(ctx context.Context, to, subject, body string) error {
	logger := middleware.GetLogger(ctx)
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)

	logger.Debug("Attempting to send email via SMTP",
		"smtp_addr", addr,
		"from", m.cfg.From,
		"to", to,
	)

	// ★★★ ここからが修正部分 ★★★

	// 1. SMTPサーバーに接続
	// smtp.Dialは平文での接続を許可する低レベルな関数
	c, err := smtp.Dial(addr)
	if err != nil {
		logger.Error("Failed to connect to SMTP server", "error", err, "addr", addr)
		return err
	}
	// 関数終了時に接続を閉じる
	defer c.Close()

	// 2. 送信元(MAIL FROM)を設定
	if err = c.Mail(m.cfg.From); err != nil {
		logger.Error("Failed to set MAIL FROM", "error", err, "from", m.cfg.From)
		return err
	}

	// 3. 宛先(RCPT TO)を設定
	if err = c.Rcpt(to); err != nil {
		logger.Error("Failed to set RCPT TO", "error", err, "to", to)
		return err
	}

	// 4. データ(DATA)の書き込みを開始
	wc, err := c.Data()
	if err != nil {
		logger.Error("Failed to open data writer", "error", err)
		return err
	}
	defer wc.Close()

	// 5. ヘッダーと本文を書き込む
	msg := "To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n"

	if _, err = wc.Write([]byte(msg)); err != nil {
		logger.Error("Failed to write email data", "error", err)
		return err
	}

	// ★★★ 修正部分ここまで ★★★

	logger.Info("Email sent successfully via SMTP", "to", to, "subject", subject)
	return nil
}

// --- NewMailer ファクトリ関数 ---
func NewMailer(cfg *config.Config) Mailer {
	logger := slog.Default()
	switch cfg.Mailer.Type {
	case "smtp":
		logger.Info("Initializing SMTP mailer...")
		return &SmtpMailer{cfg: &cfg.SMTP}
	case "log":
		logger.Info("Initializing Log mailer...")
		return &LogMailer{}
	default:
		logger.Warn("Unknown mailer type, defaulting to LogMailer", "type", cfg.Mailer.Type)
		return &LogMailer{}
	}
}
