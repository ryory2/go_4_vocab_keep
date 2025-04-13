// internal/repository/db.go
package repository

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres" // postgresドライバ
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDB は GORM DB インスタンスを初期化します
func NewDB(databaseURL string) (*gorm.DB, error) {
	// GORMロガー設定
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info, // Log level (Silent, Error, Warn, Info)
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Disable color
		},
	)

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		log.Printf("Error connecting to database with GORM: %v\n", err)
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("Error getting underlying sql.DB from GORM: %v\n", err)
		return nil, err
	}

	// Pingで接続確認
	if err = sqlDB.Ping(); err != nil {
		log.Printf("Error pinging database: %v\n", err)
		sqlDB.Close()
		return nil, err
	}

	// コネクションプールの設定 (推奨)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("Database connection established with GORM")

	// AutoMigrate は開発初期やテストでは便利だが、本番環境では
	// golang-migrate/migrate のようなマイグレーションツールを使うことを強く推奨します。
	// err = db.AutoMigrate(&model.Tenant{}, &model.Word{}, &model.LearningProgress{})
	// if err != nil {
	//  log.Printf("Warning: Failed to auto migrate database: %v", err)
	// } else {
	//  log.Println("Database auto migration checked/completed.")
	// }

	return db, nil
}
