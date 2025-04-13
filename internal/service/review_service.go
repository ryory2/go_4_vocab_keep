// internal/service/review_service.go
package service

import (
	"context"
	"errors"
	"log"
	"time"

	"go_1_test_repository/internal/config"
	"go_1_test_repository/internal/model"
	"go_1_test_repository/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReviewService interface {
	GetReviewWords(ctx context.Context, tenantID uuid.UUID) ([]*model.ReviewWordResponse, error)
	SubmitReviewResult(ctx context.Context, tenantID, wordID uuid.UUID, isCorrect bool) error
}

type reviewService struct {
	db       *gorm.DB // トランザクション用にDB接続を持つ
	progRepo repository.ProgressRepository
	cfg      config.Config // ReviewLimit を使う
}

func NewReviewService(db *gorm.DB, progRepo repository.ProgressRepository, cfg config.Config) ReviewService {
	return &reviewService{
		db:       db,
		progRepo: progRepo,
		cfg:      cfg,
	}
}

func (s *reviewService) GetReviewWords(ctx context.Context, tenantID uuid.UUID) ([]*model.ReviewWordResponse, error) {
	now := time.Now()
	// リポジトリにDB接続を渡す
	progresses, err := s.progRepo.FindReviewableByTenant(ctx, s.db, tenantID, now, s.cfg.App.ReviewLimit)
	if err != nil {
		log.Printf("Error finding reviewable words: %v", err)
		return nil, model.ErrInternalServer
	}

	// レスポンスDTOに変換 (PreloadしたWord情報を使う)
	responses := make([]*model.ReviewWordResponse, 0, len(progresses))
	for _, p := range progresses {
		// PreloadしたWordがnilでないことを確認 (理論上Joinsで絞っているのでnilにはならないはず)
		if p.Word == nil {
			log.Printf("Warning: Found progress ID %s with nil Word during review generation.", p.ProgressID)
			continue // Word情報がないものはスキップ
		}
		responses = append(responses, &model.ReviewWordResponse{
			WordID:     p.WordID,
			Term:       p.Word.Term,
			Definition: p.Word.Definition,
			Level:      p.Level,
		})
	}

	return responses, nil
}

func (s *reviewService) SubmitReviewResult(ctx context.Context, tenantID, wordID uuid.UUID, isCorrect bool) error {

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 該当の学習進捗を取得 (トランザクション内で)
		// FindByWordIDもtxを受け取るように変更するか、ここで直接クエリ実行
		var progress *model.LearningProgress
		result := tx.Preload("Word").Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&progress)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				log.Printf("Progress not found for word %s tenant %s during submit", wordID, tenantID)
				return model.ErrNotFound
			}
			log.Printf("Error finding progress by word ID in transaction: %v", result.Error)
			return model.ErrInternalServer
		}
		// Wordが論理削除されていないかもチェック
		if progress.Word == nil || progress.Word.DeletedAt.Valid {
			log.Printf("Word (ID: %s) associated with progress is deleted or missing.", wordID)
			return model.ErrNotFound // 対象の単語が存在しない
		}

		// 2. レベルと次回レビュー日を計算
		now := time.Now()
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
				progress.Level = model.Level3
				progress.NextReviewDate = now.AddDate(0, 0, 7) // Or longer
			default:
				log.Printf("Warning: Invalid progress level %d found for progress %s. Resetting to Level 1.", progress.Level, progress.ProgressID)
				progress.Level = model.Level1
				progress.NextReviewDate = now.AddDate(0, 0, 1)
			}
		} else {
			progress.Level = model.Level1
			progress.NextReviewDate = now.AddDate(0, 0, 1)
		}

		// 3. 進捗を更新
		if err := s.progRepo.Update(ctx, tx, progress); err != nil { // リポジトリメソッドも tx を受け取る
			log.Printf("Error updating progress in transaction: %v", err)
			return model.ErrInternalServer
		}

		return nil // コミット
	})

	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return err
		}
		log.Printf("Transaction failed for SubmitReviewResult: %v", err)
		return model.ErrInternalServer
	}

	return nil
}
