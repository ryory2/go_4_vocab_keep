//go:generate mockery --name WordRepository --output ./mocks --outpkg mocks --case=underscore
package repository

import (
	"context"
	"errors"
	"fmt"

	// middleware.GetLoggerが返す型として必要
	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WordRepository インターフェース (変更なし)
type WordRepository interface {
	Create(ctx context.Context, tx *gorm.DB, word *model.Word) error
	FindByID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.Word, error)
	FindByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) ([]*model.Word, error)
	Update(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID, updates map[string]interface{}) error
	Delete(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID) error
	CheckTermExists(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, term string, excludeWordID *uuid.UUID) (bool, error)
}

// gormWordRepository 構造体から logger フィールドを削除
type gormWordRepository struct{}

// NewGormWordRepository コンストラクタから logger 引数を削除
func NewGormWordRepository() WordRepository {
	return &gormWordRepository{}
}

func (r *gormWordRepository) Create(ctx context.Context, tx *gorm.DB, word *model.Word) error {
	logger := middleware.GetLogger(ctx)
	result := tx.WithContext(ctx).Create(word)
	if result.Error != nil {
		logger.Error("Error creating word in DB",
			"error", result.Error,
			"tenant_id", word.TenantID.String(),
			"term", word.Term,
		)
		return fmt.Errorf("gormWordRepository.Create: %w", result.Error)
	}
	return nil
}

func (r *gormWordRepository) FindByID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.Word, error) {
	logger := middleware.GetLogger(ctx)
	var word model.Word
	result := db.WithContext(ctx).Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&word)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		logger.Error("Error finding word by ID in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
			"word_id", wordID.String(),
		)
		return nil, fmt.Errorf("gormWordRepository.FindByID: %w", result.Error)
	}
	return &word, nil
}

func (r *gormWordRepository) FindByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) ([]*model.Word, error) {
	logger := middleware.GetLogger(ctx)
	var words []*model.Word
	result := db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&words)
	if result.Error != nil {
		logger.Error("Error finding words by tenant in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
		)
		return nil, fmt.Errorf("gormWordRepository.FindByTenant: %w", result.Error)
	}
	return words, nil
}

func (r *gormWordRepository) Update(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID, updates map[string]interface{}) error {
	logger := middleware.GetLogger(ctx)
	if len(updates) == 0 {
		return nil
	}
	result := tx.WithContext(ctx).Model(&model.Word{}).Where("tenant_id = ? AND word_id = ?", tenantID, wordID).Updates(updates)
	if result.Error != nil {
		logger.Error("Error updating word in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
			"word_id", wordID.String(),
		)
		return fmt.Errorf("gormWordRepository.Update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *gormWordRepository) Delete(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID) error {
	logger := middleware.GetLogger(ctx)
	result := tx.WithContext(ctx).Where("tenant_id = ?", tenantID).Delete(&model.Word{}, wordID)
	if result.Error != nil {
		logger.Error("Error deleting word in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
			"word_id", wordID.String(),
		)
		return fmt.Errorf("gormWordRepository.Delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *gormWordRepository) CheckTermExists(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, term string, excludeWordID *uuid.UUID) (bool, error) {
	logger := middleware.GetLogger(ctx)
	var count int64
	query := db.WithContext(ctx).Model(&model.Word{}).Where("tenant_id = ? AND term = ?", tenantID, term)
	if excludeWordID != nil {
		query = query.Where("word_id != ?", *excludeWordID)
	}
	result := query.Count(&count)
	if result.Error != nil {
		logger.Error("Error checking term existence in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
			"term", term,
		)
		return false, fmt.Errorf("gormWordRepository.CheckTermExists: %w", result.Error)
	}
	return count > 0, nil
}
