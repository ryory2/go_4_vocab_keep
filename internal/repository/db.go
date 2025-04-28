package repository

import (
	"log/slog"
	"os"
	"strings"
	"time"

	slogGorm "github.com/orandin/slog-gorm" // slogGormはエイリアス
	"gorm.io/driver/postgres"               // postgresドライバ
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// インスタンス
func NewDB(databaseURL string, appLogger *slog.Logger) (*gorm.DB, error) {

	// === slog を利用する GORM Logger の設定 ===
	var gormLogLevel gormlogger.LogLevel
	// 例: 環境変数 APP_ENV によって GORM のログレベルを切り替え
	if strings.ToLower(os.Getenv("APP_ENV")) == "dev" {
		gormLogLevel = gormlogger.Info
	} else {
		gormLogLevel = gormlogger.Warn
	}

	// slog-gorm ロガーを作成 (slogGorm.Interface を返す)
	slogGormLogger := slogGorm.New(
		slogGorm.WithHandler(appLogger.Handler()),
		slogGorm.WithTraceAll(),
		slogGorm.WithSlowThreshold(500*time.Millisecond), // 遅いクエリの閾値を調整
	)

	// LogMode を適用して、最終的な gormlogger.Interface を取得
	// ★ 修正点: LogModeの結果を別の変数に格納するか、直接Configに渡す ★
	finalGormLogger := slogGormLogger.LogMode(gormLogLevel)

	// === GORM 接続設定 ===
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		// ★ 修正点: LogMode を適用した後のロガー (gormlogger.Interface 型) を設定 ★
		Logger: finalGormLogger,
		// 他の GORM 設定 (例: NamingStrategy など)
		// NamingStrategy: schema.NamingStrategy{ ... },
	})

	if err != nil {
		// ★ エラーログは注入された appLogger を使う ★
		appLogger.Error("Failed to connect to database with GORM", slog.Any("error", err))
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		appLogger.Error("Error getting underlying sql.DB from GORM", slog.Any("error", err))
		return nil, err
	}

	// Pingで接続確認
	if err = sqlDB.Ping(); err != nil {
		appLogger.Error("Error pinging database", slog.Any("error", err))
		sqlDB.Close() // Ping失敗時はここでClose
		return nil, err
	}

	// コネクションプールの設定 (推奨)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// ★ 接続成功ログも appLogger を使う ★
	appLogger.Info("Database connection established with GORM")

	// AutoMigrate (必要であれば)
	// ...

	return db, nil
}
