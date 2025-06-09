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
	UpsertLearningProgressBasedOnReview(ctx context.Context, tenantID, wordID uuid.UUID, isCorrect bool) error
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
	year, month, day := now.Date()
	endOfDay := time.Date(year, month, day, 23, 59, 59, 0, now.Location())

	// リポジトリにDB接続を渡す
	progresses, err := s.progRepo.FindReviewableByTenant(ctx, s.db, tenantID, endOfDay, s.cfg.App.ReviewLimit)
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
func (s *reviewService) UpsertLearningProgressBasedOnReview(ctx context.Context, tenantID, wordID uuid.UUID, isCorrect bool) error {
	// 関数名に合わせて operation を変更
	operation := "RecordReviewOutcome"
	logger := s.logger.With(
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", wordID.String()),
	)

	// トランザクションを開始 (内部ロジックは前回の修正と同じ)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 該当の学習進捗を取得試行 (リポジトリ経由)
		progress, findErr := s.progRepo.FindByWordID(ctx, tx, tenantID, wordID)

		// --- エラーハンドリング (FindByWordIDの結果による分岐) ---
		if findErr != nil && !errors.Is(findErr, model.ErrNotFound) {
			logger.Error("Error finding progress via repository in transaction", slog.Any("error", findErr))
			return model.ErrInternalServer
		}

		// --- ここから進捗の作成または更新処理 ---
		now := time.Now()
		newLevel := model.Level1
		var nextReviewDate time.Time

		if isCorrect {
			currentLevel := model.Level1
			if progress != nil {
				currentLevel = progress.Level
			}
			switch currentLevel {
			case model.Level1:
				newLevel = model.Level2
				nextReviewDate = now.AddDate(0, 0, 3)
			case model.Level2:
				newLevel = model.Level3
				nextReviewDate = now.AddDate(0, 0, 7)
			case model.Level3:
				newLevel = model.Level3
				nextReviewDate = now.AddDate(0, 0, 14)
			default:
				logger.Warn("Invalid progress level found, resetting to Level 1",
					slog.Int("invalid_level", int(currentLevel)),
					slog.String("word_id", wordID.String()),
				)
				newLevel = model.Level1
				nextReviewDate = now.AddDate(0, 0, 1)
			}
		} else {
			newLevel = model.Level1
			nextReviewDate = now.AddDate(0, 0, 1)
		}

		// --- データベース操作 (Create or Update) ---
		if errors.Is(findErr, model.ErrNotFound) {
			// --- 新規作成 ---
			logger.Info("Progress not found, creating new progress.", slog.Bool("is_correct", isCorrect))
			// (Word存在確認は省略)
			newProgress := &model.LearningProgress{
				ProgressID:     uuid.New(),
				TenantID:       tenantID,
				WordID:         wordID,
				Level:          newLevel,
				NextReviewDate: nextReviewDate,
				LastReviewedAt: &now,
			}
			if createErr := s.progRepo.Create(ctx, tx, newProgress); createErr != nil {
				logger.Error("Error creating new progress in transaction", slog.Any("error", createErr))
				return model.ErrInternalServer
			}
			logger.Debug("New progress created", slog.Any("new_progress", newProgress))

		} else {
			// --- 更新 ---
			if progress.Word == nil || progress.Word.DeletedAt.Valid {
				logger.Info("Word associated with progress is deleted or missing (checked before update)")
				return model.ErrNotFound
			}
			previousLevel := progress.Level
			previousNextReviewDate := progress.NextReviewDate
			logger.Debug("Updating existing progress level and next review date",
				slog.Int("previous_level", int(previousLevel)),
				slog.Int("new_level", int(newLevel)),
				slog.Time("previous_next_review_date", previousNextReviewDate),
				slog.Time("new_next_review_date", nextReviewDate),
				slog.Bool("is_correct", isCorrect),
				slog.String("progress_id", progress.ProgressID.String()),
			)
			progress.Level = newLevel
			progress.NextReviewDate = nextReviewDate
			progress.LastReviewedAt = &now
			if updateErr := s.progRepo.Update(ctx, tx, progress); updateErr != nil {
				logger.Error("Error updating existing progress in transaction", slog.Any("error", updateErr))
				return model.ErrInternalServer
			}
		}
		return nil // コミット
	}) // トランザクション終了

	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return err
		}
		logger.Error("Transaction failed", slog.Any("error", err))
		return model.ErrInternalServer
	}

	// 関数名に合わせてログメッセージも変更
	logger.Info("Review outcome recorded successfully")
	return nil
}
