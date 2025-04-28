//go:generate mockery --name ProgressRepository --srcpkg go_4_vocab_keep/internal/repository --output ../repository/mocks --outpkg mocks --case=underscore
package repository

import (
	"context"
	"errors"
	"log/slog" // slog パッケージをインポート
	"time"

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
}

// gormProgressRepository 構造体に logger フィールドを追加
type gormProgressRepository struct {
	logger *slog.Logger // slog.Logger フィールドを追加
}

// NewGormProgressRepository コンストラクタで logger を受け取るように変更
func NewGormProgressRepository(logger *slog.Logger) ProgressRepository { // logger を引数に追加
	// ロガーが nil の場合にデフォルトロガーを使用
	if logger == nil {
		logger = slog.Default()
	}
	return &gormProgressRepository{
		logger: logger, // logger を設定
	}
}

func (r *gormProgressRepository) Create(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error {
	result := tx.WithContext(ctx).Create(progress)
	if result.Error != nil {
		// slog で予期せぬDBエラーログ
		r.logger.Error("Error creating progress in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", progress.TenantID.String()),
			slog.String("word_id", progress.WordID.String()),
			slog.String("progress_id", progress.ProgressID.String()),
		)
		return result.Error // エラーをそのまま返す
	}
	return nil
}

func (r *gormProgressRepository) FindByWordID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.LearningProgress, error) {
	var progress model.LearningProgress
	result := db.WithContext(ctx).Preload("Word").Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&progress)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound // 見つからない場合はログ不要
		}
		// slog で予期せぬDBエラーログ
		r.logger.Error("Error finding progress by word ID in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return nil, result.Error // エラーをそのまま返す
	}
	// PreloadしたWordが論理削除されているかチェック (必要なら)
	if progress.Word != nil && progress.Word.DeletedAt.Valid {
		// slog で情報ログ (データ不整合の可能性)
		r.logger.Info("Found progress but associated word is deleted",
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
			slog.String("progress_id", progress.ProgressID.String()),
		)
		return nil, model.ErrNotFound // 実質的にProgressも無効とみなす
	}
	return &progress, nil
}

func (r *gormProgressRepository) Update(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error {
	result := tx.WithContext(ctx).Save(progress)
	if result.Error != nil {
		// slog で予期せぬDBエラーログ
		r.logger.Error("Error updating progress in DB (using Save)",
			slog.Any("error", result.Error),
			slog.String("tenant_id", progress.TenantID.String()),
			slog.String("word_id", progress.WordID.String()),
			slog.String("progress_id", progress.ProgressID.String()),
		)
		return result.Error // エラーをそのまま返す
	}
	// Save は RowsAffected が 0 でもエラーにならないことがある
	// if result.RowsAffected == 0 {
	// 	// 必要であればログ出力やエラーハンドリング
	// }
	return nil
}

func (r *gormProgressRepository) FindReviewableByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, today time.Time, limit int) ([]*model.LearningProgress, error) {
	var progresses []*model.LearningProgress
	todayDate := today.Truncate(24 * time.Hour)

	result := db.WithContext(ctx).
		Preload("Word", "deleted_at IS NULL").
		Joins("JOIN words ON words.word_id = learning_progress.word_id AND words.deleted_at IS NULL").
		Where("learning_progress.tenant_id = ? AND learning_progress.next_review_date <= ?", tenantID, todayDate).
		Order("learning_progress.next_review_date ASC, learning_progress.level ASC").
		Limit(limit).
		Find(&progresses)

	if result.Error != nil {
		// slog で予期せぬDBエラーログ
		r.logger.Error("Error finding reviewable progress by tenant in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", tenantID.String()),
			slog.Time("today_date", todayDate),
			slog.Int("limit", limit),
		)
		return nil, result.Error // エラーをそのまま返す
	}

	return progresses, nil
}
