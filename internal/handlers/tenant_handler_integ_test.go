// internal/handlers/tenant_handler_integ_test.go
package handlers_test // _test パッケージ

import (
	"bytes"
	"encoding/json"
	"io" // ★ io.Discard のためにインポート
	"log"
	"log/slog" // ★ slog をインポート
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	// DB接続のタイムアウトなどに
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres" // 使用するDBドライバ
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	// 実際の依存関係をインポート
	"go_4_vocab_keep/internal/handlers"
	"go_4_vocab_keep/internal/model"      // アプリケーションのモデル
	"go_4_vocab_keep/internal/repository" // 実際のリポジトリパッケージ
	"go_4_vocab_keep/internal/service"    // 実際のサービスパッケージ
	// "go_4_vocab_keep/internal/webutil" // 必要に応じてwebutilも
)

// 関数の実行順は以下
// TestMain が開始され、DB接続などの全体セットアップを行う。
// TestMain が m.Run() を呼び出す。
// TestTenantHandler_CreateTenant_Integration が実行される。
// 	clearTenantsTable が実行される (テーブルクリア)。
// 	テストケースのループが始まる。
// 	各テストケースについて t.Run が実行される。
// 		リクエストが準備され、ハンドラ (CreateTenant) が実行される。
// 		レスポンスが検証される。
// 		verifyDB が定義されていれば、その中のDB検証コードが実行される。
// 	すべてのテストケースが終わる。
// 	TestTenantHandler_CreateTenant_Integration が終了する。
// m.Run() が終了し、TestMain に処理が戻る。
// TestMain がDB切断などの全体ティアダウンを行う。
// テストプロセスが終了する。

// --- グローバル変数 (テスト用DB接続) ---
var (
	testDB *gorm.DB
)

// --- TestMain: テスト全体のセットアップとティアダウン ---
func TestMain(m *testing.M) {
	// --- テスト用DBへの接続 ---
	// 環境変数や設定ファイルからテスト用DBのURLを取得
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// 環境変数がなければデフォルト値（ローカル環境などに合わせて調整）
		dsn = "postgres://admin:password@task_postgres:5432/vocab_keep?sslmode=disable"
		log.Printf("DATABASE_URL not set, using default: %s", dsn)
	}

	var err error
	// GORMでDBに接続。テスト中はログレベルをSilentに設定することが多い。
	testDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // テスト中はSQLログを抑制
	})
	if err != nil {
		log.Fatalf("Failed to connect to test database: %v", err)
	}

	// DB接続確認 (任意だが推奨)
	sqlDB, err := testDB.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	if err = sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping test database: %v", err)
	}

	// (任意) ここでDBマイグレーションを実行することも可能
	// 例: migrateDatabase(testDB)

	// --- テストの実行 ---
	log.Println("Running integration tests...")
	exitCode := m.Run() // ここで各テスト関数が実行される

	// --- ティアダウン ---
	// DB接続を閉じる
	log.Println("Closing database connection...")
	if err = sqlDB.Close(); err != nil {
		log.Printf("Warning: failed to close test database connection: %v", err)
	}

	log.Println("Integration tests finished.")
	os.Exit(exitCode)
}

// --- ヘルパー関数: テーブルをクリア ---
func clearTenantsTable(t *testing.T) {
	t.Helper() // これを追加すると、エラー発生時の行番号が呼び出し元になる
	// テーブルデータを削除し、シーケンスもリセットする（PostgreSQLの場合）
	result := testDB.Exec("TRUNCATE TABLE tenants RESTART IDENTITY CASCADE")
	// require はエラー発生時にテストを即座に中断させる
	require.NoError(t, result.Error, "Failed to truncate tenants table")
}

// --- インテグレーションテスト関数 ---
func TestTenantHandler_CreateTenant_Integration(t *testing.T) {
	// --- テストごとのセットアップ ---
	// 各テスト実行前にテーブルをクリアして独立性を保つ
	clearTenantsTable(t)

	// --- テスト用ロガーの作成 ---
	// ログ出力を無視するロガーを作成
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil)) // ★ テスト用ロガー作成

	// --- 実際の依存関係を使ってハンドラをセットアップ ---
	// NewGormTenantRepository が *gorm.DB を引数に取る場合:
	// (もし NewGormTenantRepository も logger を取るように変更した場合は、ここも修正が必要)
	tenantRepo := repository.NewGormTenantRepository(nil) // リポジトリの実際の実装 (DB接続と必要ならロガーを渡す)
	// 実際のサービス実装 (テストDB接続とリポジトリを使用)
	tenantService := service.NewTenantService(testDB, tenantRepo, testLogger)
	tenantHandler := handlers.NewTenantHandler(tenantService, testLogger) // ★ ハンドラの実際の実装にロガーを渡す

	// --- ルーターのセットアップ (テスト対象のエンドポイントを登録) ---
	router := chi.NewRouter()
	router.Post("/tenants", tenantHandler.CreateTenant)

	// --- テストケースの定義 ---
	// テストごとにユニークな名前を生成して、他のテストケースとの干渉を避ける
	uniqueTenantName1 := "Integ Test Tenant " + uuid.NewString()[:8]
	uniqueTenantName2 := "Another Integ Tenant " + uuid.NewString()[:8]

	tests := []struct {
		name             string                                    // テストケース名
		requestBody      interface{}                               // 送信するリクエストボディ (struct または 不正なJSON文字列)
		expectedStatus   int                                       // 期待するHTTPステータスコード
		expectBody       bool                                      // 正常なレスポンスボディ(Tenant情報)を期待するか
		expectedRespName string                                    // 期待されるレスポンス内のテナント名 (成功時)
		expectedDBName   string                                    // DB検証時に使う名前 (成功/重複ケース用)
		expectedErrMsg   string                                    // 期待されるエラーメッセージの部分文字列 (エラー時)
		verifyDB         func(t *testing.T, expectedDBName string) // DBの状態を検証する関数
	}{
		{
			name:             "正常系: 新規テナント作成成功",
			requestBody:      handlers.CreateTenantRequest{Name: uniqueTenantName1},
			expectedStatus:   http.StatusCreated,
			expectBody:       true,
			expectedRespName: uniqueTenantName1,
			expectedDBName:   uniqueTenantName1, // DB検証用に名前を保持
			verifyDB: func(t *testing.T, expectedDBName string) {
				t.Helper()
				var tenant model.Tenant
				result := testDB.Where("name = ?", expectedDBName).First(&tenant)
				require.NoError(t, result.Error, "Tenant [%s] should exist in DB", expectedDBName)
				assert.Equal(t, expectedDBName, tenant.Name)
				assert.NotEqual(t, uuid.Nil, tenant.TenantID, "TenantID should be set")
			},
		},
		{
			name:           "異常系: バリデーションエラー (nameが空)",
			requestBody:    handlers.CreateTenantRequest{Name: ""},
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
			expectedDBName: "",                  // DB検証用 (空の名前で検索)
			expectedErrMsg: "Validation failed", // バリデーションエラーのメッセージの一部
			verifyDB: func(t *testing.T, expectedDBName string) {
				t.Helper()
				var count int64
				// バリデーションエラーの場合、DBには何も作成されないはず
				testDB.Model(&model.Tenant{}).Where("name = ?", expectedDBName).Count(&count)
				assert.Zero(t, count, "No tenant should be created on validation error")
			},
		},
		{
			// このテストケースは、verifyDB内で2回目のリクエストを実行する構造
			name:             "異常系: 重複エラー (同じ名前で2回作成)",
			requestBody:      handlers.CreateTenantRequest{Name: uniqueTenantName2}, // 最初の作成リクエスト
			expectedStatus:   http.StatusCreated,                                    // 1回目は成功する
			expectBody:       true,
			expectedRespName: uniqueTenantName2,
			expectedDBName:   uniqueTenantName2, // DB検証用に名前を保持
			verifyDB: func(t *testing.T, expectedDBName string) {
				t.Helper()
				log.Printf("Running second request for conflict test with name: %s", expectedDBName)

				// --- 2回目のリクエストを準備・実行 ---
				reqBodyBytes, _ := json.Marshal(handlers.CreateTenantRequest{Name: expectedDBName})
				req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBuffer(reqBodyBytes))
				req.Header.Set("Content-Type", "application/json")
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req) // 同じルーターを使ってリクエストを処理

				// --- 2回目のレスポンスを検証 (Conflictを期待) ---
				assert.Equal(t, http.StatusConflict, rr.Code, "Expected Conflict status on second request")
				var errResp map[string]string // エラーレスポンスの形式を想定
				err := json.Unmarshal(rr.Body.Bytes(), &errResp)
				// エラーレスポンスのキーが 'message' であることを想定（実際のレスポンス形式に合わせる）
				if assert.NoError(t, err, "Failed to unmarshal conflict error response") {
					// model.ErrConflict.Error() 等、期待するエラーメッセージが含まれるか確認
					// ★ クライアント向けのエラーメッセージをチェック
					assert.Contains(t, errResp["message"], "Failed to create tenant", "Conflict error message mismatch")
				}

				// --- DBの状態を最終確認 (1件だけ存在すること) ---
				var count int64
				testDB.Model(&model.Tenant{}).Where("name = ?", expectedDBName).Count(&count)
				assert.Equal(t, int64(1), count, "Only one tenant [%s] should exist after conflict attempt", expectedDBName)
			},
		},
		// --- テストケースの一つを定義する構造体の要素 ---
		// これは、複数のテストパターン（テストケース）をまとめたリスト（スライス）の中の一つです。
		// 各テストケースは、特定の入力と、それに対する期待される結果、
		// そして必要であれば追加の検証処理を定義します。
		{
			// --- name フィールド ---
			// このテストケースの名前（説明）を文字列で指定します。
			// テスト実行時に表示されるので、何をしているテストか分かりやすい名前をつけます。
			name: "異常系: JSON構文エラー (閉じ括弧なし)",

			// --- requestBody フィールド ---
			// このテストケースでHTTPリクエストのボディとして送信するデータを指定します。
			// ここでは、わざと構文が間違っているJSON文字列を ` ` (バッククォート) で囲んで定義しています。
			// 末尾の閉じ括弧 } が欠けているため、JSONとして不正なデータです。
			requestBody: `{"name": "invalid json"`, // 不正なJSON文字列

			// --- expectedStatus フィールド ---
			// このテストケースのリクエストを送った結果、期待されるHTTPステータスコードを指定します。
			// 不正なリクエストなので、サーバーは "Bad Request" (400番) を返すはずだと期待しています。
			// http.StatusBadRequest はGoの標準ライブラリで定義されている 400 を示す定数です。
			expectedStatus: http.StatusBadRequest,

			// --- expectBody フィールド ---
			// このテストケースで、正常なレスポンスボディ（例えば作成されたテナントの情報）を
			// 期待するかどうかを true/false で指定します。
			// 今回はエラーになるはずなので、正常なボディは期待せず、false を指定します。
			expectBody: false,

			// --- expectedDBName フィールド ---
			// このテストケースの検証（特にDB検証）で使うための補助的なデータです。
			// ここでは、不正なJSONに含まれていた（であろう）名前を保持しています。
			// このケースでは、この名前のデータがDBに「作られていないこと」を確認するために使います。
			expectedDBName: "invalid json", // 不正JSON内の名前（DBには入らないはず）

			// --- expectedErrMsg フィールド ---
			// このテストケースで、エラーレスポンスのボディに含まれると期待される
			// エラーメッセージの一部（部分文字列）を指定します。
			// 不正なJSONリクエストを処理した場合、"Invalid request body" というメッセージが
			// 返ってくるはずだと期待しています。
			expectedErrMsg: "Invalid request body", // デコードエラー時のメッセージ

			// --- verifyDB フィールド ---
			// このテストケースの実行後、データベースの状態が期待通りかを確認するための
			// 追加の処理（関数）を指定します。
			// ここでは、「無名関数」または「匿名関数」と呼ばれる、その場で定義される関数を使っています。

			// --- 無名関数の定義 ---
			// verifyDB: func(t *testing.T, expectedDBName string) { ... }
			//
			// 関数の書式（シグネチャ）の説明:
			// func                  : これから関数を定義しますよ、というキーワード。
			// (                     : 関数の引数リストの始まり。
			//   t *testing.T        : 1番目の引数。
			//                         - `t`        : 引数名（関数の中でこの名前で使う）。
			//                         - `*testing.T`: 引数の型。Goのテスト機能を使うための特別な型へのポインタ。
			//                                      テストの失敗を報告したり(t.Fail(), assertなど)、ログを出したり(t.Logf)するのに使う。
			//   ,                   : 引数の区切り。
			//   expectedDBName string : 2番目の引数。
			//                         - `expectedDBName` : 引数名。
			//                         - `string`         : 引数の型（文字列）。このテストケースのexpectedDBNameフィールドの値が渡される。
			// )                     : 引数リストの終わり。
			// {                     : 関数の処理本体の始まり。
			verifyDB: func(t *testing.T, expectedDBName string) {
				// --- 関数本体の処理 ---

				// t.Helper() を呼び出すと、もしこの関数内でテストが失敗した場合（assert などで）、
				// Goのテストフレームワークは、この関数を呼び出した元のテストケースの行番号を
				// エラー箇所として報告してくれるようになります（デバッグ時に便利）。
				t.Helper()

				// データベースから取得したレコード数を格納するための変数を宣言します。
				// int64 は 64ビットの整数型で、通常、データベースのレコード数を扱うのに使われます。
				var count int64

				// --- データベースの検証 (1回目) ---
				// testDB は、テスト用に準備されたデータベース接続（GORMの*gorm.DB型）です。
				// .Model(&model.Tenant{}) : 操作対象のテーブルを `model.Tenant` 構造体（= tenants テーブル）に指定します。
				// .Where("name = ?", expectedDBName) : 検索条件を指定します。「name カラムが expectedDBName の値と等しい」レコードを探します。
				//                                     `?` はプレースホルダで、SQLインジェクションを防ぐ安全な方法です。
				// .Count(&count) : 上記の条件に一致するレコード数を数えて、その結果を `count` 変数に格納します。
				//                `&count` は `count` 変数のメモリアドレスを渡しています（Count関数が結果を書き込めるようにするため）。
				testDB.Model(&model.Tenant{}).Where("name = ?", expectedDBName).Count(&count)

				// assert.Zero(t, count, "メッセージ") は、`testify/assert` ライブラリの関数です。
				// `count` 変数の値が 0 であることを表明（アサート）します。
				// もし 0 でなければ、テストは失敗し、指定されたメッセージが表示されます。
				// ここでは、不正なJSONリクエストだったので、特定の名前のテナントがDBに作られていないはず、ということを確認しています。
				assert.Zero(t, count, "No tenant should be created on invalid JSON request")

				// --- データベースの検証 (2回目) ---
				// 念のため、テーブル全体のレコード数も確認します。
				// .Model(&model.Tenant{}) : 再度、tenants テーブルを指定します。
				// .Count(&count)         : 今度は Where 条件なしで、テーブル内の全レコード数を数えます。
				testDB.Model(&model.Tenant{}).Count(&count)

				// テーブルクリア後にこのテストケースを実行しているので、テーブル全体のレコード数も 0 のはずだと表明します。
				// これにより、意図しないデータが（他の原因で）混入していないかもチェックできます。
				assert.Zero(t, count, "Total tenant count should be zero after invalid JSON request (assuming table was cleared)")

			}, // verifyDB 関数の定義終わり
		}, // このテストケースの定義終わり (リストの次の要素へ続くか、リストが終わる)
		// 必要に応じて他のテストケースを追加
		// - バリデーションの境界値 (例: 100文字の名前、101文字の名前)
		// - 他の必須フィールド（もしあれば）
	}

	// --- テストケースの実行ループ ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearTenantsTable(t) // ★ 重複エラーテストのためにループ内に移動 ★
			// 1. リクエストボディの準備
			var reqBodyReader *bytes.Reader
			if bodyStr, ok := tt.requestBody.(string); ok {
				// リクエストボディが文字列（不正JSONテスト用）の場合
				reqBodyReader = bytes.NewReader([]byte(bodyStr))
			} else {
				// リクエストボディが構造体の場合
				reqBodyBytes, err := json.Marshal(tt.requestBody)
				require.NoError(t, err, "Failed to marshal request body for test: %s", tt.name)
				reqBodyReader = bytes.NewReader(reqBodyBytes)
			}

			// 2. HTTPリクエストを作成 (httptest を使用)
			req := httptest.NewRequest(http.MethodPost, "/tenants", reqBodyReader)
			req.Header.Set("Content-Type", "application/json")

			// 3. レスポンスレコーダーを作成 (レスポンスをキャプチャするため)
			rr := httptest.NewRecorder()

			// 4. ハンドラを実行 (ルーター経由で)
			router.ServeHTTP(rr, req)

			// 5. レスポンスのステータスコードを検証
			// ★ 重複エラーテストでは verifyDB 内で2回目のレスポンスを検証するため、ここではスキップ ★
			if tt.name != "異常系: 重複エラー (同じ名前で2回作成)" {
				assert.Equal(t, tt.expectedStatus, rr.Code, "Status code mismatch")
			}

			// 6. レスポンスボディの検証
			// ★ 重複エラーテストでは verifyDB 内で検証するため、ここではスキップ ★
			if tt.name != "異常系: 重複エラー (同じ名前で2回作成)" {
				if tt.expectBody {
					// 正常系のレスポンスボディを検証
					assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Content-Type should be application/json")
					var respTenant model.Tenant
					err := json.Unmarshal(rr.Body.Bytes(), &respTenant)
					require.NoError(t, err, "Failed to unmarshal success response body")
					assert.Equal(t, tt.expectedRespName, respTenant.Name, "Response tenant name mismatch")
					assert.NotEqual(t, uuid.Nil, respTenant.TenantID, "Response TenantID should not be nil")
					assert.False(t, respTenant.CreatedAt.IsZero(), "Response CreatedAt should not be zero")
					assert.False(t, respTenant.UpdatedAt.IsZero(), "Response UpdatedAt should not be zero")
				} else if tt.expectedErrMsg != "" {
					// エラー系のレスポンスボディを検証
					var errResp map[string]string // エラーレスポンスの形式を想定
					err := json.Unmarshal(rr.Body.Bytes(), &errResp)
					// エラーレスポンスのキーが 'message' であることを想定
					if assert.NoError(t, err, "Failed to unmarshal error response body (expected JSON with 'message' key)") {
						assert.Contains(t, errResp["message"], tt.expectedErrMsg, "Error message mismatch")
					} else {
						// JSONとしてパースできなかった場合、生のボディに含まれるか確認 (代替策)
						assert.Contains(t, rr.Body.String(), tt.expectedErrMsg, "Raw error message mismatch in non-JSON response")
					}
				}
			}

			// 7. DBの状態を検証 (verifyDB 関数が定義されていれば実行)
			if tt.verifyDB != nil {
				tt.verifyDB(t, tt.expectedDBName)
			}
		})
	}
}
