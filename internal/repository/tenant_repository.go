package repository

import (
	"context"
	"errors"
	"fmt"

	// slogはmiddleware.GetLoggerが返す型として必要
	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type TenantRepository interface {
	Create(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error
	FindByID(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) (*model.Tenant, error)
	FindByName(ctx context.Context, db *gorm.DB, name string) (*model.Tenant, error)
	FindByEmail(ctx context.Context, db *gorm.DB, email string) (*model.Tenant, error)
	Delete(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) error
}

// loggerフィールドを削除
type gormTenantRepository struct{}

// logger引数を削除
func NewGormTenantRepository() TenantRepository {
	return &gormTenantRepository{}
}

func (r *gormTenantRepository) Create(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error {
	logger := middleware.GetLogger(ctx)

	result := db.WithContext(ctx).Create(tenant)
	if result.Error != nil {
		var pgErr *pgconn.PgError
		if errors.As(result.Error, &pgErr) && pgErr.Code == "23505" {
			logger.Warn(
				"Duplicate key error on create tenant",
				"error", result.Error,
				"tenant_name", tenant.Name,
				"email", tenant.Email,
			)
			return model.ErrConflict
		}

		logger.Error(
			"Error creating tenant in DB",
			"error", result.Error,
			"tenant_name", tenant.Name,
		)
		return fmt.Errorf("gormTenantRepository.Create: %w", result.Error)
	}

	return nil
}

func (r *gormTenantRepository) FindByID(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) (*model.Tenant, error) {
	logger := middleware.GetLogger(ctx)
	var tenant model.Tenant

	result := db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&tenant)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		logger.Error(
			"Error finding tenant by ID in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
		)
		return nil, fmt.Errorf("gormTenantRepository.FindByID: %w", result.Error)
	}
	return &tenant, nil
}

func (r *gormTenantRepository) FindByName(ctx context.Context, db *gorm.DB, name string) (*model.Tenant, error) {
	logger := middleware.GetLogger(ctx)
	var tenant model.Tenant

	result := db.WithContext(ctx).Where("name = ?", name).First(&tenant)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			logger.Debug("Tenant not found by name", "name", name)
			return nil, model.ErrNotFound
		}
		logger.Error(
			"Error finding tenant by name in DB",
			"error", result.Error,
			"name", name,
		)
		return nil, fmt.Errorf("gormTenantRepository.FindByName: %w", result.Error)
	}

	return &tenant, nil
}

func (r *gormTenantRepository) Delete(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) error {
	logger := middleware.GetLogger(ctx)
	result := db.WithContext(ctx).Delete(&model.Tenant{}, tenantID)

	if result.Error != nil {
		logger.Error(
			"Error deleting tenant in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
		)
		return fmt.Errorf("gormTenantRepository.Delete: %w", result.Error)
	}

	// 冪等性を保つため、RowsAffected == 0 でもエラーにしない
	if result.RowsAffected == 0 {
		logger.Warn("Tenant not found for deletion (idempotent)", "tenant_id", tenantID.String())
		// ここで ErrNotFound を返すと、呼び出し側で「見つからなかった」場合の処理が必要になる。
		// DELETE 操作では、最終的に「存在しない」状態になれば良いので、nilを返すのが一般的。
	}

	return nil
}

func (r *gormTenantRepository) FindByEmail(ctx context.Context, db *gorm.DB, email string) (*model.Tenant, error) {
	logger := middleware.GetLogger(ctx)
	var tenant model.Tenant

	result := db.WithContext(ctx).Where("email = ?", email).First(&tenant)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			logger.Debug("Tenant not found by email", "email", email)
			return nil, model.ErrNotFound
		}
		logger.Error(
			"Error finding tenant by email in DB",
			"error", result.Error,
			"email", email,
		)
		return nil, fmt.Errorf("gormTenantRepository.FindByEmail: %w", result.Error)
	}
	return &tenant, nil
}
