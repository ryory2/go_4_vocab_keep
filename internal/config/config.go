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

type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Server   ServerConfig   `mapstructure:"server"`
	App      AppConfig      `mapstructure:"app"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Log      LogConfig      `mapstructure:"log"` // Log設定用のフィールドを追加
}

var Cfg Config

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
	viper.SetEnvPrefix("APP") // 例: APP_AUTH_ENABLED のように接頭辞をつける場合
	viper.AutomaticEnv()      // 環境変数を自動で読み込む

	// 環境変数を Config 構造体のフィールドに紐付け、config.yamlから環境変数を設定
	// viper.BindEnv("database.url", EnvKeyDatabaseURL) // APP_DATABASE_URL ではなく DATABASE_URL を強制的に読む
	// viper.BindEnv("server.port", EnvKeyServerPort)   // APP_PORT ではなく PORT を強制的に読む
	// viper.BindEnv("log.level", EnvKeyLogLevel)       // APP_LOG_LEVEL ではなく LOG_LEVEL を強制的に読む
	// viper.BindEnv("log.format", EnvKeyLogFormat)     // APP_LOG_FORMAT ではなく LOG_FORMAT を強制的に読む
	// viper.BindEnv("app.review_limit", EnvKeyAppReviewLimit)
	// viper.BindEnv("auth.enabled", EnvKeyAuthEnabled)

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

	// 読み込み完了ログ (LoadConfig内では標準logのまま)
	log.Println("Config loaded successfully")
	log.Printf("  Server Port: %s", Cfg.Server.Port)
	log.Printf("  Review Limit: %d", Cfg.App.ReviewLimit)
	log.Printf("  Database URL Set: %t", Cfg.Database.URL != "") // URL自体は出力しない
	log.Printf("  Auth Enabled: %t", Cfg.Auth.Enabled)
	log.Printf("  Log Level: %s", Cfg.Log.Level)
	log.Printf("  Log Format: %s", Cfg.Log.Format)

	return nil
}
