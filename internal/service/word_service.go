package service

import (
	"context"
	"errors"
	"time"

	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WordService インターフェース (変更なし)
type WordService interface {
	PostWord(ctx context.Context, tenantID uuid.UUID, req *model.PostWordRequest) (*model.Word, error)
	GetWord(ctx context.Context, tenantID, wordID uuid.UUID) (*model.Word, error)
	GetWords(ctx context.Context, tenantID uuid.UUID) ([]*model.Word, error)
	PutWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.PutWordRequest) (*model.Word, error)
	PatchWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.PatchWordRequest) (*model.Word, error)
	DeleteWord(ctx context.Context, tenantID, wordID uuid.UUID) error
}

// wordService 構造体から logger フィールドを削除
type wordService struct {
	db       *gorm.DB
	wordRepo repository.WordRepository
	progRepo repository.ProgressRepository
}

// NewWordService コンストラクタから logger 引数を削除
func NewWordService(db *gorm.DB, wordRepo repository.WordRepository, progRepo repository.ProgressRepository) WordService {
	return &wordService{
		db:       db,
		wordRepo: wordRepo,
		progRepo: progRepo,
	}
}

func (s *wordService) PostWord(ctx context.Context, tenantID uuid.UUID, req *model.PostWordRequest) (*model.Word, error) {
	logger := middleware.GetLogger(ctx)
	var createdWord *model.Word

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		exists, err := s.wordRepo.CheckTermExists(ctx, tx, tenantID, req.Term, nil)
		if err != nil {
			logger.Error("Failed to check term existence", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部でエラーが発生しました。", "", err)
		}
		if exists {
			logger.Info("Term already exists", "term", req.Term)
			return model.NewAppError("DUPLICATE_TERM", "その単語は既に使用されています。", "term", model.ErrConflict)
		}

		word := &model.Word{
			WordID:     uuid.New(),
			TenantID:   tenantID,
			Term:       req.Term,
			Definition: req.Definition,
		}
		if err := s.wordRepo.Create(ctx, tx, word); err != nil {
			logger.Error("Failed to create word", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "単語の作成に失敗しました。", "", err)
		}

		progress := &model.LearningProgress{
			ProgressID:     uuid.New(),
			TenantID:       tenantID,
			WordID:         word.WordID,
			Level:          model.Level1,
			NextReviewDate: time.Now(),
		}
		if err := s.progRepo.Create(ctx, tx, progress); err != nil {
			logger.Error("Failed to create initial learning progress", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "学習進捗の作成に失敗しました。", "", err)
		}

		createdWord = word
		return nil
	})

	if err != nil {
		return nil, err // トランザクション内で返されたエラー(AppError)をそのまま返す
	}

	logger.Info("Word created successfully", "word_id", createdWord.WordID)
	return createdWord, nil
}

func (s *wordService) GetWord(ctx context.Context, tenantID, wordID uuid.UUID) (*model.Word, error) {
	logger := middleware.GetLogger(ctx)
	word, err := s.wordRepo.FindByID(ctx, s.db, tenantID, wordID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, model.NewAppError("NOT_FOUND", "指定された単語は見つかりませんでした。", "word_id", model.ErrNotFound)
		}
		logger.Error("Failed to get word", "error", err)
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "単語の取得に失敗しました。", "", err)
	}
	return word, nil
}

func (s *wordService) GetWords(ctx context.Context, tenantID uuid.UUID) ([]*model.Word, error) {
	logger := middleware.GetLogger(ctx)
	words, err := s.wordRepo.FindByTenant(ctx, s.db, tenantID)
	if err != nil {
		logger.Error("Failed to get words", "error", err)
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "単語リストの取得に失敗しました。", "", err)
	}
	return words, nil
}

func (s *wordService) PutWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.PutWordRequest) (*model.Word, error) {
	logger := middleware.GetLogger(ctx)
	var updatedWord *model.Word

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		word, err := s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return model.NewAppError("NOT_FOUND", "更新対象の単語が見つかりませんでした。", "word_id", model.ErrNotFound)
			}
			logger.Error("Failed to find word for update", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "単語の取得に失敗しました。", "", err)
		}

		updates := make(map[string]interface{})
		if req.Term != word.Term {
			exists, checkErr := s.wordRepo.CheckTermExists(ctx, tx, tenantID, req.Term, &wordID)
			if checkErr != nil {
				return model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部でエラーが発生しました。", "", checkErr)
			}
			if exists {
				return model.NewAppError("DUPLICATE_TERM", "その単語は既に使用されています。", "term", model.ErrConflict)
			}
			updates["Term"] = req.Term
		}
		if req.Definition != word.Definition {
			updates["Definition"] = req.Definition
		}

		if len(updates) > 0 {
			if updateErr := s.wordRepo.Update(ctx, tx, tenantID, wordID, updates); updateErr != nil {
				return model.NewAppError("INTERNAL_SERVER_ERROR", "単語の更新に失敗しました。", "", updateErr)
			}
		}

		updatedWord, err = s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "更新後の単語の取得に失敗しました。", "", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.Info("Word updated successfully (PUT)", "word_id", updatedWord.WordID)
	return updatedWord, nil
}

func (s *wordService) PatchWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.PatchWordRequest) (*model.Word, error) {
	logger := middleware.GetLogger(ctx)
	var patchedWord *model.Word

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		word, err := s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return model.NewAppError("NOT_FOUND", "更新対象の単語が見つかりませんでした。", "word_id", model.ErrNotFound)
			}
			return model.NewAppError("INTERNAL_SERVER_ERROR", "単語の取得に失敗しました。", "", err)
		}

		updates := make(map[string]interface{})
		if req.Term != nil && *req.Term != word.Term {
			exists, checkErr := s.wordRepo.CheckTermExists(ctx, tx, tenantID, *req.Term, &wordID)
			if checkErr != nil {
				return model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部でエラーが発生しました。", "", checkErr)
			}
			if exists {
				return model.NewAppError("DUPLICATE_TERM", "その単語は既に使用されています。", "term", model.ErrConflict)
			}
			updates["Term"] = *req.Term
		}
		if req.Definition != nil && *req.Definition != word.Definition {
			updates["Definition"] = *req.Definition
		}

		if len(updates) > 0 {
			if err := s.wordRepo.Update(ctx, tx, tenantID, wordID, updates); err != nil {
				return model.NewAppError("INTERNAL_SERVER_ERROR", "単語の更新に失敗しました。", "", err)
			}
		}

		patchedWord, err = s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "更新後の単語の取得に失敗しました。", "", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.Info("Word patched successfully", "word_id", patchedWord.WordID)
	return patchedWord, nil
}

func (s *wordService) DeleteWord(ctx context.Context, tenantID, wordID uuid.UUID) error {
	logger := middleware.GetLogger(ctx)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 関連する学習進捗も削除する必要がある
		// Wordを削除する前にProgressを削除する
		err := s.progRepo.DeleteByWordID(ctx, tx, tenantID, wordID) // ← このメソッドをprogRepoに追加する必要がある
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			logger.Error("Failed to delete learning progress", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "学習進捗の削除に失敗しました。", "", err)
		}

		err = s.wordRepo.Delete(ctx, tx, tenantID, wordID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return model.ErrNotFound // 冪等性のために、見つからなくてもエラーとしない
			}
			logger.Error("Failed to delete word", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "単語の削除に失敗しました。", "", err)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			// 冪等性: 見つからなくても成功扱い
			logger.Info("Word to delete not found, but operation is successful (idempotent)", "word_id", wordID)
			return nil
		}
		return err
	}

	logger.Info("Word and associated progress deleted successfully", "word_id", wordID)
	return nil
}
