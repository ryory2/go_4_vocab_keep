// internal/service/tenant_service.go
package service

import (
	"context" // errorsを追加
	"log"

	"go_1_test_repository/internal/model"      // プロジェクト名修正
	"go_1_test_repository/internal/repository" // プロジェクト名修正

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TenantService interface {
	CreateTenant(ctx context.Context, name string) (*model.Tenant, error)
	GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) // GetTenantを追加
}

type tenantService struct {
	db         *gorm.DB
	tenantRepo repository.TenantRepository
}

func NewTenantService(db *gorm.DB, repo repository.TenantRepository) TenantService {
	return &tenantService{db: db, tenantRepo: repo}
}

func (s *tenantService) CreateTenant(ctx context.Context, name string) (*model.Tenant, error) {
	if name == "" {
		return nil, model.ErrInvalidInput
	}
	tenant := &model.Tenant{
		TenantID: uuid.New(), // Service層でUUIDを生成
		Name:     name,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.tenantRepo.Create(ctx, tx, tenant); err != nil {
			// TODO: 重複エラー(UNIQUE constraint)などのハンドリング
			// GORMのエラーから特定のDBエラーを判定するのは少し複雑
			// ここでは単純にログ出力と汎用エラーを返す
			log.Printf("Error creating tenant in repo: %v", err)
			return model.ErrInternalServer // 汎用エラーを返す
		}
		return nil // コミット
	})
	if err != nil {
		// トランザクション内で発生したエラー (model.ErrInternalServer など)
		return nil, err
	}
	return tenant, nil
}

// GetTenant は指定されたIDのテナントを取得します (認証用などに利用)
func (s *tenantService) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	tenant, err := s.tenantRepo.FindByID(ctx, s.db, tenantID)
	if err != nil {
		// model.ErrNotFound や model.ErrInternalServer が返る想定
		return nil, err
	}
	return tenant, nil
}
