package main

import (
	"context"
	"errors"
	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/handlers"
	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/repository"
	"go_4_vocab_keep/internal/service"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/lmittmann/tint"
	"github.com/rs/cors"
)

func main() {
	if err := config.LoadConfig("./configs"); err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// --- ロガー初期化 ---
	logLevel := new(slog.LevelVar)
	switch strings.ToLower(config.Cfg.Log.Level) {
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "info":
		logLevel.Set(slog.LevelInfo)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}

	var logHandler slog.Handler
	appEnv := os.Getenv("APP_ENV")
	if strings.ToLower(config.Cfg.Log.Format) == "text" {
		noColor := (appEnv != "dev" && appEnv != "local")
		tintOpts := &tint.Options{Level: logLevel, TimeFormat: time.RFC3339, AddSource: true, NoColor: noColor}
		logHandler = tint.NewHandler(os.Stdout, tintOpts)
	} else {
		logHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel, AddSource: true})
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	slog.Info("Application starting...", "env", os.Getenv("APP_ENV"))

	db, err := repository.NewDB(config.Cfg.Database.URL, logger)
	if err != nil {
		slog.Error("Error initializing database", "error", err)
		os.Exit(1)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// Dependency Injection
	tenantRepo := repository.NewGormTenantRepository()
	identityRepo := repository.NewGormIdentityRepository()
	wordRepo := repository.NewGormWordRepository()
	progressRepo := repository.NewGormProgressRepository()
	tokenRepo := repository.NewGormTokenRepository()

	mailer := service.NewMailer(&config.Cfg)

	wordService := service.NewWordService(db, wordRepo, progressRepo)
	reviewService := service.NewReviewService(db, progressRepo, &config.Cfg)
	authService := service.NewAuthService(db, tenantRepo, identityRepo, tokenRepo, mailer, &config.Cfg)

	wordHandler := handlers.NewWordHandler(wordService)
	reviewHandler := handlers.NewReviewHandler(reviewService)
	authHandler := handlers.NewAuthHandler(authService)

	r := chi.NewRouter()

	// Middleware
	// 基本的なアクセスログ (メソッド、パス、ステータス等)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.LoggingMiddleware(logger))

	// CORS 設定と適用 (設定ファイルから読み込んだ値を使用)
	corsOptions := cors.Options{
		AllowedOrigins:   config.Cfg.CORS.AllowedOrigins,
		AllowedMethods:   config.Cfg.CORS.AllowedMethods,
		AllowedHeaders:   config.Cfg.CORS.AllowedHeaders,
		ExposedHeaders:   config.Cfg.CORS.ExposedHeaders,
		AllowCredentials: config.Cfg.CORS.AllowCredentials,
		MaxAge:           config.Cfg.CORS.MaxAge,
		Debug:            false,
	}
	corsHandler := cors.New(corsOptions)
	r.Use(corsHandler.Handler)

	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// API Routes
	r.Route("/api/v1", func(r chi.Router) {
		// 認証不要
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Get("/verify-email", authHandler.VerifyAccount)
		r.Post("/forgot-password", authHandler.RequestPasswordReset)
		r.Post("/reset-password", authHandler.ResetPassword)
		r.Post("/auth/google/callback", authHandler.HandleGoogleLogin)

		// 要認証
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuthMiddleware(&config.Cfg))

			// 認証
			r.Route("/auth", func(r chi.Router) {
				r.Get("/me", authHandler.GetMe)
			})

			// 単語
			r.Route("/words", func(r chi.Router) {
				r.Post("/", wordHandler.PostWord)
				r.Get("/", wordHandler.GetWords)
				r.Get("/{word_id}", wordHandler.GetWord)
				r.Put("/{word_id}", wordHandler.PutWord)
				r.Patch("/{word_id}", wordHandler.PatchWord)
				r.Delete("/{word_id}", wordHandler.DeleteWord)
			})

			// 復習
			r.Route("/reviews", func(r chi.Router) {
				r.Get("/", reviewHandler.GetReviewWords)
				r.Get("/summary", reviewHandler.GetReviewSummary)
				r.Put("/{word_id}/result", reviewHandler.UpsertLearningProgressBasedOnReview)
			})
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
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", slog.Any("error", err))
	}

	slog.Info("Server exiting")
}
