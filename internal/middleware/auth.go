package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go_4_vocab_keep/internal/config" // ★ config パッケージをインポート
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/webutil"

	"github.com/golang-jwt/jwt/v5" // ★ JWTライブラリをインポート
	"github.com/google/uuid"
)

// ★★★ ここからが新しいJWT認証ミドルウェア ★★★

// JWTAuthMiddleware は Authorization ヘッダーの Bearer トークンを検証するミドルウェア
func JWTAuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := GetLogger(r.Context())

			// 1. Authorization ヘッダーからトークンを取得
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn("JWT auth failed: Authorization header missing")
				appErr := model.NewAppError("UNAUTHORIZED", "Authorizationヘッダーが必要です。", "", model.ErrForbidden)
				webutil.HandleError(w, logger, appErr)
				return
			}

			// "Bearer {token}" の形式を検証
			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) != 2 || strings.ToLower(headerParts[0]) != "bearer" {
				logger.Warn("JWT auth failed: Invalid Authorization header format")
				appErr := model.NewAppError("UNAUTHORIZED", "Authorizationヘッダーの形式が正しくありません。", "", model.ErrForbidden)
				webutil.HandleError(w, logger, appErr)
				return
			}
			tokenString := headerParts[1]

			// 2. JWTをパースし、署名と有効期限を検証
			// jwt.Parse は署名と有効期限(exp)の両方を検証してくれる
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// 署名アルゴリズムが期待通り(HS256)かチェック
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, model.NewAppError("UNEXPECTED_SIGNING_METHOD", "予期しない署名アルゴリズです。", "", errors.New("unexpected signing method"))
				}
				// 設定ファイルから署名キーを返す
				return []byte(cfg.JWT.SecretKey), nil
			})

			if err != nil {
				logger.Warn("JWT auth failed: Invalid token", "error", err)
				// jwt.ErrTokenExpired などの具体的なエラーに応じてメッセージを変えることも可能
				appErr := model.NewAppError("INVALID_TOKEN", "トークンが無効です。", "", model.ErrForbidden)
				webutil.HandleError(w, logger, appErr)
				return
			}

			// 3. トークンが有効で、クレームが取得可能な場合
			if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				// 4. ペイロードから subject (ユーザーID/テナントID) を取得
				subject, err := claims.GetSubject()
				if err != nil {
					logger.Warn("JWT auth failed: Subject (sub) claim missing", "error", err)
					appErr := model.NewAppError("INVALID_TOKEN", "トークンにユーザー情報が含まれていません。", "", model.ErrForbidden)
					webutil.HandleError(w, logger, appErr)
					return
				}

				userID, err := uuid.Parse(subject)
				if err != nil {
					logger.Warn("JWT auth failed: Invalid subject (sub) format", "subject", subject, "error", err)
					appErr := model.NewAppError("INVALID_TOKEN", "トークンのユーザー情報が不正です。", "", model.ErrForbidden)
					webutil.HandleError(w, logger, appErr)
					return
				}

				// ★ リクエストコンテキストにユーザーIDをセット
				// これまでの TenantIDKey とは別に UserIDKey を使うと、責務が明確になる
				ctx := context.WithValue(r.Context(), model.TenantIDKey, userID)

				// 成功。次のハンドラに処理を渡す
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				// このルートを通ることは稀だが、念のため
				logger.Warn("JWT auth failed: Unknown claims type or invalid token")
				appErr := model.NewAppError("INVALID_TOKEN", "トークンが無効です。", "", model.ErrForbidden)
				webutil.HandleError(w, logger, appErr)
			}
		})
	}
}

func GetTenantIDFromContext(ctx context.Context) (uuid.UUID, error) {
	// ★ UserIDKey を使って値を取得する
	value, ok := ctx.Value(model.TenantIDKey).(uuid.UUID)
	if !ok {
		// コンテキストにユーザーIDが見つからない（ミドルウェアが正しく動作していない等の内部エラー）
		return uuid.Nil, model.NewAppError("INTERNAL_SERVER_ERROR", "コンテキストからユーザー情報を取得できませんでした。", "", model.ErrInternalServer)
	}
	return value, nil
}
