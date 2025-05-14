// internal/config/config.go
package config

import (
	"log"
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

func LoadConfig(path string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(path)
	viper.AddConfigPath(".")

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

	// 設定ファイル（config.yaml）の読み込み
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Warning: Config file not found. Using default settings or environment variables if available.")
		} else {
			log.Printf("Error reading config file: %s\n", err)
			return err
		}
	}
	// config.yamlの内容を、前述で定義したConfigに変換（アンマーシャルはデータをプログラムのデータにすること）
	err := viper.Unmarshal(&Cfg)
	if err != nil {
		log.Printf("Error unmarshalling config: %s\n", err)
		return err
	}

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
