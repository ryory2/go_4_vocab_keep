package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// responseDetailRecorder は http.ResponseWriter をラップし、
// ステータスコードとレスポンスボディ、ヘッダーを記録・アクセス可能にします。
type responseDetailRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// newResponseDetailRecorder は新しい responseDetailRecorder を作成します。
func newResponseDetailRecorder(w http.ResponseWriter) *responseDetailRecorder {
	return &responseDetailRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // デフォルトのステータスコード
		body:           new(bytes.Buffer),
	}
}

// Header は元の ResponseWriter の Header を返します。
// これにより、ハンドラが設定したヘッダーを後でミドルウェアが読み取れます。
func (rdr *responseDetailRecorder) Header() http.Header {
	return rdr.ResponseWriter.Header()
}

// WriteHeader はステータスコードを記録し、元の ResponseWriter の WriteHeader を呼び出します。
func (rdr *responseDetailRecorder) WriteHeader(statusCode int) {
	rdr.statusCode = statusCode
	rdr.ResponseWriter.WriteHeader(statusCode)
}

// Write はレスポンスボディをバッファに書き込み、元の ResponseWriter の Write を呼び出します。
// 書き込まれたバイト数とエラーを返します。
func (rdr *responseDetailRecorder) Write(b []byte) (int, error) {
	n, err := rdr.ResponseWriter.Write(b)
	if err == nil {
		// Writeが成功した場合、書き込まれたバイト数分をバッファに保存
		if n > 0 {
			rdr.body.Write(b[:n])
		}
	}
	return n, err
}

// Flush は、ラップされたResponseWriterがhttp.Flusherを実装している場合にFlushを呼び出します。
func (rdr *responseDetailRecorder) Flush() {
	if flusher, ok := rdr.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// ResponseDetailLoggingMiddleware はレスポンスのヘッダーとボディ詳細をログに出力するミドルウェアです。
func ResponseDetailLoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := chimiddleware.GetReqID(r.Context())
			if requestID == "" {
				requestID = "N/A" // フォールバック
			}

			recorder := newResponseDetailRecorder(w)

			next.ServeHTTP(recorder, r) // ハンドラを実行し、レスポンスをrecorderに記録

			// --- ログ属性の準備 ---
			logAttrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("type", "response_detail_log"),
				slog.Int("status_code", recorder.statusCode),
			}

			// --- レスポンスヘッダーのログ出力 ---
			// `formatHeadersForDetailLog` と `sensitiveHeadersForDetailLog` は、
			// `RequestDetailLoggingMiddleware` が定義されているファイル（例: internal/middleware/middleware.go や
			// internal/middleware/request_detail_logger.go）で定義されているものを利用することを想定しています。
			// `sensitiveHeadersForDetailLog` に "set-cookie": true などを追加して、
			// レスポンスヘッダーのセンシティブ情報をマスキングしてください。
			responseHeadersMap := formatHeadersForDetailLog(recorder.Header())
			if len(responseHeadersMap) > 0 {
				headerLogAttrs := make([]interface{}, 0, len(responseHeadersMap)*2)
				for k, v := range responseHeadersMap {
					headerLogAttrs = append(headerLogAttrs, slog.String(k, v))
				}
				logAttrs = append(logAttrs, slog.Group("response_headers", headerLogAttrs...))
			}

			// --- レスポンスボディのログ出力 ---
			responseBodyBytes := recorder.body.Bytes()
			if len(responseBodyBytes) > 0 {
				contentType := recorder.Header().Get("Content-Type")
				if contentType == "" {
					// Content-Typeが設定されていない場合、net/httpが推測したものが使われることがあるが、
					// ここでは主にハンドラが明示的に設定したものを期待。
					// Writeが呼ばれた後なら、デフォルトで http.DetectContentType が使われる。
					contentType = http.DetectContentType(responseBodyBytes)
				}

				// ボディサイズのログ出力制限 (設定ファイルから読み込めるようにするとより柔軟)
				const maxLogBodySizeBytes = 2048 // 例: 2KBまで詳細ログ（JSONパース対象やテキスト表示）

				if strings.HasPrefix(contentType, "application/json") {
					if len(responseBodyBytes) > maxLogBodySizeBytes {
						logAttrs = append(logAttrs, slog.String("response_body_info",
							fmt.Sprintf("[JSON response body too large to log in detail: %d bytes, limit: %d bytes, Content-Type: %s]",
								len(responseBodyBytes), maxLogBodySizeBytes, contentType)))
					} else {
						var jsonData interface{} // map[string]interface{} や []interface{} を想定
						if err := json.Unmarshal(responseBodyBytes, &jsonData); err == nil {
							// ★★★ 重要 ★★★
							// レスポンスボディのJSONデータ (`jsonData`) に含まれる可能性のある
							// 機密情報 (個人情報、アクセストークンなど) は、ログに出力する前に
							// 必ずマスキング処理を行ってください。
							// 例: func maskSensitiveResponseJSONFields(data interface{}) interface{} を定義し適用。
							logAttrs = append(logAttrs, slog.Any("response_body", jsonData))
						} else {
							errorMsg := fmt.Sprintf("failed to unmarshal JSON response body: %v", err)
							logAttrs = append(logAttrs, slog.String("response_body_error", errorMsg))
							// パースできなかったJSON（またはその一部）を文字列としてログることも検討
							rawBodyStr := string(responseBodyBytes)
							if len(rawBodyStr) > maxLogBodySizeBytes/2 { // エラー時でもログサイズを考慮
								logAttrs = append(logAttrs, slog.String("response_body_raw", rawBodyStr[:maxLogBodySizeBytes/2]+"... (truncated)"))
							} else {
								logAttrs = append(logAttrs, slog.String("response_body_raw", rawBodyStr))
							}
						}
					}
				} else if strings.HasPrefix(contentType, "text/") {
					bodyStr := string(responseBodyBytes)
					if len(bodyStr) > maxLogBodySizeBytes {
						logAttrs = append(logAttrs, slog.String("response_body", bodyStr[:maxLogBodySizeBytes]+"... (truncated)"))
					} else {
						logAttrs = append(logAttrs, slog.String("response_body", bodyStr))
					}
				} else {
					// JSONでもTextでもない場合 (例: application/octet-stream, image/*)
					// バイナリデータなどをそのままログに出力するのは避ける
					logAttrs = append(logAttrs, slog.String("response_body_info",
						fmt.Sprintf("[Non-JSON/text response body: %d bytes, Content-Type: %s. Body not logged in detail.]",
							len(responseBodyBytes), contentType)))
				}
			} else {
				logAttrs = append(logAttrs, slog.String("response_body_info", "(empty body)"))
			}

			// --- ログレベルの決定 ---
			// RequestDetailLoggingMiddlewareと同様のロジックを想定
			logLevel := slog.LevelDebug // デフォルトはDebugレベルで詳細ログを出力
			if recorder.statusCode >= 500 {
				logLevel = slog.LevelError
			} else if recorder.statusCode >= 400 {
				logLevel = slog.LevelWarn
			}
			// 環境や設定に応じて、常にDEBUGレベルではなく、INFOレベルにするなどの調整も可能

			logger.LogAttrs(r.Context(), logLevel, "HTTP response detail", logAttrs...)
		})
	}
}

// --- `formatHeadersForDetailLog` と `sensitiveHeadersForDetailLog` について ---
// 以下のヘルパー関数および変数は、既存の `RequestDetailLoggingMiddleware` が
// 定義されているファイル（例: `internal/middleware/middleware.go` や
// `internal/middleware/request_detail_logger.go`）に存在することを想定しています。
//
// `sensitiveHeadersForDetailLog` マップに、レスポンスヘッダーでセンシティブなもの
// (特に "set-cookie") を追加してください。
//
// 例 (RequestDetailLoggingMiddleware があるファイルに追記・修正):
/*
package middleware

import (
	"net/http"
	"strings"
)

// sensitiveHeadersForDetailLog はログ出力時に値をマスキングするヘッダー名のリスト (小文字で定義)
// ★★★ このマップに "set-cookie": true を追加してください ★★★
var sensitiveHeadersForDetailLog = map[string]bool{
	"authorization": true,
	"cookie":        true, // リクエストCookie用
	"set-cookie":    true, // ★レスポンスSet-Cookie用に追加★
	"x-api-key":     true,
	"x-csrf-token":  true,
	// 必要に応じて他のセンシティブなリクエスト/レスポンスヘッダーを追加
}

// formatHeadersForDetailLog はヘッダー情報をログ出力用に整形・マスキングするヘルパー関数
// (この関数は既存のものをそのまま利用できます)
func formatHeadersForDetailLog(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		// ヘッダーキーをslogで扱いやすいスネークケースに変換 (オプション)
		// logKey := strings.ReplaceAll(lowerKey, "-", "_")
		logKey := lowerKey // またはそのままのキー名
		if sensitiveHeadersForDetailLog[lowerKey] {
			result[logKey] = "[SENSITIVE_VALUE_MASKED]"
		} else {
			result[logKey] = strings.Join(values, ", ")
		}
	}
	return result
}
*/
