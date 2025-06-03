// internal/middleware/logger.go
package middleware

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5/middleware" // chiのミドルウェアヘルパーを使う
)

func NewStructuredLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	// ホスト名を取得 - これにより、ログにホスト名を含める
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown_host"
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// WrapResponseWriter を使ってレスポンス情報を取得
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			// startTime := time.Now()

			requestID := middleware.GetReqID(r.Context()) // リクエストIDを先に取得

			// ロガーに共通属性を追加
			reqLogger := logger.With(
				slog.String("request_id", requestID),
				// slog.String("server_hostname", hostname),
			)

			// リクエスト完了後にログを出力
			defer func() {
				status := ww.Status()
				// latency := time.Since(startTime)
				// latencyMs := latency.Milliseconds() // (追加) ミリ秒単位の整数値

				// レベルを選択 (例: 5xxエラーはError、4xxはWarn、それ以外はInfo)
				logLevel := slog.LevelInfo
				if status >= http.StatusInternalServerError { // 500系
					logLevel = slog.LevelError
				} else if status >= http.StatusBadRequest { // 400系
					logLevel = slog.LevelWarn
				}

				attrs := []slog.Attr{
					// --- リクエスト基本情報 ---
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("query_params", r.URL.RawQuery),

					// --- レスポンス情報 ---
					slog.Int("status", status),
					// slog.Int("response_size_bytes", ww.BytesWritten()), // (変更) bytes_out から変更

					// --- パフォーマンス ---
					// slog.Int64("duration_ms", latencyMs),            // (変更) ミリ秒単位の数値 (推奨)
					// slog.String("duration_human", latency.String()), // (変更なし) 継続 (オプション)

					// --- クライアント情報 ---
					// slog.String("remote_addr", r.RemoteAddr), // (変更) remote_ip から変更, X-Forwarded-For等の考慮が必要
					// slog.String("user_agent", r.UserAgent()), // (変更) コメント解除
					// slog.String("referer", r.Referer()),      // (追加)

					// --- (オプション) HTTPヘッダー (機密情報フィルタリング必須) ---
					// slog.Any("request_headers", filterHeaders(r.Header, []string{"Authorization", "Cookie", "X-Api-Key"})),
				}

				// リクエストスコープのロガーを使ってログ出力
				reqLogger.LogAttrs(r.Context(), logLevel, "HTTP request processed", attrs...)
			}()

			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}
