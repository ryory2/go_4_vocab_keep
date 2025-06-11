package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// --- 構造体定義 (変更なし) ---

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type DatabaseConfig struct {
	URL string `mapstructure:"url"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	ReviewLimit int    `mapstructure:"review_limit"`
	FrontendURL string `mapstructure:"frontend_url"`
}

type AuthConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
	Debug            bool     `mapstructure:"debug"`
}

type JWTConfig struct {
	SecretKey      string        `mapstructure:"secret_key"`
	AccessTokenTTL time.Duration `mapstructure:"access_token_ttl"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

type SESConfig struct {
	Region          string `mapstructure:"region"`
	From            string `mapstructure:"from"`
	AuthType        string `mapstructure:"auth_type"` // "iam_role" or "env_var"
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
}

type MailerConfig struct {
	Type string `mapstructure:"type"` // "log", "smtp", or "ses"
}

type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Server   ServerConfig   `mapstructure:"server"`
	App      AppConfig      `mapstructure:"app"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Log      LogConfig      `mapstructure:"log"`
	CORS     CORSConfig     `mapstructure:"cors"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	SMTP     SMTPConfig     `mapstructure:"smtp"`
	SES      SESConfig      `mapstructure:"ses"`
	Mailer   MailerConfig   `mapstructure:"mailer"`
}

// Cfg はアプリケーション全体の設定を保持するグローバル変数
var Cfg Config

// LoadConfig は設定を階層的に読み込みます
func LoadConfig(configPath string) error {
	v := viper.New()

	log.Println("--- Starting Configuration Loading ---")
	currentDir, _ := os.Getwd()
	log.Printf("[DEBUG] Current working directory: %s", currentDir)
	log.Printf("[DEBUG] Config search path provided: %s", configPath)

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		log.Printf("[WARN] Could not resolve absolute path for config: %v", err)
	} else {
		log.Printf("[DEBUG] Resolved absolute config search path: %s", absConfigPath)
	}

	// 1. デフォルト設定ファイルを読み込む
	v.SetConfigName("config.default")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	if err := v.ReadInConfig(); err != nil {
		// デフォルト設定ファイルは必須とする
		return fmt.Errorf("fatal error: default config file not found: %w", err)
	}

	// 2. 環境変数 APP_ENV に応じて環境別設定ファイルをマージする
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development" // デフォルトは開発環境
	}
	log.Printf("Loading configuration for environment: %s", env)
	v.SetConfigName(fmt.Sprintf("config.%s", env))

	// MergeInConfig は、ファイルが存在すれば設定を上書きし、なければ何もしない
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error merging env config file 'config.%s.yaml': %w", env, err)
		}
		log.Printf("Info: No environment specific config file found. Using defaults and environment variables.")
	} else {
		log.Printf("Successfully merged config file: %s", v.ConfigFileUsed())
	}

	// 3. 環境変数でさらに上書きする (機密情報用)
	v.SetEnvPrefix("APP")
	v.AutomaticEnv()
	// ネストしたキー (e.g., database.url) を環境変数 (APP_DATABASE_URL) にマッピング
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 4. 全ての設定を Cfg 構造体にアンマーシャル
	if err := v.Unmarshal(&Cfg); err != nil {
		return fmt.Errorf("unable to decode config into struct: %w", err)
	}

	log.Println("Configuration loaded successfully.")
	return nil
}
