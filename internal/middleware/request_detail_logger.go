package middleware

import (
	"bytes"
	// "context" // chimiddleware.GetReqID を使うため、直接的な context.WithValue は不要
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net/http"
	"strings"

	// "github.com/google/uuid" // 通常はChiがIDを生成・提供するため不要
	chimiddleware "github.com/go-chi/chi/v5/middleware" // Chiのミドルウェアパッケージをインポート
)

// sensitiveHeadersForDetailLog はログ出力時に値をマスキングするヘッダー名のリスト (小文字で定義)
var sensitiveHeadersForDetailLog = map[string]bool{
	"authorization": true,
	"cookie":        true,
	"x-api-key":     true,
	"x-csrf-token":  true,
	// 必要に応じて他のセンシティブなヘッダーを追加
}

// loggingDetailResponseWriter はステータスコードを保持するためのラッパー
type loggingDetailResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingDetailResponseWriter(w http.ResponseWriter) *loggingDetailResponseWriter {
	return &loggingDetailResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (lrw *loggingDetailResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// RequestDetailLoggingMiddleware はリクエストヘッダーとBody（エラー時または設定による）をログに出力するミドルウェア
func RequestDetailLoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// startTime := time.Now()

			// Chiのヘルパー関数を使ってリクエストIDを取得
			requestID := chimiddleware.GetReqID(r.Context())
			// もしGetReqIDが空文字列を返した場合のフォールバック (通常はChiがIDを保証)
			// if requestID == "" {
			// 	requestID = uuid.New().String() // 最後の手段
			// }

			// リクエストBodyの読み取り、バッファリング、整形
			var requestBodyBytes []byte
			var requestBodyForLog interface{}

			if r.Body != nil && r.ContentLength > 0 {
				requestBodyBytes, err := ioutil.ReadAll(r.Body)
				if err != nil {
					// Body読み取りエラー時の処理 (ログなど)
					logger.ErrorContext(r.Context(), "Failed to read request body in middleware", slog.Any("error", err), slog.String("request_id", requestID))
					// エラーが発生しても処理を続行する（Bodyなしとして扱う）か、エラーレスポンスを返すかなど、ポリシーによる
					// ここではBodyなしとして続行
				} else {
					r.Body = ioutil.NopCloser(bytes.NewBuffer(requestBodyBytes)) // 再セット

					contentType := r.Header.Get("Content-Type")
					if strings.HasPrefix(contentType, "application/json") && len(requestBodyBytes) > 0 {
						var jsonData map[string]interface{}
						if err := json.Unmarshal(requestBodyBytes, &jsonData); err == nil {
							// ★★★ JSON Body内のセンシティブ情報をマスキング ★★★
							// この部分はアプリケーションのデータ構造に合わせて具体的に実装が必要です。
							// 例: maskSensitiveJSONFields(jsonData) のようなヘルパー関数を呼び出す
							// if _, ok := jsonData["password"]; ok { jsonData["password"] = "[MASKED]" }
							requestBodyForLog = jsonData
						} else {
							requestBodyForLog = fmt.Sprintf("[Unparseable JSON body: %d bytes, error: %v]", len(requestBodyBytes), err)
						}
					} else if len(requestBodyBytes) > 0 {
						requestBodyForLog = fmt.Sprintf("[Non-JSON body: %d bytes, Content-Type: %s]", len(requestBodyBytes), contentType)
					}
				}
			}

			lrw := newLoggingDetailResponseWriter(w)
			next.ServeHTTP(lrw, r) // ハンドラー実行

			// duration := time.Since(startTime)

			// ログ属性の準備
			logAttrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("type", "request_detail_log"),
				// slog.Time("timestamp", startTime),
				// slog.String("remote_ip", r.RemoteAddr),
				slog.String("method", r.Method),
				slog.String("uri", r.RequestURI),
				slog.String("proto", r.Proto),
				slog.Int("status_code", lrw.statusCode),
				// slog.Float64("latency_ms", float64(duration.Nanoseconds())/1e6),
				// slog.String("user_agent", r.UserAgent()),
			}

			// リクエストヘッダーの追加 (マスキング処理込み)
			formattedHeaders := formatHeadersForDetailLog(r.Header)
			if len(formattedHeaders) > 0 {
				// slog.Groupを使ってヘッダーを構造化
				headerAttrs := make([]interface{}, 0, len(formattedHeaders)*2)
				for k, v := range formattedHeaders {
					headerAttrs = append(headerAttrs, slog.String(k, v))
				}
				logAttrs = append(logAttrs, slog.Group("request_headers", headerAttrs...))
			}

			// リクエストBodyの追加 (エラー時または設定に応じて)
			// TODO: 設定ファイルからBodyを常に出力するかどうかを制御できるようにすると良い
			shouldLogBody := lrw.statusCode >= 400 // エラー時のみログ出力するポリシー
			if shouldLogBody && requestBodyForLog != nil {
				logAttrs = append(logAttrs, slog.Any("request_body", requestBodyForLog))
			} else if shouldLogBody && len(requestBodyBytes) > 0 {
				logAttrs = append(logAttrs, slog.String("request_body_info", fmt.Sprintf("[Raw body not logged in detail: %d bytes, Content-Type: %s]", len(requestBodyBytes), r.Header.Get("Content-Type"))))
			}

			// ログレベルの決定
			logLevel := slog.LevelDebug // デフォルトはDebug
			if lrw.statusCode >= 500 {
				logLevel = slog.LevelError
			} else if lrw.statusCode >= 400 {
				logLevel = slog.LevelWarn
			}

			logger.LogAttrs(r.Context(), logLevel, "HTTP request detail", logAttrs...)
		})
	}
}

// formatHeadersForDetailLog はヘッダー情報をログ出力用に整形・マスキングするヘルパー関数
func formatHeadersForDetailLog(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		logKey := strings.ReplaceAll(lowerKey, "-", "_") // ヘッダーキーをスネークケースに
		if sensitiveHeadersForDetailLog[lowerKey] {
			result[logKey] = "[SENSITIVE_VALUE_MASKED]"
		} else {
			result[logKey] = strings.Join(values, ", ")
		}
	}
	return result
}

// (オプション) JSON Body内のセンシティブ情報をマスキングするヘルパー関数の例
// func maskSensitiveJSONFields(data map[string]interface{}) {
//    if pass, ok := data["password"].(string); ok && pass != "" {
//        data["password"] = "[MASKED_PASSWORD]"
//    }
//    // 他のセンシティブなフィールドも同様に処理
//    if userDetails, ok := data["user_details"].(map[string]interface{}); ok {
//        if ssn, ok := userDetails["ssn"].(string); ok && ssn != "" {
//            userDetails["ssn"] = "[MASKED_SSN]"
//        }
//    }
// }
