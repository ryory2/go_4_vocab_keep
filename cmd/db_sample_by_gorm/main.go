package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger" // Optional: for logging SQL queries
)

// User モデル定義 (GORM 用)
// テーブル名は構造体名 `User` から複数形の `users` と自動推論されます。
// カラム名はフィールド名 `CreatedAt` -> `created_at` のように自動変換されます。
type User struct {
	ID        int64     `gorm:"primaryKey"` // 主キーを指定 (int64 or uint)
	Name      string    `gorm:"not null"`   // NOT NULL 制約
	CreatedAt time.Time // GORM が自動で created_at カラムとして認識
	UpdatedAt time.Time // GORM が自動で updated_at カラムとして認識
}

// TableName メソッドを定義すると、テーブル名を明示的に指定できます（オプション）
// func (User) TableName() string {
//  return "my_custom_users_table"
// }

func main() {
	// --- 1. データベースへの接続 (GORM) ---
	// 環境変数 DATABASE_URL から接続文字列を取得 (なければデフォルト値を使用)
	// GORM の PostgreSQL ドライバはデータベースURL形式を受け付けます
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// !!重要!!: 以下にご自身のデータベース接続情報を設定してください
		// Docker Compose 環境の場合はホスト名をサービス名 (例: task_postgres) にします。
		dbURL = "postgres://admin:password@task_postgres:5432/vocab_keep?sslmode=disable"
		log.Println("DATABASE_URL environment variable not set, using default:", dbURL)
	}

	// GORM ロガーの設定 (オプション: SQL をコンソールに出力)
	// logger.Info にすると実行される SQL が表示されます
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer (ログ出力先)
		logger.Config{
			SlowThreshold:             200 * time.Millisecond, // 遅い SQL の閾値
			LogLevel:                  logger.Info,            // ログレベル (Silent, Error, Warn, Info)
			IgnoreRecordNotFoundError: true,                   // ErrRecordNotFound をログ出力しない
			Colorful:                  true,                   // カラーログを有効化
		},
	)

	// GORM でデータベースに接続
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: newLogger, // 設定したロガーを使用
	})
	if err != nil {
		log.Fatalf("Failed to connect database using GORM: %v", err)
	}

	// GORM は内部的に *sql.DB (コネクションプール) を持つ。Ping で接続確認可能
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	err = sqlDB.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Successfully connected to database using GORM!")

	// --- GORM AutoMigrate (参考) ---
	// もしテーブルが存在しない場合に、User 構造体に基づいて自動でテーブルを作成したい場合、
	// 以下のコメントアウトを解除して実行します。
	// ただし、既存のテーブル構造と完全に一致しない場合や、本番環境では
	// スキーマ管理に `migrate` ツールを使うことが強く推奨されます。
	// err = db.AutoMigrate(&User{})
	// if err != nil {
	//  log.Fatalf("Failed to auto migrate: %v", err)
	// }
	// fmt.Println("Auto migration completed (if necessary).")

	// --- 2. レコードの書き込み (Create) ---
	fmt.Println("\n--- Creating a new user ---")
	// 構造体のポインタを渡してレコードを作成
	user1 := User{Name: "Charlie"}
	// db.Create() は渡した構造体のフィールド (ID, CreatedAt, UpdatedAt など) を自動で更新します
	result := db.Create(&user1)
	if result.Error != nil {
		// エラーハンドリング (例: ユニーク制約違反など)
		log.Printf("Failed to create user 'Charlie': %v", result.Error)
	} else {
		fmt.Printf("Created user: ID=%d, Name=%s\n", user1.ID, user1.Name)
		// result.RowsAffected は挿入された行数を返します (通常は 1)
		fmt.Printf("  -> RowsAffected: %d\n", result.RowsAffected)
	}
	createdID1 := user1.ID // 後で読み込みや削除に使用するため ID を保持

	fmt.Println("\n--- Creating another user ---")
	user2 := User{Name: "David"}
	result = db.Create(&user2)
	if result.Error != nil {
		log.Printf("Failed to create user 'David': %v", result.Error)
	} else {
		fmt.Printf("Created user: ID=%d, Name=%s\n", user2.ID, user2.Name)
	}
	createdID2 := user2.ID // 後で削除に使用するため ID を保持

	// --- 3. レコードの読み込み (Read) ---

	// --- 3a. IDを指定して単一レコードを読み込む (First) ---
	if createdID1 > 0 { // 最初の挿入が成功していたら
		fmt.Printf("\n--- Getting user by ID (First): %d ---\n", createdID1)
		var foundUser1 User
		// db.First() は主キーで検索し、見つかった最初のレコードを取得します。
		// 第2引数に検索条件となる主キーの値を渡します。
		// 結果は第1引数で渡した構造体のポインタに格納されます。
		result = db.First(&foundUser1, createdID1)
		if result.Error != nil {
			// レコードが見つからない場合は gorm.ErrRecordNotFound が返ります
			if result.Error == gorm.ErrRecordNotFound {
				fmt.Printf("User with ID %d not found.\n", createdID1)
			} else {
				// その他のデータベースエラー
				log.Printf("Failed to get user by ID %d: %v", createdID1, result.Error)
			}
		} else {
			fmt.Printf("Found user: ID=%d, Name=%s, CreatedAt=%s, UpdatedAt=%s\n",
				foundUser1.ID, foundUser1.Name, foundUser1.CreatedAt.Format(time.RFC3339), foundUser1.UpdatedAt.Format(time.RFC3339))
		}
	}

	// --- 3b. 全てのレコードを読み込む (Find) ---
	fmt.Println("\n--- Getting all users (Find) ---")
	var allUsers []User
	// db.Find() は条件に一致する全てのレコードを取得します (条件を指定しなければ全件)。
	// 結果は第1引数で渡したスライスのポインタに格納されます。
	result = db.Find(&allUsers)
	if result.Error != nil {
		// Find では ErrRecordNotFound は返らない (結果が空のスライスになる)
		log.Fatalf("Failed to get all users: %v", result.Error)
	}

	if len(allUsers) == 0 {
		fmt.Println("No users found in the database.")
	} else {
		// result.RowsAffected は SELECT クエリで取得した行数を返します
		fmt.Printf("Found %d users (RowsAffected: %d):\n", len(allUsers), result.RowsAffected)
		for _, u := range allUsers {
			fmt.Printf("- ID=%d, Name=%s, CreatedAt=%s, UpdatedAt=%s\n",
				u.ID, u.Name, u.CreatedAt.Format(time.RFC3339), u.UpdatedAt.Format(time.RFC3339))
		}
	}

	// --- (参考) レコードの更新 (Update) ---
	// 更新のサンプルも入れておきます
	if createdID1 > 0 {
		fmt.Printf("\n--- Updating user name for ID: %d ---\n", createdID1)
		// 更新にはいくつかの方法があります
		// 方法1: Model と Update で特定のフィールドを更新
		//        Map を使うとゼロ値も更新できる
		// result = db.Model(&User{}).Where("id = ?", createdID1).Update("name", "Charlie (Updated)")

		// 方法2: 対象のレコードを取得してから Save で全フィールドを更新
		//        (構造体のゼロ値フィールドも NULL や 0 で上書きされるので注意)
		var userToUpdate User
		if err := db.First(&userToUpdate, createdID1).Error; err == nil {
			userToUpdate.Name = "Charlie (Updated by Save)"
			result = db.Save(&userToUpdate) // Save は UpdatedAt も自動更新します
			if result.Error != nil {
				log.Printf("Failed to update user %d using Save: %v", createdID1, result.Error)
			} else {
				fmt.Printf("Updated user %d using Save. RowsAffected: %d\n", createdID1, result.RowsAffected)
				// 更新後の状態を再取得して確認
				var updatedUser User
				db.First(&updatedUser, createdID1)
				fmt.Printf("  -> New Name: %s, New UpdatedAt: %s\n", updatedUser.Name, updatedUser.UpdatedAt.Format(time.RFC3339))
			}
		} else {
			log.Printf("Could not find user %d to update.", createdID1)
		}
	}

	// --- 4. レコードの削除 (Delete) ---
	if createdID2 > 0 { // 2番目の挿入が成功していたら
		fmt.Printf("\n--- Deleting user by ID: %d ---\n", createdID2)
		// db.Delete() でレコードを削除します。
		// 第1引数にモデルの型 (空の構造体のポインタでOK)、第2引数に削除条件 (主キー) を渡します。
		result = db.Delete(&User{}, createdID2)
		if result.Error != nil {
			log.Printf("Failed to delete user with ID %d: %v", createdID2, result.Error)
		} else {
			// result.RowsAffected は削除された行数を返します
			fmt.Printf("Deleted user(s). RowsAffected: %d\n", result.RowsAffected)
		}

		// --- 5. 削除されたか確認 ---
		fmt.Println("\n--- Getting all users after deletion ---")
		var usersAfterDelete []User
		// 再度全件取得
		db.Find(&usersAfterDelete) // エラーチェックは省略 (デモのため)
		fmt.Printf("Found %d users:\n", len(usersAfterDelete))
		for _, u := range usersAfterDelete {
			fmt.Printf("- ID=%d, Name=%s, CreatedAt=%s, UpdatedAt=%s\n",
				u.ID, u.Name, u.CreatedAt.Format(time.RFC3339), u.UpdatedAt.Format(time.RFC3339))
		}
	}

	fmt.Println("\n--- GORM Sample finished ---")
}
