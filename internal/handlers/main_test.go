// internal/handlers/main_test.go
package handlers_test // テストパッケージ名は _test サフィックス

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt" // fmt を追加
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing" // testing パッケージ
	"time"    // time を追加

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"go_4_vocab_keep/internal/config"     // config パッケージをインポート (プロジェクト名修正)
	"go_4_vocab_keep/internal/handlers"   // テスト対象のハンドラパッケージ (プロジェクト名修正)
	"go_4_vocab_keep/internal/middleware" // ミドルウェア (プロジェクト名修正)
	"go_4_vocab_keep/internal/model"      // モデル (プロジェクト名修正)
	"go_4_vocab_keep/internal/repository" // リポジトリ (プロジェクト名修正)
	"go_4_vocab_keep/internal/service"    // サービス (プロジェクト名修正)
)

var (
	testDB     *gorm.DB // テスト用DBコネクション (パッケージ全体で共有)
	testRouter *chi.Mux // テスト用ルーター (パッケージ全体で共有)
)

// TestMain は、パッケージ内のテストが実行される前に一度だけ実行される特別な関数です。
// テスト全体のセットアップ（DB接続、ルーター初期化など）とティアダウンを行います。
func TestMain(m *testing.M) {
	// --- セットアップ ---
	log.Println("Setting up handlers test environment...")

	// 1. Viper を使って設定を読み込む
	// config.yaml がプロジェクトルートの configs/ ディレクトリにある想定
	// テスト実行時のカレントディレクトリがプロジェクトルートであることを期待
	if err := config.LoadConfig("configs"); err != nil {
		// go test を internal/handlers で実行した場合などはパスが変わる可能性がある
		// 代替パスも試す (internal/handlers から見て ../configs)
		if err := config.LoadConfig("../configs"); err != nil {
			log.Printf("Warning: Failed to load config file from 'configs' or '../configs', will rely on defaults/env: %v", err)
		} else {
			log.Println("Loaded config from '../configs'")
		}
	} else {
		log.Println("Loaded config from 'configs'")
	}

	// 2. テスト用設定の上書き
	// 環境変数 TEST_DATABASE_URL があればそれを使う
	testDbURL := os.Getenv("TEST_DATABASE_URL")
	if testDbURL != "" {
		log.Printf("Using database URL from TEST_DATABASE_URL environment variable: %s", testDbURL)
		config.Cfg.Database.URL = testDbURL
	} else if config.Cfg.Database.URL == "" {
		// config.yamlにも環境変数にも設定がない場合の最終フォールバック
		// ★★★ 必ずテスト用のDBを指定してください ★★★
		config.Cfg.Database.URL = "postgres://admin:password@container_postgres:5432/vocab_keep?sslmode=disable" // テスト用DBのURL
		log.Printf("Database URL not found in config or env, using hardcoded test default: %s", config.Cfg.Database.URL)
	} else {
		// config.yaml または対応する環境変数から読み込まれた値を使用
		log.Printf("Using database URL from config/env: %s", config.Cfg.Database.URL)
	}

	// テスト中は認証を無効化する設定にする (config.yaml や環境変数で設定されていても上書き)
	config.Cfg.Auth.Enabled = false
	log.Printf("Authentication forced to disabled for tests.")

	// 必要なら他の設定も上書き (例: レビュー上限)
	// config.Cfg.App.ReviewLimit = 10
	// log.Printf("Review limit set to %d for tests.", config.Cfg.App.ReviewLimit)

	// 3. テスト用DBへの接続 (設定された config.Cfg.Database.URL を使用)
	var err error
	testDB, err = repository.NewDB(config.Cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to test database using URL '%s': %v", config.Cfg.Database.URL, err)
	}
	// DB接続確認 (Ping)
	sqlDB, err := testDB.DB()
	if err != nil { // underlying sql.DB の取得エラーチェック
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = sqlDB.PingContext(pingCtx)
	if err != nil {
		log.Fatalf("Failed to ping test database: %v", err)
	}
	log.Println("Connected to test database.")

	// 4. (重要) テスト用DBのマイグレーション実行確認
	// この TestMain 実行前に migrate コマンドなどでマイグレーションが適用されている必要があります。
	log.Println("Assuming test database schema is up-to-date.")

	// 5. テスト用ルーターと依存関係の初期化 (DI)
	// TestMain では主に共通で使うものを初期化
	tenantRepo := repository.NewGormTenantRepository()
	tenantService := service.NewTenantService(testDB, tenantRepo)
	tenantHandler := handlers.NewTenantHandler(tenantService)

	// ルーター設定 (共通部分)
	testRouter = chi.NewRouter()
	// 必要なら共通ミドルウェアを追加
	// testRouter.Use(chimiddleware.Logger)

	// ルート定義
	testRouter.Route("/api/v1", func(r chi.Router) {
		// 公開API
		r.Post("/tenants", tenantHandler.CreateTenant)

		// 認証が必要なAPIグループ
		r.Group(func(r chi.Router) {
			// テスト用認証ミドルウェアを適用
			r.Use(middleware.DevTenantContextMiddleware)

			// ★注意★: ここで wordHandler や reviewHandler を使うルートを定義すると、
			// それらのハンドラが依存するサービス（WordService, ReviewService）の
			// 実インスタンスが必要になる。
			// 各テストファイルでモックを使う場合、共通ルーターには含めず、
			// テストファイル内でモックハンドラを登録したローカルルーターを使う方が良い。
			// もし共通ルーターを使いたいなら、ハンドラ生成時に nil を渡すなどの工夫が必要。
			// 例:
			// wordHandlerForRoute := handlers.NewWordHandler(nil) // モックは後でテスト内で使う
			// r.Route("/words", func(r chi.Router) {
			// 	r.Post("/", wordHandlerForRoute.CreateWord)
			// 	r.Get("/", wordHandlerForRoute.ListWords)
			//     // ... 他のwordルート
			// })
			// reviewHandlerForRoute := handlers.NewReviewHandler(nil)
			// r.Route("/reviews", func(r chi.Router){
			//     // ... reviewルート
			// })
		})
	})

	// --- テストの実行 ---
	log.Println("Running handler tests...")
	exitCode := m.Run() // パッケージ内の全テストを実行

	// --- ティアダウン ---
	log.Println("Tearing down handlers test environment...")
	sqlDB, err = testDB.DB() // 再度取得
	if err == nil {
		err = sqlDB.Close()
		if err == nil {
			log.Println("Test database connection closed.")
		} else {
			log.Printf("Error closing test database connection: %v", err)
		}
	} else {
		log.Printf("Error getting DB for closing: %v", err)
	}

	os.Exit(exitCode) // テスト結果をOSに返す
}

// --- テストヘルパー関数 (パッケージ内で共有) ---

// clearTables はテスト前にテーブルをクリーンアップします
func clearTables(t *testing.T) {
	// t.Helper() // これを追加するとテストヘルパーとして認識される
	if testDB == nil {
		t.Fatal("clearTables called before testDB was initialized")
	}
	// 外部キー制約のため、依存される側から削除
	if err := testDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.LearningProgress{}).Error; err != nil {
		t.Fatalf("Failed to clear learning_progress table: %v", err)
	}
	if err := testDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Word{}).Error; err != nil {
		t.Fatalf("Failed to clear words table: %v", err)
	}
	// テナントは各テストで作成・利用する想定なので、ここでは削除しない
	// log.Println("Test tables cleared.")
}

// createTestTenant はテスト用のテナントを作成するヘルパー関数
func createTestTenant(t *testing.T) *model.Tenant {
	// t.Helper()
	if testDB == nil {
		t.Fatal("createTestTenant called before testDB was initialized")
	}
	tenantRepo := repository.NewGormTenantRepository()
	tenantService := service.NewTenantService(testDB, tenantRepo) // TestMain で初期化された testDB を使用
	tenantName := fmt.Sprintf("TestTenant_%s", uuid.New().String())
	tenant, err := tenantService.CreateTenant(context.Background(), tenantName)
	if err != nil {
		t.Fatalf("Failed to create test tenant '%s' for test %s: %v", tenantName, t.Name(), err)
	}
	return tenant
}

// executeRequest はテスト用のHTTPリクエストを実行し、レスポンスレコーダーを返します。
// TestMain で初期化された共通ルーター (testRouter) を使用します。
func executeRequest(req *http.Request) *httptest.ResponseRecorder {
	if testRouter == nil {
		// TestMain が正常に実行されなかった場合に備える
		log.Panic("executeRequest called before testRouter was initialized")
	}
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	return rr
}

// createRequest はテスト用のHTTPリクエストオブジェクトを作成します。
// tenantIDが指定されていれば X-Tenant-ID ヘッダーを追加します。
func createRequest(t *testing.T, method, url string, body interface{}, tenantID *uuid.UUID) *http.Request {
	// t.Helper()
	var reqBodyBytes []byte
	var err error

	if body != nil {
		switch b := body.(type) {
		case string:
			reqBodyBytes = []byte(b)
		case []byte:
			reqBodyBytes = b
		default:
			reqBodyBytes, err = json.Marshal(body)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}
		}
	}

	var bodyReader *bytes.Buffer
	if reqBodyBytes != nil {
		bodyReader = bytes.NewBuffer(reqBodyBytes)
	} else {
		bodyReader = bytes.NewBuffer([]byte{}) // ボディがない場合も空のバッファ
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	if body != nil {
		// ボディがある場合は Content-Type を設定
		req.Header.Set("Content-Type", "application/json")
	}

	if tenantID != nil {
		req.Header.Set("X-Tenant-ID", tenantID.String())
	}
	return req
}
