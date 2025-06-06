// internal/middleware/auth.go
package middleware

import (
	"context"
	"errors"
	"log/slog" // logをslogに変更
	"net/http"

	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/service"
	"go_4_vocab_keep/internal/webutil"

	"github.com/google/uuid"
)

// TenantAuthenticator はテナント認証を行うためのインターフェース
type TenantAuthenticator interface {
	Authenticate(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

// serviceTenantAuthenticator は TenantService を使って認証を行う実装
type serviceTenantAuthenticator struct {
	tenantService service.TenantService
	logger        *slog.Logger // slog.Loggerを追加
}

// NewServiceTenantAuthenticator は新しいオーセンティケータを作成します
func NewServiceTenantAuthenticator(ts service.TenantService, logger *slog.Logger) TenantAuthenticator {
	if logger == nil {
		logger = slog.Default()
	}
	return &serviceTenantAuthenticator{
		tenantService: ts,
		logger:        logger,
	}
}

// Authenticate はテナントの存在と有効性を確認します
func (a *serviceTenantAuthenticator) Authenticate(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	_, err := a.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return false, nil // 見つからない場合は認証失敗 (エラーではない)
		}
		// その他のエラー (DBエラーなど) はログに記録
		a.logger.Error("Error during tenant authentication check", slog.String("tenant_id", tenantID.String()), slog.Any("error", err))
		return false, err
	}
	// エラーなく取得できれば有効なテナント
	return true, nil
}

// TenantAuthMiddleware はテナント認証を行うミドルウェアを作成します
func TenantAuthMiddleware(auth TenantAuthenticator, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantIDStr := r.Header.Get("X-Tenant-ID")
			if tenantIDStr == "" {
				logger.Warn("Authentication failed: X-Tenant-ID header missing")
				appErr := model.NewAppError("MISSING_TENANT_HEADER", "X-Tenant-IDヘッダーが必要です。", "X-Tenant-ID", model.ErrForbidden)
				webutil.HandleError(w, logger, appErr)
				return
			}

			tenantID, err := uuid.Parse(tenantIDStr)
			if err != nil {
				logger.Warn("Authentication failed: Invalid X-Tenant-ID format", "X-Tenant-ID", tenantIDStr)
				appErr := model.NewAppError("INVALID_TENANT_ID_FORMAT", "X-Tenant-IDの形式が正しくありません。", "X-Tenant-ID", model.ErrForbidden)
				webutil.HandleError(w, logger, appErr)
				return
			}

			isValid, err := auth.Authenticate(r.Context(), tenantID)
			if err != nil {
				// 認証処理中の内部エラー (DBエラーなど)
				// Authenticateメソッド内で既にログは出力されているので、ここでは不要
				webutil.HandleError(w, logger, err) // 内部エラーとして処理を委譲
				return
			}
			if !isValid {
				logger.Warn("Forbidden: Tenant not found or inactive", slog.String("tenant_id", tenantID.String()))
				appErr := model.NewAppError("INVALID_TENANT", "指定されたテナントは無効、または存在しません。", "X-Tenant-ID", model.ErrTenantNotFound)
				webutil.HandleError(w, logger, appErr)
				return
			}

			ctx := context.WithValue(r.Context(), model.TenantIDKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantIDFromContext は、渡されたコンテキスト(ctx)からテナントIDを取得します。
// この関数はログを出力せず、値の取得とエラーの返却に専念します。
func GetTenantIDFromContext(ctx context.Context) (uuid.UUID, error) {
	value, ok := ctx.Value(model.TenantIDKey).(uuid.UUID)
	if !ok {
		// ログ出力を削除。呼び出し元がエラーをハンドリングし、必要に応じてログを出力する。
		return uuid.Nil, model.ErrTenantNotFound
	}
	return value, nil
}
