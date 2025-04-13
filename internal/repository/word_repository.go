// internal/repository/word_repository.go
package repository

import (
	"context"
	"errors"

	"go_1_test_repository/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WordRepository interface {
	Create(ctx context.Context, tx *gorm.DB, word *model.Word) error // トランザクション対応
	FindByID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.Word, error)
	FindByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) ([]*model.Word, error)
	Update(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID, updates map[string]interface{}) error // トランザクション対応
	Delete(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID) error                                 // トランザクション対応
	CheckTermExists(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, term string, excludeWordID *uuid.UUID) (bool, error)
}

type gormWordRepository struct {
	// DB接続はService層から渡される想定
}

func NewGormWordRepository() WordRepository {
	return &gormWordRepository{}
}

func (r *gormWordRepository) Create(ctx context.Context, tx *gorm.DB, word *model.Word) error {
	// UUIDはService層で設定済み想定
	result := tx.WithContext(ctx).Create(word)
	return result.Error
}

func (r *gormWordRepository) FindByID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.Word, error) {
	var word model.Word
	result := db.WithContext(ctx).Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&word)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		return nil, result.Error
	}
	return &word, nil
}

func (r *gormWordRepository) FindByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) ([]*model.Word, error) {
	var words []*model.Word
	result := db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&words)
	if result.Error != nil {
		return nil, result.Error
	}
	return words, nil
}

func (r *gormWordRepository) Update(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil // 更新内容がなければ何もしない
	}
	result := tx.WithContext(ctx).Model(&model.Word{}).Where("tenant_id = ? AND word_id = ?", tenantID, wordID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 更新対象が見つからなかった場合、FindByIDで事前にチェックしているのでエラーとするのが一般的
		return model.ErrNotFound
	}
	return nil
}

func (r *gormWordRepository) Delete(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID) error {
	result := tx.WithContext(ctx).Where("tenant_id = ?", tenantID).Delete(&model.Word{}, wordID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *gormWordRepository) CheckTermExists(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, term string, excludeWordID *uuid.UUID) (bool, error) {
	var count int64
	query := db.WithContext(ctx).Model(&model.Word{}).Where("tenant_id = ? AND term = ?", tenantID, term)
	if excludeWordID != nil {
		query = query.Where("word_id != ?", *excludeWordID)
	}
	// GORMのCountは論理削除されたものを除外する
	result := query.Count(&count)
	if result.Error != nil {
		return false, result.Error
	}
	return count > 0, nil
}
