// internal/config/config.go
package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	// 文字列操作用にインポート
	// os をインポート (環境変数チェック用に追加)
	"github.com/spf13/viper"
)

// 各設定の構造体を定義（この構造体を通してGoの設定値を取得する）
// ログ設定
type LogConfig struct {
	Level  string `mapstructure:"level"`  // ログレベル (e.g., "debug", "info", "warn", "error")
	Format string `mapstructure:"format"` // ログフォーマット (e.g., "json", "text")
}

// DB設定
type DatabaseConfig struct {
	URL string `mapstructure:"url"`
}

// サーバー設定
type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// アプリケーション設定
type AppConfig struct {
	ReviewLimit int `mapstructure:"review_limit"`
}

// 認証設定
type AuthConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// CORS設定
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"` // 例: ["http://localhost:3000", "https://yourdomain.com"]
	AllowedMethods   []string `mapstructure:"allowed_methods"` // 例: ["GET", "POST", "PUT"]
	AllowedHeaders   []string `mapstructure:"allowed_headers"` // 例: ["Content-Type", "Authorization"]
	ExposedHeaders   []string `mapstructure:"exposed_headers"` // 例: ["Content-Length"]
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"` // プリフライトリクエストのキャッシュ時間 (秒)
	Debug            bool     `mapstructure:"debug"`   // CORSデバッグモード
}

type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Server   ServerConfig   `mapstructure:"server"`
	App      AppConfig      `mapstructure:"app"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Log      LogConfig      `mapstructure:"log"` // Log設定用のフィールドを追加
	CORS     CORSConfig     `mapstructure:"cors"`
}

var Cfg Config

// 環境変数名を定数として定義 (推奨)
const (
	EnvPrefix            = "APP"
	EnvKeyDatabaseURL    = "APP_DATABASE_URL"
	EnvKeyServerPort     = "APP_SERVER_PORT"
	EnvKeyLogLevel       = "APP_LOG_LEVEL"
	EnvKeyLogFormat      = "APP_LOG_FORMAT"
	EnvKeyAppReviewLimit = "APP_APP_REVIEW_LIMIT"
	EnvKeyAuthEnabled    = "APP_AUTH_ENABLED"
	// CORS関連の環境変数キー (例)
	EnvKeyCORSAllowedOrigins   = "APP_CORS_ALLOWED_ORIGINS" // カンマ区切り文字列として扱うことが多い
	EnvKeyCORSAllowedMethods   = "APP_CORS_ALLOWED_METHODS" // カンマ区切り
	EnvKeyCORSAllowedHeaders   = "APP_CORS_ALLOWED_HEADERS" // カンマ区切り
	EnvKeyCORSExposedHeaders   = "APP_CORS_EXPOSED_HEADERS" // カンマ区切り
	EnvKeyCORSAllowCredentials = "APP_CORS_ALLOW_CREDENTIALS"
	EnvKeyCORSMaxAge           = "APP_CORS_MAX_AGE"
	EnvKeyCORSDebug            = "APP_CORS_DEBUG"
)

func LoadConfig(relativePathToSearch string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// --- 現在のワーキングディレクトリをログに出力 ---
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: Could not get current working directory: %v", err)
	} else {
		log.Printf("Current working directory: %s", currentDir)
	}
	viper.AddConfigPath(".")
	log.Println("  - Added search path: . (current working directory)")

	// --- 設定ファイルの検索パスをログに出力 ---
	log.Println("Attempting to add config search paths:")
	if relativePathToSearch != "" {
		viper.AddConfigPath(relativePathToSearch)
		log.Printf("  - Added search path (from argument): %s (relative to current working dir if not absolute)", relativePathToSearch)
		// もし絶対パスで解決してログ出力したい場合
		// absPath, _ := filepath.Abs(filepath.Join(currentDir, relativePathToSearch))
		// log.Printf("    Resolved to absolute path: %s", absPath)
	}
	viper.AddConfigPath(".")
	log.Println("  - Added search path: . (current working directory)")
	// (オプション) Viperがデフォルトで探す他のパスもログに出したい場合
	// log.Printf("  - Viper default search paths might also be considered (e.g., $HOME, /etc)")
	// viper.SetConfigName("config")
	// viper.SetConfigType("yaml")
	// viper.AddConfigPath(path)
	// viper.AddConfigPath(".")

	// 環境変数名を指定して読み込むことも可能 (例: AUTH_ENABLED)
	viper.SetEnvPrefix(EnvPrefix) // 例: APP_AUTH_ENABLED のように接頭辞をつける場合
	viper.AutomaticEnv()          // 環境変数を自動で読み込む

	// 環境変数を Config 構造体のフィールドに紐付け、config.yamlから環境変数を設定
	viper.BindEnv("database.url", EnvKeyDatabaseURL)
	viper.BindEnv("server.port", EnvKeyServerPort)
	viper.BindEnv("log.level", EnvKeyLogLevel)
	viper.BindEnv("log.format", EnvKeyLogFormat)
	viper.BindEnv("app.review_limit", EnvKeyAppReviewLimit)
	viper.BindEnv("auth.enabled", EnvKeyAuthEnabled)

	// CORS関連の環境変数紐付け
	viper.BindEnv("cors.allowed_origins", EnvKeyCORSAllowedOrigins)
	viper.BindEnv("cors.allowed_methods", EnvKeyCORSAllowedMethods)
	viper.BindEnv("cors.allowed_headers", EnvKeyCORSAllowedHeaders)
	viper.BindEnv("cors.exposed_headers", EnvKeyCORSExposedHeaders)
	viper.BindEnv("cors.allow_credentials", EnvKeyCORSAllowCredentials)
	viper.BindEnv("cors.max_age", EnvKeyCORSMaxAge)
	viper.BindEnv("cors.debug", EnvKeyCORSDebug)

	// 設定ファイルの読み込み試行
	log.Println("Attempting to read config file (e.g., config.yaml)...")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Warning: Config file not found in specified paths. Using environment variables and/or defaults.")
			// 検索したパスの詳細をここでもう一度表示しても良い
			log.Println("  Viper searched in (among others):")
			if relativePathToSearch != "" {
				log.Printf("    - %s (from argument)", relativePathToSearch)
			}
			log.Println("    - . (current working directory)")
		} else {
			// 設定ファイルが見つからない以外のエラー
			log.Printf("Error reading config file: %s\n", err)
			return fmt.Errorf("error reading config file: %w", err) // fmt.Errorf でエラーをラップ
		}
	} else {
		// --- 読み込みに成功した場合、どのファイルが読み込まれたかをログに出力 ---
		log.Printf("Successfully read config file from: %s", viper.ConfigFileUsed())
	}

	if err := viper.Unmarshal(&Cfg); err != nil {
		log.Printf("Error unmarshalling config: %s\n", err)
		return fmt.Errorf("error unmarshalling config: %w", err) // fmt.Errorf でエラーをラップ
	}

	// --- デバッグ用のログ出力 (現状維持) ---
	log.Println("--- Viper & Config Debug ---")
	log.Printf("Viper - database.url: %s", viper.GetString("database.url"))
	// ... (他のViper GetStringログ) ...
	log.Printf("Cfg.Database.URL: %s", Cfg.Database.URL)
	// ... (他のCfgフィールドログ) ...
	log.Println("--------------------------")

	// --- デフォルト値の設定 ---
	// viper.IsSet で設定ファイルや環境変数で明示的に値が設定されたかを確認できる
	if !viper.IsSet("server.port") && Cfg.Server.Port == "" { // 環境変数 PORT も考慮
		log.Println("Server port not set, using default ':8080'")
		Cfg.Server.Port = ":8080"
	}
	// ReviewLimit のデフォルト値
	if Cfg.App.ReviewLimit <= 0 { // viper.IsSetを使っても良い
		log.Println("App review limit not set or invalid, using default '20'")
		Cfg.App.ReviewLimit = 20
	}
	// Database URL は必須かもしれないので、デフォルト値を設定するか、エラーにするか検討
	if Cfg.Database.URL == "" {
		log.Println("Warning: Database URL is not set in config.")
	}
	// Auth.Enabled のデフォルト値
	if !viper.IsSet("auth.enabled") {
		log.Println("Auth enabled flag not set, defaulting to true (enabled)")
		Cfg.Auth.Enabled = true
	}
	// Log Level のデフォルト値
	if !viper.IsSet("log.level") {
		log.Println("Log level not set, defaulting to 'info'")
		Cfg.Log.Level = "info"
	} else {
		// 読み込んだ値を小文字に正規化（比較しやすくするため）
		Cfg.Log.Level = strings.ToLower(Cfg.Log.Level)
	}
	// Log Format のデフォルト値
	if !viper.IsSet("log.format") {
		log.Println("Log format not set, defaulting to 'json'")
		Cfg.Log.Format = "json"
	} else {
		// 読み込んだ値を小文字に正規化
		Cfg.Log.Format = strings.ToLower(Cfg.Log.Format)
	}
	// CORSのデフォルト値設定
	// AllowedOrigins: 環境変数でカンマ区切り文字列で渡された場合、ここでパースするか、
	// Viperの機能で直接スライスに変換できるか確認 (Viperは文字列からのスライス変換をサポート)
	// もしCfg.CORS.AllowedOriginsが空ならデフォルトを設定
	if len(Cfg.CORS.AllowedOrigins) == 0 {
		log.Println("CORS AllowedOrigins not set, using default ['http://localhost:3000']")
		Cfg.CORS.AllowedOrigins = []string{"http://localhost:3000"}
	}
	if len(Cfg.CORS.AllowedMethods) == 0 {
		log.Println("CORS AllowedMethods not set, using default ['GET', 'POST', 'OPTIONS']")
		Cfg.CORS.AllowedMethods = []string{"GET", "POST", "OPTIONS"}
	}
	if len(Cfg.CORS.AllowedHeaders) == 0 {
		log.Println("CORS AllowedHeaders not set, using default ['Content-Type', 'Authorization']")
		Cfg.CORS.AllowedHeaders = []string{"Content-Type", "Authorization"}
	}
	// AllowCredentials はデフォルト false のことが多いが、true が必要なら true に。
	if !viper.IsSet("cors.allow_credentials") { // viper.IsSet で明示的な設定があったか確認
		log.Println("CORS AllowCredentials not set, defaulting to true")
		Cfg.CORS.AllowCredentials = true
	}
	if Cfg.CORS.MaxAge <= 0 { // 0以下は無効とみなすか、デフォルト値を設定
		log.Println("CORS MaxAge not set or invalid, using default 300 seconds")
		Cfg.CORS.MaxAge = 300 // 5 minutes
	}
	// Debug はデフォルト false
	if !viper.IsSet("cors.debug") {
		log.Println("CORS Debug not set, defaulting to false")
		Cfg.CORS.Debug = false
	}

	// 読み込み完了ログ (LoadConfig内では標準logのまま)
	log.Println("Config loaded successfully")
	log.Printf("    Server Port: %s", Cfg.Server.Port)
	log.Printf("    Review Limit: %d", Cfg.App.ReviewLimit)
	log.Printf("    Database URL Set: %t", Cfg.Database.URL != "") // URL自体は出力しない
	log.Printf("    Auth Enabled: %t", Cfg.Auth.Enabled)
	log.Printf("    Log Level: %s", Cfg.Log.Level)
	log.Printf("    Log Format: %s", Cfg.Log.Format)
	log.Printf("  CORS Config:")
	log.Printf("    AllowedOrigins:   [%s]", strings.Join(Cfg.CORS.AllowedOrigins, ", "))
	log.Printf("    AllowedMethods:   [%s]", strings.Join(Cfg.CORS.AllowedMethods, ", "))
	log.Printf("    AllowedHeaders:   [%s]", strings.Join(Cfg.CORS.AllowedHeaders, ", "))
	log.Printf("    ExposedHeaders:   [%s]", strings.Join(Cfg.CORS.ExposedHeaders, ", "))
	log.Printf("    AllowCredentials: %t", Cfg.CORS.AllowCredentials) // bool型は %t
	log.Printf("    MaxAge (seconds): %d", Cfg.CORS.MaxAge)           // int型は %d
	log.Printf("    Debug:            %t", Cfg.CORS.Debug)            // bool型は %t
	log.Println("--------------------------")                         // 他のログセクションとの区切り

	return nil
}
