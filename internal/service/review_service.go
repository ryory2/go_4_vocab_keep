package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReviewService インターフェース (変更なし)
type ReviewService interface {
	GetReviewWords(ctx context.Context, tenantID uuid.UUID) ([]*model.ReviewWordResponse, error)
	GetReviewWordsCount(ctx context.Context, tenantID uuid.UUID) (int64, error)
	UpsertLearningProgressBasedOnReview(ctx context.Context, tenantID, wordID uuid.UUID, isCorrect bool) error
}

// reviewService 構造体から logger を削除
type reviewService struct {
	db       *gorm.DB
	progRepo repository.ProgressRepository
	cfg      *config.Config // ポインタ型に変更
}

// NewReviewService コンストラクタから logger を削除し、cfg をポインタで受け取る
func NewReviewService(db *gorm.DB, progRepo repository.ProgressRepository, cfg *config.Config) ReviewService {
	return &reviewService{
		db:       db,
		progRepo: progRepo,
		cfg:      cfg,
	}
}

func (s *reviewService) GetReviewWords(ctx context.Context, tenantID uuid.UUID) ([]*model.ReviewWordResponse, error) {
	logger := middleware.GetLogger(ctx).With("tenant_id", tenantID)

	progresses, err := s.progRepo.FindReviewableByTenant(ctx, s.db, tenantID, time.Now(), s.cfg.App.ReviewLimit)
	if err != nil {
		logger.Error("Failed to find reviewable words from repository", "error", err)
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "復習単語の取得に失敗しました。", "", err)
	}

	responses := make([]*model.ReviewWordResponse, 0, len(progresses))
	for _, p := range progresses {
		if p.Word == nil {
			logger.Warn("Found progress with nil Word during review generation, skipping", "progress_id", p.ProgressID)
			continue
		}
		responses = append(responses, &model.ReviewWordResponse{
			WordID:     p.WordID,
			Term:       p.Word.Term,
			Definition: p.Word.Definition,
			Level:      p.Level,
		})
	}

	logger.Info("Successfully retrieved review words", "count", len(responses))
	return responses, nil
}

func (s *reviewService) GetReviewWordsCount(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	logger := middleware.GetLogger(ctx).With("tenant_id", tenantID)

	// リポジトリにカウント用のメソッドを追加するのが理想だが、
	// ここでは既存のメソッドを流用して効率的に実装
	progresses, err := s.progRepo.FindReviewableByTenant(ctx, s.db, tenantID, time.Now(), 9999) // limitを大きく設定
	if err != nil {
		logger.Error("Failed to find reviewable words for count", "error", err)
		return 0, model.NewAppError("INTERNAL_SERVER_ERROR", "単語数の取得に失敗しました。", "", err)
	}

	count := int64(len(progresses))
	logger.Info("Successfully counted reviewable words", "count", count)
	return count, nil
}

func (s *reviewService) UpsertLearningProgressBasedOnReview(ctx context.Context, tenantID, wordID uuid.UUID, isCorrect bool) error {
	logger := middleware.GetLogger(ctx).With("tenant_id", tenantID, "word_id", wordID)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		progress, err := s.progRepo.FindByWordID(ctx, tx, tenantID, wordID)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			logger.Error("Error finding progress in transaction", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "学習進捗の確認中にエラーが発生しました。", "", err)
		}

		// isFound は、進捗が見つかったかどうかを示すフラグ
		isFound := !errors.Is(err, model.ErrNotFound)

		// 次のレベルと復習日を計算する
		newLevel, nextReviewDate := calculateNextProgress(progress, isCorrect, logger)

		if !isFound {
			// --- 新規作成 ---
			logger.Info("Progress not found, creating new progress.", "is_correct", isCorrect)
			now := time.Now()
			newProgress := &model.LearningProgress{
				ProgressID:     uuid.New(),
				TenantID:       tenantID,
				WordID:         wordID,
				Level:          newLevel,
				NextReviewDate: nextReviewDate,
				LastReviewedAt: &now,
			}
			if createErr := s.progRepo.Create(ctx, tx, newProgress); createErr != nil {
				logger.Error("Error creating new progress", "error", createErr)
				return model.NewAppError("INTERNAL_SERVER_ERROR", "学習進捗の作成に失敗しました。", "", createErr)
			}
			logger.Debug("New progress created", "new_progress", newProgress)
		} else {
			// --- 更新 ---
			now := time.Now()
			logger.Info("Updating existing progress.", "is_correct", isCorrect)
			progress.Level = newLevel
			progress.NextReviewDate = nextReviewDate
			progress.LastReviewedAt = &now
			if updateErr := s.progRepo.Update(ctx, tx, progress); updateErr != nil {
				// 更新対象が見つからないエラーは、競合状態の可能性がある
				if errors.Is(updateErr, model.ErrNotFound) {
					logger.Warn("Failed to update progress, record not found", "error", updateErr)
					return model.NewAppError("NOT_FOUND", "更新対象の学習進捗が見つかりませんでした。", "", updateErr)
				}
				logger.Error("Error updating existing progress", "error", updateErr)
				return model.NewAppError("INTERNAL_SERVER_ERROR", "学習進捗の更新に失敗しました。", "", updateErr)
			}
			logger.Debug("Progress updated", "updated_progress", progress)
		}

		return nil // トランザクション成功
	})
}

// calculateNextProgress は、次のレベルと復習日を計算するヘルパー関数
func calculateNextProgress(progress *model.LearningProgress, isCorrect bool, logger *slog.Logger) (model.ProgressLevel, time.Time) {
	now := time.Now()

	if !isCorrect {
		// 間違えたら必ずレベル1に戻り、翌日復習
		return model.Level1, now.AddDate(0, 0, -1)
	}

	// 正解した場合
	currentLevel := model.Level1
	if progress != nil {
		currentLevel = progress.Level
	}

	var newLevel model.ProgressLevel
	var nextReviewDate time.Time

	switch currentLevel {
	case model.Level1:
		newLevel = model.Level2
		nextReviewDate = now.AddDate(0, 0, 3)
	case model.Level2:
		newLevel = model.Level3
		nextReviewDate = now.AddDate(0, 0, 7)
	case model.Level3:
		newLevel = model.Level3 // 最高レベル維持
		nextReviewDate = now.AddDate(0, 0, 14)
	default:
		logger.Warn("Invalid progress level found, resetting to Level 1", "invalid_level", int(currentLevel))
		newLevel = model.Level1
		nextReviewDate = now.AddDate(0, 0, 1)
	}

	return newLevel, nextReviewDate
}
