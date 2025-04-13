// internal/service/word_service.go
package service

import (
	"context"
	"errors"
	"log"
	"time"

	"go_1_test_repository/internal/model"
	"go_1_test_repository/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WordService interface {
	CreateWord(ctx context.Context, tenantID uuid.UUID, req *model.CreateWordRequest) (*model.Word, error)
	GetWord(ctx context.Context, tenantID, wordID uuid.UUID) (*model.Word, error)
	ListWords(ctx context.Context, tenantID uuid.UUID) ([]*model.Word, error)
	UpdateWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.UpdateWordRequest) (*model.Word, error)
	DeleteWord(ctx context.Context, tenantID, wordID uuid.UUID) error
}

type wordService struct {
	db       *gorm.DB // トランザクション用にDB接続を持つ
	wordRepo repository.WordRepository
	progRepo repository.ProgressRepository
}

func NewWordService(db *gorm.DB, wordRepo repository.WordRepository, progRepo repository.ProgressRepository) WordService {
	return &wordService{
		db:       db,
		wordRepo: wordRepo,
		progRepo: progRepo,
	}
}

func (s *wordService) CreateWord(ctx context.Context, tenantID uuid.UUID, req *model.CreateWordRequest) (*model.Word, error) {
	if req.Term == "" || req.Definition == "" {
		return nil, model.ErrInvalidInput
	}

	var createdWord *model.Word

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 重複チェック
		exists, err := s.wordRepo.CheckTermExists(ctx, tx, tenantID, req.Term, nil)
		if err != nil {
			log.Printf("Error checking term existence in transaction: %v", err)
			return model.ErrInternalServer
		}
		if exists {
			return model.ErrConflict // 重複エラー
		}

		// 2. 単語を作成
		word := &model.Word{
			WordID:     uuid.New(),
			TenantID:   tenantID,
			Term:       req.Term,
			Definition: req.Definition,
		}
		if err := s.wordRepo.Create(ctx, tx, word); err != nil {
			log.Printf("Error creating word in transaction: %v", err)
			return model.ErrInternalServer
		}

		// 3. 学習進捗を作成
		progress := &model.LearningProgress{
			ProgressID:     uuid.New(),
			TenantID:       tenantID,
			WordID:         word.WordID,
			Level:          model.Level1,
			NextReviewDate: time.Now().AddDate(0, 0, 1),
		}
		if err := s.progRepo.Create(ctx, tx, progress); err != nil {
			log.Printf("Error creating progress in transaction: %v", err)
			// GORMが制約違反エラーを返す可能性がある
			if errors.Is(err, gorm.ErrDuplicatedKey) { // これはドライバ依存の可能性あり
				return model.ErrConflict
			}
			return model.ErrInternalServer
		}

		createdWord = word
		return nil // コミット
	})

	if err != nil {
		// トランザクション内で返されたエラー
		if errors.Is(err, model.ErrConflict) {
			return nil, err // そのまま返す
		}
		log.Printf("Transaction failed for CreateWord: %v", err)
		return nil, model.ErrInternalServer
	}

	return createdWord, nil
}

func (s *wordService) GetWord(ctx context.Context, tenantID, wordID uuid.UUID) (*model.Word, error) {
	// サービス層でDB接続(s.db)を渡す
	word, err := s.wordRepo.FindByID(ctx, s.db, tenantID, wordID)
	if err != nil {
		// エラーはリポジトリで変換済みのはず
		return nil, err
	}
	return word, nil
}

func (s *wordService) ListWords(ctx context.Context, tenantID uuid.UUID) ([]*model.Word, error) {
	words, err := s.wordRepo.FindByTenant(ctx, s.db, tenantID)
	if err != nil {
		log.Printf("Error listing words: %v", err)
		return nil, model.ErrInternalServer
	}
	return words, nil
}

func (s *wordService) UpdateWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.UpdateWordRequest) (*model.Word, error) {
	var updatedWord *model.Word

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 存在確認 (トランザクション内でロックを取得する意味合いもある)
		word, err := s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			return err // model.ErrNotFound or model.ErrInternalServer
		}

		// 2. 更新内容の準備と重複チェック
		updates := make(map[string]interface{})
		performUpdate := false
		if req.Term != nil && *req.Term != "" && *req.Term != word.Term {
			exists, err := s.wordRepo.CheckTermExists(ctx, tx, tenantID, *req.Term, &wordID)
			if err != nil {
				log.Printf("Error checking term existence during update: %v", err)
				return model.ErrInternalServer
			}
			if exists {
				return model.ErrConflict
			}
			updates["Term"] = *req.Term
			performUpdate = true
		}
		if req.Definition != nil && *req.Definition != "" && *req.Definition != word.Definition {
			updates["Definition"] = *req.Definition
			performUpdate = true
		}

		// 3. 更新実行 (更新内容がある場合のみ)
		if performUpdate {
			if err := s.wordRepo.Update(ctx, tx, tenantID, wordID, updates); err != nil {
				log.Printf("Error updating word in transaction: %v", err)
				// Update内でErrNotFoundが返る可能性あり
				if errors.Is(err, model.ErrNotFound) {
					return model.ErrNotFound
				}
				return model.ErrInternalServer
			}
		}

		// 更新後のデータを取得 (トランザクション内で取得するのが確実)
		updatedWord, err = s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			log.Printf("Error fetching updated word in transaction: %v", err)
			return model.ErrInternalServer // 更新は成功したが取得に失敗
		}

		return nil // コミット
	})

	if err != nil {
		if errors.Is(err, model.ErrNotFound) || errors.Is(err, model.ErrConflict) {
			return nil, err
		}
		log.Printf("Transaction failed for UpdateWord: %v", err)
		return nil, model.ErrInternalServer
	}

	return updatedWord, nil
}

func (s *wordService) DeleteWord(ctx context.Context, tenantID, wordID uuid.UUID) error {
	// 論理削除もアトミックに行いたい場合はトランザクション内で実行
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 存在確認 (削除対象があるか)
		_, err := s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			return err // model.ErrNotFound or model.ErrInternalServer
		}

		// 削除実行
		if err := s.wordRepo.Delete(ctx, tx, tenantID, wordID); err != nil {
			log.Printf("Error deleting word in transaction: %v", err)
			// Delete内でErrNotFoundが返る可能性あり (並行処理で先に消された場合など)
			if errors.Is(err, model.ErrNotFound) {
				return model.ErrNotFound
			}
			return model.ErrInternalServer
		}
		return nil // コミット
	})

	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return err
		}
		log.Printf("Transaction failed for DeleteWord: %v", err)
		return model.ErrInternalServer
	}

	// 関連するProgressの扱いは現状維持 (Wordに紐づいたまま残る)
	// 必要ならここでProgressも削除または無効化する処理を追加

	return nil
}
