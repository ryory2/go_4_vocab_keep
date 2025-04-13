// internal/middleware/auth.go
package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"go_4_vocab_keep/internal/model" // プロジェクト名修正
	// プロジェクト名修正
	"go_4_vocab_keep/internal/service" // serviceを追加 (Authenticatorから使う場合)
	"go_4_vocab_keep/internal/webutil" // プロジェクト名修正

	"github.com/google/uuid"
	// gormは直接使わない (service経由にする)
)

// TenantAuthenticator はテナント認証を行うためのインターフェース
type TenantAuthenticator interface {
	Authenticate(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

// serviceTenantAuthenticator は TenantService を使って認証を行う実装
type serviceTenantAuthenticator struct {
	tenantService service.TenantService
}

// NewServiceTenantAuthenticator は新しいオーセンティケータを作成します
func NewServiceTenantAuthenticator(ts service.TenantService) TenantAuthenticator {
	return &serviceTenantAuthenticator{tenantService: ts}
}

// Authenticate はテナントの存在と有効性を確認します
func (a *serviceTenantAuthenticator) Authenticate(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	_, err := a.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return false, nil // 見つからない場合は認証失敗 (エラーではない)
		}
		// その他のエラー (DBエラーなど)
		log.Printf("Error during tenant authentication check for %s: %v", tenantID, err)
		return false, err
	}
	// エラーなく取得できれば有効なテナント
	return true, nil
}

// TenantAuthMiddleware はテナント認証を行うミドルウェアを作成します
func TenantAuthMiddleware(auth TenantAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantIDStr := r.Header.Get("X-Tenant-ID")
			if tenantIDStr == "" {
				log.Println("Authentication failed: X-Tenant-ID header missing")
				webutil.RespondWithError(w, http.StatusUnauthorized, "Unauthorized: Missing X-Tenant-ID header")
				return
			}

			tenantID, err := uuid.Parse(tenantIDStr)
			if err != nil {
				log.Printf("Authentication failed: Invalid X-Tenant-ID format: %s", tenantIDStr)
				webutil.RespondWithError(w, http.StatusUnauthorized, "Unauthorized: Invalid X-Tenant-ID format")
				return
			}

			// --- テナントの存在と有効性をチェック ---
			isValid, err := auth.Authenticate(r.Context(), tenantID)
			if err != nil {
				// 認証処理中のエラー (DBエラーなど)
				webutil.RespondWithError(w, http.StatusInternalServerError, "Authentication check failed")
				return
			}
			if !isValid {
				log.Printf("Forbidden: Tenant ID %s not found or inactive", tenantIDStr)
				webutil.RespondWithError(w, http.StatusForbidden, "Forbidden: Invalid or inactive tenant")
				return
			}
			// --- チェック完了 ---

			// コンテキストにテナントIDをセット
			ctx := context.WithValue(r.Context(), model.TenantIDKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantIDFromContext はコンテキストからテナントIDを取得します
func GetTenantIDFromContext(ctx context.Context) (uuid.UUID, error) {
	tenantID, ok := ctx.Value(model.TenantIDKey).(uuid.UUID)
	if !ok {
		log.Println("Error: Tenant ID not found in context")
		// ここでは内部エラーではなく、認証が正しく行われなかった状態を示す
		return uuid.Nil, model.ErrTenantNotFound
	}
	return tenantID, nil
}
