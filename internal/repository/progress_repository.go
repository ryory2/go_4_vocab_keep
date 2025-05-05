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

// 学習進捗レコードをデータベースに挿入する
// ctx: リクエストスコープのコンテキスト。
// tx: GORM トランザクションオブジェクト。
// progress: データベースに作成する学習進捗データ。
// error: データベースエラーが発生した場合に返却。成功した場合は nil。
func (r *gormProgressRepository) Create(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error {
	result := tx.WithContext(ctx).Create(progress)
	if result.Error != nil {
		// slog で予期せぬDBエラーログ
		r.logger.ErrorContext(ctx, "Error creating progress in DB",
			slog.Any("error", result.Error),
			slog.String("tenant_id", progress.TenantID.String()),
			slog.String("word_id", progress.WordID.String()),
			slog.String("progress_id", progress.ProgressID.String()),
		)
		return result.Error // エラーをそのまま返す
	}
	return nil
}

// 指定されたテナントIDと単語IDに一致する学習進捗レコードを返却する。
// ctx: リクエストスコープのコンテキスト。
// db: GORM データベース接続またはトランザクションオブジェクト。
// tenantID: 検索するテナントID。
// wordID: 検索する単語ID。
// (*model.LearningProgress): 見つかった学習進捗データ。見つからない場合や関連単語が削除済みの場合は nil。
// error: レコードが見つからない場合や関連単語が削除済みの場合は `model.ErrNotFound`。その他のDBエラーの場合はそのエラー。
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

// Save は主キーに基づいて動作し、レコードが存在しない場合は作成しようとしますが、
// 通常このメソッドは既存レコードの更新に使われます。
// データベース操作で予期せぬエラーが発生した場合、エラー内容と関連情報を slog を用いてログに出力します。
//
// ctx: リクエストスコープのコンテキスト。
// tx: GORM トランザクションオブジェクト。
// progress: 更新する学習進捗データ。ProgressID が設定されている必要があります。
// error: データベースエラーが発生した場合に返されます。成功した場合は nil。
func (r *gormProgressRepository) Update(ctx context.Context, tx *gorm.DB, progress *model.LearningProgress) error {
	// レコードの作成（Create）と更新（Update）の両方の機能を持つメソッド
	// モデルインスタンスの主キーを確認し、存在する場合はすべてのフィールドを更新、しない場合はインサートする。
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

// 指定されたテナントについて、復習期限 (`next_review_date`) が指定された `today` 以前であり、
// かつ関連する単語 (`words` テーブル) が論理削除されていない学習進捗レコードを検索
// 結果は `next_review_date` の昇順、次に `level` の昇順でソートされ、`limit` 件数に制限

// ctx: リクエストスコープのコンテキスト。
// db: GORM データベース接続またはトランザクションオブジェクト。
// tenantID: 検索対象のテナントID。
// today: 復習期限の基準日。この日の0時0分0秒以前が対象となります。
// limit: 取得する最大レコード数。
// ([]*model.LearningProgress): 条件に一致した学習進捗データのスライス。見つからない場合は空のスライス。
// error: データベースエラーが発生した場合に返されます。成功した場合は nil。
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
