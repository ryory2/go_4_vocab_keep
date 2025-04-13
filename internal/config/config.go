// internal/config/config.go
package config

import (
	"log"
	// os をインポート (環境変数チェック用に追加)
	"github.com/spf13/viper"
)

type Config struct {
	Database struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"database"`
	Server struct {
		Port string `mapstructure:"port"`
	} `mapstructure:"server"`
	App struct {
		ReviewLimit int `mapstructure:"review_limit"`
	} `mapstructure:"app"`
	Auth struct { // <--- Auth フィールドを追加
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"auth"`
}

var Cfg Config

func LoadConfig(path string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(path)
	viper.AddConfigPath(".")

	// 環境変数名を指定して読み込むことも可能 (例: AUTH_ENABLED)
	viper.SetEnvPrefix("APP")                     // 例: APP_AUTH_ENABLED のように接頭辞をつける場合
	viper.AutomaticEnv()                          // 環境変数を自動で読み込む
	viper.BindEnv("auth.enabled", "AUTH_ENABLED") // AUTH_ENABLED 環境変数を auth.enabled に紐付け

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Warning: Config file not found. Using default settings or environment variables if available.")
		} else {
			log.Printf("Error reading config file: %s\n", err)
			return err
		}
	}

	err := viper.Unmarshal(&Cfg)
	if err != nil {
		log.Printf("Error unmarshalling config: %s\n", err)
		return err
	}

	// --- デフォルト値の設定 ---
	if Cfg.Server.Port == "" {
		log.Println("Server port not set, using default ':8080'")
		Cfg.Server.Port = ":8080"
	}
	if Cfg.App.ReviewLimit <= 0 {
		log.Println("App review limit not set or invalid, using default '20'")
		Cfg.App.ReviewLimit = 20
	}
	if Cfg.Database.URL == "" {
		log.Println("Warning: Database URL is not set in config.")
	}

	// Auth.Enabled のデフォルト値を設定 (未設定なら true = 有効 にする)
	// viper.IsSet() で設定ファイルや環境変数で明示的に設定されたか確認できる
	if !viper.IsSet("auth.enabled") {
		// 環境変数 NODE_ENV や APP_ENV などで開発環境かどうかを判定しても良い
		// if os.Getenv("APP_ENV") == "development" {
		//     log.Println("Auth enabled flag not set in development, defaulting to false (disabled)")
		//     Cfg.Auth.Enabled = false
		// } else {
		log.Println("Auth enabled flag not set, defaulting to true (enabled)")
		Cfg.Auth.Enabled = true
		// }
	}

	log.Println("Config loaded successfully")
	log.Printf("Server Port: %s", Cfg.Server.Port)
	log.Printf("Review Limit: %d", Cfg.App.ReviewLimit)
	log.Printf("Auth Enabled: %t", Cfg.Auth.Enabled) // 認証状態をログ出力

	return nil
}
