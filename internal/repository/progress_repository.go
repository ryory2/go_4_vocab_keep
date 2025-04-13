// internal/repository/progress_repository.go
package repository

import (
	"context"
	"errors"
	"time"

	"go_4_vocab_keep/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProgressRepository interface {
	Create(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error // トランザクション対応
	FindByWordID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.LearningProgress, error)
	Update(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error                                                            // トランザクション対応
	FindReviewableByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, today time.Time, limit int) ([]*model.LearningProgress, error) // WordはPreloadする
}

type gormProgressRepository struct {
	// DB接続はService層から渡される想定
}

func NewGormProgressRepository() ProgressRepository {
	return &gormProgressRepository{}
}

func (r *gormProgressRepository) Create(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error {
	// UUIDはService層で設定済み想定
	result := tx.WithContext(ctx).Create(progress)
	// GORMは複合ユニーク制約違反などをErrorで返す
	return result.Error
}

func (r *gormProgressRepository) FindByWordID(ctx context.Context, db *gorm.DB, tenantID, wordID uuid.UUID) (*model.LearningProgress, error) {
	var progress model.LearningProgress
	// Preloadで関連するWordも取得する (必要に応じて)
	result := db.WithContext(ctx).Preload("Word").Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&progress)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		return nil, result.Error
	}
	// PreloadしたWordが論理削除されているかチェック (必要なら)
	if progress.Word != nil && progress.Word.DeletedAt.Valid {
		return nil, model.ErrNotFound // 実質的にProgressも無効とみなす
	}
	return &progress, nil
}

func (r *gormProgressRepository) Update(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error {
	// progress オブジェクト全体を渡して更新
	// Select で更新対象カラムを限定することも可能
	result := tx.WithContext(ctx).Save(progress) // Saveは主キーに基づいてUpdate or Insertを行う。ここではUpdate。
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// Saveの場合、更新がなくてもエラーにならないことがある。事前に存在確認が必要。
		// ここでは呼び出し元(Service)で存在確認している想定
	}
	return nil
}

func (r *gormProgressRepository) FindReviewableByTenant(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, today time.Time, limit int) ([]*model.LearningProgress, error) {
	var progresses []*model.LearningProgress
	todayDate := today.Truncate(24 * time.Hour)

	// Preloadを使用して関連するWord情報も取得する
	// JOIN条件はGORMがモデル定義から自動生成するが、明示も可能
	// Wordが論理削除されていないものだけを対象にする
	result := db.WithContext(ctx).
		Preload("Word", "deleted_at IS NULL").                                                         // WordをPreloadし、論理削除されていないもののみ
		Joins("JOIN words ON words.word_id = learning_progress.word_id AND words.deleted_at IS NULL"). // Wordが存在しかつ削除されていないもののみJOIN
		Where("learning_progress.tenant_id = ? AND learning_progress.next_review_date <= ?", tenantID, todayDate).
		Order("learning_progress.next_review_date ASC, learning_progress.level ASC").
		Limit(limit).
		Find(&progresses)

	if result.Error != nil {
		return nil, result.Error
	}

	// PreloadだけだとWordがNULLになる場合があるため、Joinsで絞り込んだ上で取得する

	return progresses, nil
}
