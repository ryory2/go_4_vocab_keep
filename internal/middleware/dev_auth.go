// internal/middleware/dev_auth.go (新規作成)
package middleware

import (
	"context"
	"log"
	"net/http"

	"go_4_vocab_keep/internal/model"   // プロジェクト名修正
	"go_4_vocab_keep/internal/webutil" // プロジェクト名修正

	"github.com/google/uuid"
)

// DevTenantContextMiddleware は開発時用ミドルウェアです。
// X-Tenant-ID ヘッダーからUUIDを抽出し、コンテキストに設定します。
// DBでのテナント存在チェックは行いません。
func DevTenantContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantIDStr := r.Header.Get("X-Tenant-ID")
		if tenantIDStr == "" {
			// 開発時でも Tenant ID は必須とする (API利用のために)
			log.Println("[DEV AUTH] Failed: X-Tenant-ID header missing")
			webutil.RespondWithError(w, http.StatusUnauthorized, "[DEV] Unauthorized: Missing X-Tenant-ID header")
			return
		}

		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			log.Printf("[DEV AUTH] Failed: Invalid X-Tenant-ID format: %s", tenantIDStr)
			webutil.RespondWithError(w, http.StatusUnauthorized, "[DEV] Unauthorized: Invalid X-Tenant-ID format")
			return
		}

		// DB検証はスキップ
		log.Printf("[DEV AUTH] Tenant ID %s set to context (no validation)", tenantID)

		// コンテキストにテナントIDをセット
		ctx := context.WithValue(r.Context(), model.TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
