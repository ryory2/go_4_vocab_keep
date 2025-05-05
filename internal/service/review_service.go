//go:generate mockery --name ReviewService --srcpkg go_4_vocab_keep/internal/service --output ./mocks --outpkg mocks --case=underscore
package service

import (
	"context"
	"errors"
	"log/slog" // slog パッケージをインポート
	"time"

	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReviewService インターフェース
type ReviewService interface {
	GetReviewWords(ctx context.Context, tenantID uuid.UUID) ([]*model.ReviewWordResponse, error)
	SubmitReviewResult(ctx context.Context, tenantID, wordID uuid.UUID, isCorrect bool) error
}

// reviewService 構造体に logger フィールドを追加
type reviewService struct {
	db       *gorm.DB
	progRepo repository.ProgressRepository
	cfg      config.Config
	logger   *slog.Logger // slog.Logger フィールドを追加
}

// NewReviewService コンストラクタで logger を受け取るように変更
func NewReviewService(db *gorm.DB, progRepo repository.ProgressRepository, cfg config.Config, logger *slog.Logger) ReviewService { // logger を引数に追加
	// ロガーが nil の場合にデフォルトロガーを使用
	if logger == nil {
		logger = slog.Default()
	}
	return &reviewService{
		db:       db,
		progRepo: progRepo,
		cfg:      cfg,
		logger:   logger, // logger を設定
	}
}

func (s *reviewService) GetReviewWords(ctx context.Context, tenantID uuid.UUID) ([]*model.ReviewWordResponse, error) {
	operation := "GetReviewWords"
	logger := s.logger.With(slog.String("operation", operation), slog.String("tenant_id", tenantID.String()))
	now := time.Now()

	// リポジトリにDB接続を渡す
	progresses, err := s.progRepo.FindReviewableByTenant(ctx, s.db, tenantID, now, s.cfg.App.ReviewLimit)
	if err != nil {
		// slog でエラーログ (リポジトリ層でもログされる可能性あり)
		logger.Error("Error finding reviewable words from repository", slog.Any("error", err))
		return nil, model.ErrInternalServer
	}

	// レスポンスDTOに変換
	responses := make([]*model.ReviewWordResponse, 0, len(progresses))
	for _, p := range progresses {
		// PreloadしたWordがnilでないことを確認
		if p.Word == nil {
			// slog で警告ログ (データ不整合の可能性)
			logger.Warn("Found progress with nil Word during review generation, skipping",
				slog.String("progress_id", p.ProgressID.String()),
				slog.String("word_id", p.WordID.String()), // WordIDはProgressにあるはず
			)
			continue
		}
		responses = append(responses, &model.ReviewWordResponse{
			WordID:     p.WordID,
			Term:       p.Word.Term,
			Definition: p.Word.Definition,
			Level:      p.Level,
		})
	}

	// slog で成功ログ (任意)
	logger.Info("Successfully retrieved review words", slog.Int("count", len(responses)))
	return responses, nil
}

// 進捗状況を更新する
func (s *reviewService) SubmitReviewResult(ctx context.Context, tenantID, wordID uuid.UUID, isCorrect bool) error {
	operation := "SubmitReviewResult"
	logger := s.logger.With(
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", wordID.String()),
	)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 該当の学習進捗を取得 (トランザクション内で)
		var progress *model.LearningProgress
		// FindByWordID に tx を渡す (リポジトリが対応している必要あり)
		// または、ここで直接クエリを実行する
		// progress, findErr := s.progRepo.FindByWordID(ctx, tx, tenantID, wordID)
		result := tx.WithContext(ctx).Preload("Word").Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&progress)
		findErr := result.Error

		if findErr != nil {
			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				// slog で情報ログ
				logger.Info("Progress not found for word during submit")
				return model.ErrNotFound
			}
			// slog でエラーログ
			logger.Error("Error finding progress by word ID in transaction", slog.Any("error", findErr))
			return model.ErrInternalServer
		}
		// Wordが論理削除されていないかもチェック
		if progress.Word == nil || progress.Word.DeletedAt.Valid {
			// slog で情報ログ (関連データが見つからない)
			logger.Info("Word associated with progress is deleted or missing")
			return model.ErrNotFound // 対象の単語が存在しない
		}

		// 2. レベルと次回レビュー日を計算
		now := time.Now()
		previousLevel := progress.Level
		previousNextReviewDate := progress.NextReviewDate
		progress.LastReviewedAt = &now

		if isCorrect {
			switch progress.Level {
			case model.Level1:
				progress.Level = model.Level2
				progress.NextReviewDate = now.AddDate(0, 0, 3)
			case model.Level2:
				progress.Level = model.Level3
				progress.NextReviewDate = now.AddDate(0, 0, 7)
			case model.Level3:
				progress.Level = model.Level3                   // Level3のまま
				progress.NextReviewDate = now.AddDate(0, 0, 14) // 例: さらに伸ばす
			default:
				// slog で警告ログ (不正なレベル)
				logger.Warn("Invalid progress level found, resetting to Level 1",
					slog.Int("invalid_level", int(progress.Level)),
					slog.String("progress_id", progress.ProgressID.String()),
				)
				progress.Level = model.Level1
				progress.NextReviewDate = now.AddDate(0, 0, 1)
			}
		} else {
			progress.Level = model.Level1 // 不正解ならLevel1に戻す
			progress.NextReviewDate = now.AddDate(0, 0, 1)
		}

		// slog で変更内容をログ (任意、デバッグ用)
		logger.Debug("Updating progress level and next review date",
			slog.Int("previous_level", int(previousLevel)),
			slog.Int("new_level", int(progress.Level)),
			slog.Time("previous_next_review_date", previousNextReviewDate),
			slog.Time("new_next_review_date", progress.NextReviewDate),
			slog.Bool("is_correct", isCorrect),
		)

		// 3. 進捗を更新
		// progRepo.Update も tx を受け取るように修正されている想定
		if updateErr := s.progRepo.Update(ctx, tx, progress); updateErr != nil {
			// slog でエラーログ (リポジトリ層でもログされる可能性あり)
			logger.Error("Error updating progress in transaction", slog.Any("error", updateErr))
			return model.ErrInternalServer
		}

		return nil // コミット
	})

	if err != nil {
		// トランザクション内で返されたエラー
		if errors.Is(err, model.ErrNotFound) {
			// NotFound は既にログ済み
			return err
		}
		// slog でトランザクション全体のエラーログ
		logger.Error("Transaction failed", slog.Any("error", err))
		return model.ErrInternalServer
	}

	// slog で成功ログ
	logger.Info("Review result submitted successfully")
	return nil
}
