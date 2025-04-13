// cmd/myapp/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"go_1_test_repository/internal/config"     // プロジェクト名修正
	"go_1_test_repository/internal/handlers"   // プロジェクト名修正
	"go_1_test_repository/internal/middleware" // プロジェクト名修正
	"go_1_test_repository/internal/repository" // プロジェクト名修正
	"go_1_test_repository/internal/service"    // プロジェクト名修正

	"gorm.io/gorm" // GORMはDB接続用に必要
)

func main() {
	log.Println("Starting application...")

	// 1. Load Configuration
	if err := config.LoadConfig("configs"); err != nil { // "configs" ディレクトリを指定
		log.Fatalf("Error loading configuration: %v", err)
	}

	// 2. Initialize Database Connection (GORM)
	db, err := repository.NewDB(config.Cfg.Database.URL)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Error getting underlying sql.DB from GORM: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		} else {
			log.Println("Database connection closed.")
		}
	}()

	// 3. Dependency Injection
	tenantRepo := repository.NewGormTenantRepository()
	wordRepo := repository.NewGormWordRepository()
	progressRepo := repository.NewGormProgressRepository()

	tenantService := service.NewTenantService(db, tenantRepo)
	wordService := service.NewWordService(db, wordRepo, progressRepo)
	// ReviewServiceのNew関数にdbを渡すように修正 (もし必要ならReviewServiceの実装も確認)
	reviewService := service.NewReviewService(db, progressRepo, config.Cfg)

	// Authenticator を作成
	tenantAuthenticator := middleware.NewServiceTenantAuthenticator(tenantService)

	tenantHandler := handlers.NewTenantHandler(tenantService)
	wordHandler := handlers.NewWordHandler(wordService)
	reviewHandler := handlers.NewReviewHandler(reviewService)

	// 4. Setup Router
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger) // Use chi's structured logger
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// API Routes
	r.Route("/api/v1", func(r chi.Router) {
		// --- Public routes ---
		r.Post("/tenants", tenantHandler.CreateTenant) // テナント作成 (認証不要)
		// ここに他の公開API (例: ログイン) があれば追加

		// --- Protected routes (require Tenant ID) ---
		r.Group(func(r chi.Router) {
			// 強化したテナント認証ミドルウェアを適用
			if config.Cfg.Auth.Enabled {
				// 本番モード: DB検証を行う認証ミドルウェアを適用
				log.Println("Applying production authentication middleware")
				r.Use(middleware.TenantAuthMiddleware(tenantAuthenticator))
			} else {
				// 開発モード: DB検証を行わない簡易ミドルウェアを適用
				log.Println("Applying development (no validation) authentication middleware")
				r.Use(middleware.DevTenantContextMiddleware)
			}

			// Word routes
			r.Route("/words", func(r chi.Router) {
				r.Post("/", wordHandler.CreateWord)
				r.Get("/", wordHandler.ListWords)
				r.Get("/{word_id}", wordHandler.GetWord)
				r.Put("/{word_id}", wordHandler.UpdateWord)
				r.Delete("/{word_id}", wordHandler.DeleteWord)
			})

			// Review routes
			r.Route("/reviews", func(r chi.Router) {
				r.Get("/", reviewHandler.GetReviewWords)
				r.Post("/{word_id}/result", reviewHandler.SubmitReviewResult)
			})
			// ここに他の認証が必要なエンドポイントを追加
		})
	})

	// Health Check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		// DB接続チェック
		sqlDB, err := db.DB()
		if err != nil {
			log.Printf("Health check failed: could not get DB object: %v", err)
			http.Error(w, "Health check failed", http.StatusInternalServerError)
			return
		}
		err = sqlDB.PingContext(r.Context())
		if err != nil {
			log.Printf("Health check failed: could not ping DB: %v", err)
			http.Error(w, "Health check failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 5. Start Server
	server := &http.Server{
		Addr:         config.Cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Server listening on port %s", config.Cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %s: %v\n", config.Cfg.Server.Port, err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

// GORMのDB接続を取得するヘルパー (main外に置いても良い)
func getDBFromRequest(r *http.Request) *gorm.DB {
	// ここでは単純化のため、グローバルなDB接続を使うことを想定していますが、
	// 本来はリクエストごとにDBセッションを取得するか、
	// ミドルウェアでコンテキストにDB接続をセットするのがより良い方法です。
	// このサンプルコードでは、DIされたDBインスタンスを直接使います。
	// ただし、main関数外から直接db変数にアクセスできないため、
	// 実際には引数で渡すか、別の方法で共有する必要があります。
	// ここでは concept を示すため不完全な形で記述します。
	// panic("getDBFromRequest needs proper implementation to access the db instance")
	return nil // 不完全な実装 - 実際には main の db を渡す必要がある
}
