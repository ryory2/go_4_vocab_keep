package service

import (
	"context"
	"log/slog"

	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/middleware"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// SESMailer は AWS SES を使ってメールを送信する実装です
type SESMailer struct {
	client *sesv2.Client
	cfg    *config.SESConfig
}

// NewSESMailer は設定に応じて認証方法を切り替えてSESクライアントを生成します
func NewSESMailer(cfg *config.Config) Mailer {
	// AWS SDKに渡す設定オプションのスライスを準備
	var awsCfgOpts []func(*awsconfig.LoadOptions) error

	// 必須のリージョン設定を追加
	awsCfgOpts = append(awsCfgOpts, awsconfig.WithRegion(cfg.SES.Region))

	// 設定ファイルに基づき、認証方法を決定
	switch cfg.SES.AuthType {
	case "static_credentials":
		// --- 静的認証情報 (アクセスキー) を使う場合 ---
		slog.Info("Configuring SES with static credentials.")
		if cfg.SES.AccessKeyID == "" || cfg.SES.SecretAccessKey == "" {
			slog.Error("SES auth_type is 'static_credentials' but access_key_id or secret_access_key is missing in config.")
			// 起動時にpanicさせることで、設定ミスに即座に気づけるようにする
			panic("missing static credentials for SES")
		}
		// 静的認証情報を提供するプロバイダーを作成
		creds := credentials.NewStaticCredentialsProvider(
			cfg.SES.AccessKeyID,
			cfg.SES.SecretAccessKey,
			"", // Session Token (通常は不要)
		)
		// 設定オプションに認証情報プロバイダーを追加
		awsCfgOpts = append(awsCfgOpts, awsconfig.WithCredentialsProvider(creds))

	case "iam_role":
		// --- IAMロール (ECS Task Role, EC2 Instance Profileなど) を使う場合 ---
		slog.Info("Configuring SES with IAM Role credentials.")
		// この場合、SDKが自動で認証情報を探してくれるので、特別な設定は不要

	default:
		// --- 不明な設定または未設定の場合 ---
		slog.Warn("Unknown SES auth_type specified, defaulting to IAM Role.", "type", cfg.SES.AuthType)
		// デフォルトでIAMロール方式を試みる
	}

	// 組み立てたオプションでAWS設定をロード
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsCfgOpts...)
	if err != nil {
		slog.Error("Failed to load AWS config for SES", "error", err)
		panic(err)
	}

	return &SESMailer{
		client: sesv2.NewFromConfig(awsCfg),
		cfg:    &cfg.SES,
	}
}

// Send は AWS SES を使用してメールを送信します
func (m *SESMailer) Send(ctx context.Context, to, subject, body string) error {
	logger := middleware.GetLogger(ctx)

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(m.cfg.From),
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data:    aws.String(body),
						Charset: aws.String("UTF-8"),
					},
					// HTMLメールを送信する場合は、以下を有効化する
					// Html: &types.Content{
					// 	Data:    aws.String("<h1>" + subject + "</h1><p>" + body + "</p>"),
					// 	Charset: aws.String("UTF-8"),
					// },
				},
			},
		},
	}

	// SendEmail APIを呼び出し
	_, err := m.client.SendEmail(context.Background(), input) // API呼び出し自体のコンテキストはBackgroundで良い
	if err != nil {
		logger.Error("Failed to send email via SES", "error", err, "to", to)
		return err
	}

	logger.Info("Email sent successfully via SES", "to", to, "subject", subject)
	return nil
}
