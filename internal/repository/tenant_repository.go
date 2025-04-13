// internal/repository/tenant_repository.go
package repository

import (
	"context"
	"errors"
	"log" // Logを追加

	"go_4_vocab_keep/internal/model" // プロジェクト名修正

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TenantRepository interface {
	Create(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error
	FindByID(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) (*model.Tenant, error)
}

type gormTenantRepository struct{}

func NewGormTenantRepository() TenantRepository {
	return &gormTenantRepository{}
}

func (r *gormTenantRepository) Create(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error {
	// UUIDはService層で設定するか、DBのデフォルト機能に任せる（ここではServiceで設定想定）
	// tenant.TenantID = uuid.New()
	result := db.WithContext(ctx).Create(tenant)
	if result.Error != nil {
		// TODO: より詳細なエラーハンドリング (例: 重複キーエラー)
		log.Printf("Error creating tenant in DB: %v", result.Error)
	}
	return result.Error
}

func (r *gormTenantRepository) FindByID(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) (*model.Tenant, error) {
	var tenant model.Tenant
	// First は自動で DeletedAt IS NULL を考慮する
	result := db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&tenant)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		log.Printf("Error finding tenant %s in DB: %v", tenantID, result.Error)
		return nil, model.ErrInternalServer // DBエラーは汎用エラーに
	}
	return &tenant, nil
}
