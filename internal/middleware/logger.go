// internal/middleware/logger.go
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware" // chiのミドルウェアヘルパーを使う
)

func NewStructuredLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// WrapResponseWriter を使ってレスポンス情報を取得
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			requestID := middleware.GetReqID(r.Context()) // リクエストIDを先に取得

			// リクエストスコープのロガーを作成 (リクエストIDなどの共通属性を追加)
			reqLogger := logger.With(slog.String("request_id", requestID))
			ctx := r.Context()
			ctx = context.WithValue(ctx, middleware.RequestIDKey, requestID) // リクエストIDをコンテキストに追加

			// リクエスト完了後にログを出力
			defer func() {
				// レベルを選択 (例: 5xxエラーはError、4xxはWarn、それ以外はInfo)
				level := slog.LevelInfo
				if ww.Status() >= 500 {
					level = slog.LevelError
				} else if ww.Status() >= 400 {
					level = slog.LevelWarn
				}

				// ログに出力する属性
				latency := time.Since(t1) // レイテンシ計算
				attrs := []slog.Attr{
					// slog.String("remote_ip", r.RemoteAddr),
					// slog.String("user_agent", r.UserAgent()),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", ww.Status()),
					slog.Int("bytes_out", ww.BytesWritten()),
					slog.Duration("latency_ms", latency),           // レイテンシの単位を明示 (例: ms)
					slog.String("latency_human", latency.String()), // 人が読みやすい形式も追加（オプション）
				}

				// リクエストスコープのロガーを使ってログ出力
				reqLogger.LogAttrs(ctx, level, "Request completed", attrs...)
			}()

			next.ServeHTTP(ww, r.WithContext(ctx)) // 更新されたコンテキストを持つリクエストを渡す
		}
		return http.HandlerFunc(fn)
	}
}
