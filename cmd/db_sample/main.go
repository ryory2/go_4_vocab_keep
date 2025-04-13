package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	// PostgreSQLドライバのインポート
	// _ はドライバの registration のためだけで、直接コード内で使わないため
	_ "github.com/lib/pq"
)

// ユーザー情報を保持する構造体 (email を削除し、UpdatedAt を追加)
type User struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time // updated_at カラムを追加
}

func main() {
	// --- 1. データベースへの接続 ---
	// 環境変数 DATABASE_URL から接続文字列を取得 (なければデフォルト値を使用)
	// 例: "postgres://youruser:yourpassword@host:port/yourdatabase?sslmode=disable"
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// !!重要!!: 以下にご自身のデータベース接続情報を設定してください
		// Docker Compose 環境の場合はホスト名をサービス名 (例: task_postgres) にします。
		dbURL = "postgres://admin:password@task_postgres:5432/vocab_keep?sslmode=disable"
		log.Println("DATABASE_URL environment variable not set, using default:", dbURL)
	}

	// sql.Open で *sql.DB オブジェクトを取得
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	// main 関数終了時に必ずデータベース接続を閉じる
	defer db.Close()

	// 実際にデータベースへの接続を確認 (Ping)
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Successfully connected to database!")

	// --- 2. レコードの書き込み (INSERT) ---
	fmt.Println("\n--- Inserting a new user ---")
	newUserName := "Alice"
	// insertUser に email 引数は不要
	insertedID, err := insertUser(db, newUserName)
	if err != nil {
		log.Printf("Failed to insert user: %v", err)
	} else {
		fmt.Printf("Inserted user %s with ID: %d\n", newUserName, insertedID)
	}

	// 別のユーザーも挿入
	fmt.Println("\n--- Inserting another user ---")
	newUserName2 := "Bob"
	// insertUser に email 引数は不要
	insertedID2, err := insertUser(db, newUserName2)
	if err != nil {
		log.Printf("Failed to insert user: %v", err)
	} else {
		fmt.Printf("Inserted user %s with ID: %d\n", newUserName2, insertedID2)
	}

	// --- 3. レコードの読み込み (SELECT) ---

	// --- 3a. IDを指定して単一レコードを読み込む ---
	if insertedID > 0 { // 最初の挿入が成功していたら
		fmt.Printf("\n--- Getting user by ID: %d ---\n", insertedID)
		user, err := getUserByID(db, insertedID)
		if err != nil {
			log.Printf("Failed to get user by ID %d: %v", insertedID, err)
		} else if user != nil {
			// Email 表示を削除し、UpdatedAt を追加
			fmt.Printf("Found user: ID=%d, Name=%s, CreatedAt=%s, UpdatedAt=%s\n",
				user.ID, user.Name, user.CreatedAt.Format(time.RFC3339), user.UpdatedAt.Format(time.RFC3339))
		} else {
			fmt.Printf("User with ID %d not found.\n", insertedID)
		}
	}

	// --- 3b. 全てのレコードを読み込む ---
	fmt.Println("\n--- Getting all users ---")
	users, err := getAllUsers(db)
	if err != nil {
		log.Fatalf("Failed to get all users: %v", err)
	}
	if len(users) == 0 {
		fmt.Println("No users found in the database.")
	} else {
		fmt.Printf("Found %d users:\n", len(users))
		for _, u := range users {
			// Email 表示を削除し、UpdatedAt を追加
			fmt.Printf("- ID=%d, Name=%s, CreatedAt=%s, UpdatedAt=%s\n",
				u.ID, u.Name, u.CreatedAt.Format(time.RFC3339), u.UpdatedAt.Format(time.RFC3339))
		}
	}

	// --- 4. レコードの削除 (DELETE) ---
	if insertedID2 > 0 { // 2番目の挿入が成功していたら
		fmt.Printf("\n--- Deleting user by ID: %d ---\n", insertedID2)
		rowsAffected, err := deleteUser(db, insertedID2)
		if err != nil {
			log.Printf("Failed to delete user with ID %d: %v", insertedID2, err)
		} else {
			fmt.Printf("Deleted %d user(s).\n", rowsAffected)
		}

		// --- 5. 削除されたか確認 ---
		fmt.Println("\n--- Getting all users after deletion ---")
		usersAfterDelete, err := getAllUsers(db)
		if err != nil {
			log.Fatalf("Failed to get all users after deletion: %v", err)
		}
		fmt.Printf("Found %d users:\n", len(usersAfterDelete))
		for _, u := range usersAfterDelete {
			// Email 表示を削除し、UpdatedAt を追加
			fmt.Printf("- ID=%d, Name=%s, CreatedAt=%s, UpdatedAt=%s\n",
				u.ID, u.Name, u.CreatedAt.Format(time.RFC3339), u.UpdatedAt.Format(time.RFC3339))
		}
	}

	fmt.Println("\n--- Sample finished ---")
}

// --- ヘルパー関数 ---

// insertUser は新しいユーザーをデータベースに挿入し、挿入されたレコードのIDを返します。
// email 引数を削除し、SQL 文を修正
func insertUser(db *sql.DB, name string) (int64, error) {
	sqlStatement := `INSERT INTO users (name) VALUES ($1) RETURNING id`
	var insertedID int64
	// name だけを渡すように修正
	err := db.QueryRow(sqlStatement, name).Scan(&insertedID)
	if err != nil {
		return 0, fmt.Errorf("insertUser: %w", err)
	}
	return insertedID, nil
}

// getUserByID は指定されたIDのユーザー情報をデータベースから取得します。
// SQL 文と Scan の引数を修正
func getUserByID(db *sql.DB, id int64) (*User, error) {
	// email を削除し、updated_at を追加
	sqlStatement := `SELECT id, name, created_at, updated_at FROM users WHERE id = $1`
	row := db.QueryRow(sqlStatement, id)

	var user User
	// &user.Email を削除し、&user.UpdatedAt を追加
	err := row.Scan(&user.ID, &user.Name, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 見つからない場合はエラーではなく nil を返す設計
		}
		return nil, fmt.Errorf("getUserByID %d: %w", id, err)
	}
	return &user, nil
}

// getAllUsers はデータベース内の全てのユーザー情報を取得します。
// SQL 文と Scan の引数を修正
func getAllUsers(db *sql.DB) ([]User, error) {
	// email を削除し、updated_at を追加
	sqlStatement := `SELECT id, name, created_at, updated_at FROM users ORDER BY id`
	rows, err := db.Query(sqlStatement)
	if err != nil {
		return nil, fmt.Errorf("getAllUsers query: %w", err)
	}
	// 関数終了時に必ず rows を閉じる
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		// &user.Email を削除し、&user.UpdatedAt を追加
		if err := rows.Scan(&user.ID, &user.Name, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, fmt.Errorf("getAllUsers scan: %w", err)
		}
		users = append(users, user)
	}
	// ループ終了後にエラーが発生していないか確認
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("getAllUsers rows err: %w", err)
	}

	return users, nil
}

// deleteUser は指定されたIDのユーザーをデータベースから削除し、削除された行数を返します。
// (この関数は変更不要)
func deleteUser(db *sql.DB, id int64) (int64, error) {
	sqlStatement := `DELETE FROM users WHERE id = $1`
	result, err := db.Exec(sqlStatement, id)
	if err != nil {
		return 0, fmt.Errorf("deleteUser %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("deleteUser %d: failed to get rows affected: %w", id, err)
	}

	return rowsAffected, nil
}
