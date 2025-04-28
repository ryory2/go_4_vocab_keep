//go:generate mockery --name WordRepository --srcpkg go_4_vocab_keep/internal/repository --output ../repository/mocks --outpkg mocks --case=underscore
package repository

import (
	"context"
	"errors"
	"log/slog" // ★ slog パッケージをインポート

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

// gormWordRepository 構造体に logger フィールドを追加
type gormWordRepository struct {
	logger *slog.Logger // ★ slog.Logger フィールドを追加
}

// NewGormWordRepository コンストラクタで logger を受け取るように変更
func NewGormWordRepository(logger *slog.Logger) WordRepository { // ★ logger を引数に追加
	// ロガーが nil の場合にデフォルトロガーを使用
	if logger == nil {
		logger = slog.Default()
	}
	return &gormWordRepository{
		logger: logger, // ★ logger を設定
	}
}

func (r *gormWordRepository) Create(ctx context.Context, tx *gorm.DB, word *model.Word) error {
	result := tx.WithContext(ctx).Create(word)
	if result.Error != nil {
		// ★ slog で予期せぬDBエラーログ ★
		r.logger.Error("Error creating word in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", word.TenantID.String()),
			slog.String("term", word.Term),
		)
		return result.Error // エラーをそのまま返す
	}
	return nil
}

func (r *gormWordRepository) FindByID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.Word, error) {
	var word model.Word
	result := db.WithContext(ctx).Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&word)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound // 見つからない場合はログ不要
		}
		// ★ slog で予期せぬDBエラーログ ★
		r.logger.Error("Error finding word by ID in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return nil, result.Error // エラーをそのまま返す
	}
	return &word, nil
}

func (r *gormWordRepository) FindByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) ([]*model.Word, error) {
	var words []*model.Word
	result := db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&words)
	if result.Error != nil {
		// ★ slog で予期せぬDBエラーログ ★
		// Find は ErrRecordNotFound を返さないので、エラーがあれば常に予期せぬエラー扱い
		r.logger.Error("Error finding words by tenant in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", tenantID.String()),
		)
		return nil, result.Error // エラーをそのまま返す
	}
	return words, nil
}

func (r *gormWordRepository) Update(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	result := tx.WithContext(ctx).Model(&model.Word{}).Where("tenant_id = ? AND word_id = ?", tenantID, wordID).Updates(updates)
	if result.Error != nil {
		// ★ slog で予期せぬDBエラーログ ★
		r.logger.Error("Error updating word in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
			slog.Any("updates", updates), // 注意: 更新内容に機密情報がないか確認
		)
		return result.Error // エラーをそのまま返す
	}
	if result.RowsAffected == 0 {
		// RowsAffected == 0 は ErrNotFound として扱う (ログ不要)
		return model.ErrNotFound
	}
	return nil
}

func (r *gormWordRepository) Delete(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID) error {
	result := tx.WithContext(ctx).Where("tenant_id = ?", tenantID).Delete(&model.Word{}, wordID)
	if result.Error != nil {
		// ★ slog で予期せぬDBエラーログ ★
		r.logger.Error("Error deleting word in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return result.Error // エラーをそのまま返す
	}
	if result.RowsAffected == 0 {
		// RowsAffected == 0 は ErrNotFound として扱う (ログ不要)
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
	result := query.Count(&count)
	if result.Error != nil {
		// ★ slog で予期せぬDBエラーログ ★
		// Count が返すエラーは予期せぬものとする
		r.logger.Error("Error checking term existence in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", tenantID.String()),
			slog.String("term", term),
		)
		return false, result.Error // エラーをそのまま返す
	}
	return count > 0, nil
}
