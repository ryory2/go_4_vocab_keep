package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProgressRepository インターフェース (変更なし)
type ProgressRepository interface {
	Create(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error
	FindByWordID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.LearningProgress, error)
	Update(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error
	FindReviewableByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, today time.Time, limit int) ([]*model.LearningProgress, error)
	DeleteByWordID(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID) error
}

// gormProgressRepository 構造体から logger フィールドを削除
type gormProgressRepository struct{}

// NewGormProgressRepository コンストラクタから logger 引数を削除
func NewGormProgressRepository() ProgressRepository {
	return &gormProgressRepository{}
}

func (r *gormProgressRepository) Create(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error {
	logger := middleware.GetLogger(ctx)
	result := tx.WithContext(ctx).Create(progress)
	if result.Error != nil {
		logger.Error("Error creating progress in DB",
			"error", result.Error,
			"tenant_id", progress.TenantID.String(),
			"word_id", progress.WordID.String(),
		)
		return fmt.Errorf("gormProgressRepository.Create: %w", result.Error)
	}
	return nil
}

func (r *gormProgressRepository) FindByWordID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.LearningProgress, error) {
	logger := middleware.GetLogger(ctx)
	var progress model.LearningProgress

	// 関連するWordが論理削除されていないProgressレコードのみを検索
	result := db.WithContext(ctx).
		Joins("JOIN words ON words.word_id = learning_progress.word_id AND words.deleted_at IS NULL").
		Where("learning_progress.tenant_id = ? AND learning_progress.word_id = ?", tenantID, wordID).
		First(&progress)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		logger.Error("Error finding progress by word ID in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
			"word_id", wordID.String(),
		)
		return nil, fmt.Errorf("gormProgressRepository.FindByWordID: %w", result.Error)
	}
	return &progress, nil
}

func (r *gormProgressRepository) Update(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error {
	logger := middleware.GetLogger(ctx)

	// .Updates は主キーに基づいて更新する
	result := tx.WithContext(ctx).Model(&progress).Updates(progress)

	if result.Error != nil {
		logger.Error("Error updating progress in DB",
			"error", result.Error,
			"progress_id", progress.ProgressID.String(),
		)
		return fmt.Errorf("gormProgressRepository.Update: %w", result.Error)
	}

	// 更新対象が見つからなかった場合
	if result.RowsAffected == 0 {
		logger.Warn("Progress not found for update", "progress_id", progress.ProgressID.String())
		return model.ErrNotFound
	}

	return nil
}

func (r *gormProgressRepository) FindReviewableByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, today time.Time, limit int) ([]*model.LearningProgress, error) {
	logger := middleware.GetLogger(ctx)
	var progresses []*model.LearningProgress

	// todayの日付の始まり（00:00:00）にする
	todayDate := today.Truncate(24 * time.Hour)

	result := db.WithContext(ctx).
		Preload("Word"). // 関連するWordの情報も取得
		Joins("JOIN words ON words.word_id = learning_progress.word_id AND words.deleted_at IS NULL").
		Where("learning_progress.tenant_id = ? AND learning_progress.next_review_date <= ?", tenantID, todayDate).
		Order("RANDOM()").
		Limit(limit).
		Find(&progresses)

	if result.Error != nil {
		logger.Error("Error finding reviewable progress by tenant in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
			"today_date", todayDate,
		)
		return nil, fmt.Errorf("gormProgressRepository.FindReviewableByTenant: %w", result.Error)
	}

	return progresses, nil
}

func (r *gormProgressRepository) DeleteByWordID(ctx context.Context, tx *gorm.DB, tenantID, wordID uuid.UUID) error {
	logger := middleware.GetLogger(ctx)
	// wordIDに紐づく全てのprogressを削除する
	result := tx.WithContext(ctx).Where("tenant_id = ? AND word_id = ?", tenantID, wordID).Delete(&model.LearningProgress{})
	if result.Error != nil {
		logger.Error("Error deleting progress by word_id in DB",
			"error", result.Error,
			"tenant_id", tenantID.String(),
			"word_id", wordID.String(),
		)
		return fmt.Errorf("gormProgressRepository.DeleteByWordID: %w", result.Error)
	}
	// 削除対象がなくてもエラーにしない（冪等性）
	return nil
}
