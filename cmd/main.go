// cmd/myapp/main.go
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lmittmann/tint"
	"github.com/rs/cors"

	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/handlers"
	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/repository"
	"go_4_vocab_keep/internal/service"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"gorm.io/gorm" // GORMはDB接続用
)

func main() {
	//　設定ファイル読み込み用の一時的なロガー設定
	tempLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(tempLogger)
	log.Println("Log Config Loading...")

	// Configを読み込み
	if err := config.LoadConfig("../configs"); err != nil { // "configs" ディレクトリを指定
		slog.Error("Error loading configuration", slog.Any("error", err))
		os.Exit(1) // Fatalf の代わりに Error + os.Exit
	}

	// === 設定に基づいて slog ロガーを初期化 ===
	logLevel := new(slog.LevelVar) // 動的に変更可能なレベル変数
	// config.yamlで設定したログレベルを設定
	switch strings.ToLower(config.Cfg.Log.Level) {
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "info":
		logLevel.Set(slog.LevelInfo)
	case "warn", "warning":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo) // 不明な場合はInfo
		slog.Warn("Unknown log level specified in config, defaulting to INFO", slog.String("level", config.Cfg.Log.Level))
	}
	// config.yamlで設定したログフォーマットを設定
	var handler slog.Handler
	appEnv := os.Getenv("APP_ENV")
	if strings.ToLower(appEnv) == "dev" {
		// --- tint Handler を使用 ---
		tintOpts := &tint.Options{
			Level:      logLevel,
			TimeFormat: time.RFC3339, // 2025-06-04T02:05:41+09:00
			// AddSource: true, // ソース情報を追加する。（2025-06-04T02:05:41+09:00 INF cmd/main.go:212 Server listening port=:8080）
		}
		handler = tint.NewHandler(os.Stderr, tintOpts)
		tempLogger.Info("Using TINT log handler", slog.String("APP_ENV", appEnv))
	} else {
		jsonOpts := &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: true,
		}
		handler = slog.NewJSONHandler(os.Stderr, jsonOpts)
		tempLogger.Info("Using JSON log handler", slog.String("APP_ENV", appEnv))
	}
	logger := slog.New(handler)
	log.Println("Log Config Loaded...")

	// Configファイルの読み込み完了後、アプリケーション全体のデフォルトロガーを設定
	slog.SetDefault(logger)

	slog.Info("Application starting...")

	// 2. Initialize Database Connection (GORM)
	db, err := repository.NewDB(config.Cfg.Database.URL, logger)
	if err != nil {
		slog.Error("Error initializing database", slog.Any("error", err))
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("Error getting underlying sql.DB from GORM", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			slog.Error("Error closing database connection", slog.Any("error", err))
		} else {
			slog.Info("Database connection closed.")
		}
	}()

	// 3. Dependency Injection
	tenantRepo := repository.NewGormTenantRepository(logger)
	wordRepo := repository.NewGormWordRepository(logger)
	progressRepo := repository.NewGormProgressRepository(logger)

	tenantService := service.NewTenantService(db, tenantRepo, logger)
	wordService := service.NewWordService(db, wordRepo, progressRepo, logger)
	// ReviewServiceのNew関数にdbを渡すように修正 (もし必要ならReviewServiceの実装も確認)
	reviewService := service.NewReviewService(db, progressRepo, config.Cfg, logger)

	// Authenticator を作成
	tenantAuthenticator := middleware.NewServiceTenantAuthenticator(tenantService)

	tenantHandler := handlers.NewTenantHandler(tenantService, logger)
	wordHandler := handlers.NewWordHandler(wordService, logger)
	reviewHandler := handlers.NewReviewHandler(reviewService, logger)

	// 4. Setup Router
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.NewStructuredLogger(logger)) // slogを使うカスタムロガーミドルウェア

	// CORS 設定と適用 (設定ファイルから読み込んだ値を使用)
	corsOptions := cors.Options{
		AllowedOrigins:   config.Cfg.CORS.AllowedOrigins,
		AllowedMethods:   config.Cfg.CORS.AllowedMethods,
		AllowedHeaders:   config.Cfg.CORS.AllowedHeaders,
		ExposedHeaders:   config.Cfg.CORS.ExposedHeaders,
		AllowCredentials: config.Cfg.CORS.AllowCredentials,
		MaxAge:           config.Cfg.CORS.MaxAge,
		Debug:            false, // (変更) CORSライブラリのデバッグログを常に無効化
	}
	corsHandler := cors.New(corsOptions)
	r.Use(corsHandler.Handler)

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
			slog.Info("Applying production authentication middleware")
			r.Use(middleware.TenantAuthMiddleware(tenantAuthenticator))

			// Word routes
			r.Route("/words", func(r chi.Router) {
				r.Post("/", wordHandler.PostWord)
				r.Get("/", wordHandler.GetWords)
				r.Get("/{word_id}", wordHandler.GetWord)
				r.Put("/{word_id}", wordHandler.PutWord)
				r.Patch("/{word_id}", wordHandler.PatchWord)
				r.Delete("/{word_id}", wordHandler.DeleteWord)
			})

			// Review routes
			r.Route("/reviews", func(r chi.Router) {
				r.Get("/", reviewHandler.GetReviewWords)
				// UpsertLearningProgressBasedOnReview のルーティング
				// HTTPメソッドは設計によりますが、更新または作成なので PUT または POST が一般的です。
				// URLに word_id が含まれるので、特定のリソースに対する操作として PUT が適切かもしれません。
				r.Put("/{word_id}/result", reviewHandler.UpsertLearningProgressBasedOnReview)
				// もし POST を使う場合:
				// r.Post("/{word_id}/result", reviewHandler.UpsertLearningProgressBasedOnReview)
			})
			// ここに他の認証が必要なエンドポイントを追加
		})
	})

	// Health Check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		// DB接続チェック
		ctx := r.Context()
		sqlDB, err := db.DB()
		if err != nil {
			slog.ErrorContext(ctx, "Health check failed: could not get DB object", slog.Any("error", err))
			http.Error(w, "Health check failed", http.StatusInternalServerError)
			return
		}
		err = sqlDB.PingContext(r.Context())
		if err != nil {
			slog.ErrorContext(ctx, "Health check failed: could not ping DB", slog.Any("error", err))
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
		slog.Info("Server listening", slog.String("port", config.Cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Could not listen on port", slog.String("port", config.Cfg.Server.Port), slog.Any("error", err))
			os.Exit(1) // Listen失敗は致命的
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", slog.Any("error", err))
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
