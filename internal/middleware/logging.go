package middleware

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// logCtxKey はコンテキストにロガーを格納するためのキーです。
type logCtxKey struct{}

// sensitiveHeaders はログ出力時に値をマスキングするヘッダー名のリストです (小文字で定義)。
var sensitiveHeaders = map[string]bool{
	"authorization": true,
	"cookie":        true, // リクエストヘッダー
	"set-cookie":    true, // レスポンスヘッダー
	"x-api-key":     true,
	"x-csrf-token":  true,
}

// responseLogger は http.ResponseWriter をラップし、ステータスコードとレスポンスボディを記録します。
type responseLogger struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// newResponseLogger は新しい responseLogger を作成します。
func newResponseLogger(w http.ResponseWriter) *responseLogger {
	return &responseLogger{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           new(bytes.Buffer),
	}
}

func (rl *responseLogger) WriteHeader(statusCode int) {
	rl.statusCode = statusCode
	rl.ResponseWriter.WriteHeader(statusCode)
}

func (rl *responseLogger) Write(b []byte) (int, error) {
	rl.body.Write(b) // レスポンスボディをキャプチャ
	return rl.ResponseWriter.Write(b)
}

// LoggingMiddleware はリクエスト/レスポンスのログ出力を一元管理するミドルウェアです。
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// --- ステップ1: リクエスト到着時の準備 ---

			startTime := time.Now()

			// リクエストID付きのロガーを生成し、コンテキストに格納
			requestLogger := logger.With("req_id", middleware.GetReqID(r.Context()))
			ctx := context.WithValue(r.Context(), logCtxKey{}, requestLogger)
			r = r.WithContext(ctx)

			// ★★★ 開始ログの出力 ★★★
			requestLogger.Info("Request started",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)

			// リクエストボディを安全に読み取る (デバッグ用)
			var reqBodyBytes []byte
			if logger.Enabled(r.Context(), slog.LevelDebug) && r.Body != nil {
				reqBodyBytes, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
			}

			// レスポンスを記録するためのラッパーを準備
			rl := newResponseLogger(w)

			// --- ステップ2: 次のハンドラに処理を移譲 ---
			next.ServeHTTP(rl, r)

			// --- ステップ3: レスポンス返却直前のログ出力 ---

			latency := time.Since(startTime)
			statusCode := rl.statusCode

			// ログレベルを決定
			logLevel := slog.LevelInfo
			if statusCode >= 500 {
				logLevel = slog.LevelError
			} else if statusCode >= 400 {
				logLevel = slog.LevelWarn
			}

			// ★★★ 終了ログ（概要ログ）の出力 ★★★
			requestLogger.Log(r.Context(), logLevel, "Request completed",
				"status", statusCode,
				"latency_ms", float64(latency.Nanoseconds())/1e6,
				"bytes_out", rl.body.Len(),
			)

			// ★★★ 詳細ログの出力 (デバッグレベル) ★★★
			if logger.Enabled(r.Context(), slog.LevelDebug) {
				requestLogger.Debug("Request detail",
					"headers", formatHeaders(r.Header),
					"body", string(reqBodyBytes),
				)
				requestLogger.Debug("Response detail",
					"status", statusCode, // ステータスは詳細にも含めると便利
					"headers", formatHeaders(rl.Header()),
					"body", rl.body.String(),
				)
			}
		})
	}
}

// GetLogger はコンテキストから slog.Logger を取得します。
func GetLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(logCtxKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// formatHeaders はヘッダー情報をログ出力用に整形・マスキングするヘルパー関数
func formatHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			result[key] = "[SENSITIVE]"
		} else {
			result[key] = strings.Join(values, ", ")
		}
	}
	return result
}
