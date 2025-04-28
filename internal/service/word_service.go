// internal/service/word_service.go
package service

import (
	"context"
	"errors"
	"log/slog" // slog パッケージをインポート
	"time"

	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WordService インターフェース (変更なし)
type WordService interface {
	CreateWord(ctx context.Context, tenantID uuid.UUID, req *model.CreateWordRequest) (*model.Word, error)
	GetWord(ctx context.Context, tenantID, wordID uuid.UUID) (*model.Word, error)
	ListWords(ctx context.Context, tenantID uuid.UUID) ([]*model.Word, error)
	UpdateWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.UpdateWordRequest) (*model.Word, error)
	DeleteWord(ctx context.Context, tenantID, wordID uuid.UUID) error
}

// wordService 構造体に logger フィールドを追加
type wordService struct {
	db       *gorm.DB
	wordRepo repository.WordRepository
	progRepo repository.ProgressRepository
	logger   *slog.Logger // slog.Logger フィールドを追加
}

// NewWordService コンストラクタで logger を受け取るように変更
func NewWordService(db *gorm.DB, wordRepo repository.WordRepository, progRepo repository.ProgressRepository, logger *slog.Logger) WordService { // logger を引数に追加
	// ロガーが nil の場合にデフォルトロガーを使用
	if logger == nil {
		logger = slog.Default()
	}
	return &wordService{
		db:       db,
		wordRepo: wordRepo,
		progRepo: progRepo,
		logger:   logger, // logger を設定
	}
}

func (s *wordService) CreateWord(ctx context.Context, tenantID uuid.UUID, req *model.CreateWordRequest) (*model.Word, error) {
	if req.Term == "" || req.Definition == "" {
		// slog で警告ログ (クライアント入力エラー)
		s.logger.Warn("CreateWord called with empty term or definition",
			slog.String("tenant_id", tenantID.String()),
			slog.Any("request", req), // 注意: リクエスト内容に機密情報がないか確認
		)
		return nil, model.ErrInvalidInput
	}

	var createdWord *model.Word
	operation := "CreateWord" // ログ用の操作名

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 重複チェック
		exists, err := s.wordRepo.CheckTermExists(ctx, tx, tenantID, req.Term, nil)
		if err != nil {
			// slog でエラーログ
			s.logger.Error("Error checking term existence in transaction",
				slog.Any("error", err),
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("term", req.Term),
			)
			return model.ErrInternalServer
		}
		if exists {
			// slog で情報ログ (ビジネスロジックによるエラー)
			s.logger.Info("Term already exists, conflict detected",
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("term", req.Term),
			)
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
			// slog でエラーログ
			s.logger.Error("Error creating word in transaction",
				slog.Any("error", err),
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", word.WordID.String()), // 生成したIDも記録
			)
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
			// slog でエラーログ
			// ★ 修正点: logCtx を使わず、slog.Attr を直接渡す
			s.logger.Error("Error creating progress in transaction",
				slog.Any("error", err),
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", word.WordID.String()),
				slog.String("progress_id", progress.ProgressID.String()),
			)

			if errors.Is(err, gorm.ErrDuplicatedKey) { // 制約違反の場合
				// ★ 修正点: logCtx を使わず、slog.Attr を直接渡す
				s.logger.Warn("Conflict detected while creating progress (likely duplicate)",
					slog.Any("error", err),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", word.WordID.String()),
					slog.String("progress_id", progress.ProgressID.String()),
				)
				return model.ErrConflict
			}
			return model.ErrInternalServer
		}

		createdWord = word
		return nil // コミット
	})

	if err != nil {
		// トランザクション内で返されたエラー
		if errors.Is(err, model.ErrConflict) || errors.Is(err, model.ErrInvalidInput) {
			// 既にログされているか、ビジネスロジックエラーなので追加ログ不要
			return nil, err
		}
		// slog でトランザクション全体のエラーログ
		s.logger.Error("Transaction failed",
			slog.Any("error", err),
			slog.String("operation", operation),
			slog.String("tenant_id", tenantID.String()),
		)
		// リポジトリ層で予期せぬエラーがあれば既にログされているはずだが、
		// トランザクション制御自体のエラーの可能性もあるため記録
		return nil, model.ErrInternalServer
	}

	// slog で成功ログ
	s.logger.Info("Word created successfully",
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", createdWord.WordID.String()),
	)
	return createdWord, nil
}

func (s *wordService) GetWord(ctx context.Context, tenantID, wordID uuid.UUID) (*model.Word, error) {
	word, err := s.wordRepo.FindByID(ctx, s.db, tenantID, wordID)
	if err != nil {
		// エラーはリポジトリで変換済み、ログもリポジトリ層かここで必要なら出す
		// ErrNotFound は通常ログ不要、ErrInternalServerはリポジトリでログされているはず
		if !errors.Is(err, model.ErrNotFound) {
			s.logger.Error("Failed to get word",
				slog.Any("error", err), // リポジトリから返ったエラー
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
		}
		return nil, err
	}
	return word, nil
}

func (s *wordService) ListWords(ctx context.Context, tenantID uuid.UUID) ([]*model.Word, error) {
	words, err := s.wordRepo.FindByTenant(ctx, s.db, tenantID)
	if err != nil {
		// slog でエラーログ (リポジトリでもログされている可能性あり)
		s.logger.Error("Error listing words",
			slog.Any("error", err),
			slog.String("tenant_id", tenantID.String()),
		)
		// FindByTenant は ErrNotFound を返さないので、エラーは基本的に ErrInternalServer
		return nil, model.ErrInternalServer
	}
	return words, nil
}

func (s *wordService) UpdateWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.UpdateWordRequest) (*model.Word, error) {
	var updatedWord *model.Word
	operation := "UpdateWord"

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 存在確認
		word, err := s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			// ErrNotFound や ErrInternalServer はリポジトリからそのまま返す
			// 必要ならここで追加ログ
			if !errors.Is(err, model.ErrNotFound) {
				s.logger.Error("Error finding word for update in transaction",
					slog.Any("error", err),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
			}
			return err
		}

		// 2. 更新内容の準備と重複チェック
		updates := make(map[string]interface{})
		performUpdate := false
		if req.Term != nil && *req.Term != "" && *req.Term != word.Term {
			exists, checkErr := s.wordRepo.CheckTermExists(ctx, tx, tenantID, *req.Term, &wordID)
			if checkErr != nil {
				// slog でエラーログ
				s.logger.Error("Error checking term existence during update in transaction",
					slog.Any("error", checkErr),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("new_term", *req.Term),
				)
				return model.ErrInternalServer
			}
			if exists {
				// slog で情報ログ (ビジネスロジックエラー)
				s.logger.Info("New term conflicts with existing term",
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
					slog.String("new_term", *req.Term),
				)
				return model.ErrConflict
			}
			updates["Term"] = *req.Term
			performUpdate = true
		}
		if req.Definition != nil && *req.Definition != "" && *req.Definition != word.Definition {
			updates["Definition"] = *req.Definition
			performUpdate = true
		}

		// 3. 更新実行
		if performUpdate {
			if updateErr := s.wordRepo.Update(ctx, tx, tenantID, wordID, updates); updateErr != nil {
				// ErrNotFound や ErrInternalServer はリポジトリからそのまま返す
				// 必要ならここで追加ログ
				if !errors.Is(updateErr, model.ErrNotFound) {
					s.logger.Error("Error updating word in transaction",
						slog.Any("error", updateErr),
						slog.String("operation", operation),
						slog.String("tenant_id", tenantID.String()),
						slog.String("word_id", wordID.String()),
					)
				}
				return updateErr
			}
		} else {
			// slog で情報ログ (更新内容がなかった)
			s.logger.Info("No actual changes detected for update",
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
		}

		// 更新後のデータを取得
		var fetchErr error
		updatedWord, fetchErr = s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if fetchErr != nil {
			// slog でエラーログ
			s.logger.Error("Error fetching updated word in transaction",
				slog.Any("error", fetchErr),
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
			// 更新自体は成功したかもしれないが、結果を返せないためエラーとする
			return model.ErrInternalServer
		}

		return nil // コミット
	})

	if err != nil {
		if errors.Is(err, model.ErrNotFound) || errors.Is(err, model.ErrConflict) || errors.Is(err, model.ErrInvalidInput) {
			// 既にログされているか、ビジネスロジックエラー
			return nil, err
		}
		// slog でトランザクション全体のエラーログ
		s.logger.Error("Transaction failed",
			slog.Any("error", err),
			slog.String("operation", operation),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return nil, model.ErrInternalServer
	}

	// slog で成功ログ
	s.logger.Info("Word updated successfully",
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", updatedWord.WordID.String()),
	)
	return updatedWord, nil
}

func (s *wordService) DeleteWord(ctx context.Context, tenantID uuid.UUID, wordID uuid.UUID) error {
	operation := "DeleteWord"
	// バリデーション
	if tenantID == uuid.Nil || wordID == uuid.Nil {
		// slog で警告ログ
		s.logger.Warn("DeleteWord called with invalid UUID",
			slog.String("operation", operation),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return model.ErrInvalidInput
	}

	// トランザクションを開始
	err := s.db.WithContext(ctx).Transaction(
		func(tx *gorm.DB) error {
			deleteErr := s.wordRepo.Delete(ctx, tx, tenantID, wordID)
			if deleteErr != nil {
				if errors.Is(deleteErr, model.ErrNotFound) {
					// slog で情報ログ (正常系の範囲内)
					s.logger.Info("Word not found or already deleted during delete operation",
						slog.String("operation", operation),
						slog.String("tenant_id", tenantID.String()),
						slog.String("word_id", wordID.String()),
					)
					return model.ErrNotFound // トランザクション関数からはErrNotFoundを返す
				}
				// slog でエラーログ (リポジトリでもログされている可能性あり)
				s.logger.Error("Failed to delete word in repository within transaction",
					slog.Any("error", deleteErr),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
				// GORMなどのDBエラーは内部サーバーエラーとして扱う
				return model.ErrInternalServer
			}
			// 成功
			return nil // トランザクションコミット
		})

	// トランザクション全体のエラーハンドリング
	if err != nil {
		// トランザクション関数から返されたエラー(ErrNotFound, ErrInternalServerなど)
		if errors.Is(err, model.ErrNotFound) {
			// ErrNotFound は正常系として扱う（冪等性）
			s.logger.Info("Delete operation resulted in NotFound (idempotent)",
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
			return nil // クライアントには成功として返す or ErrNotFoundを返すかは仕様次第 (ここではnil)
		}
		// トランザクション制御自体のエラーなど、予期せぬエラー
		// (トランザクション内でログされているはずだが念のため)
		s.logger.Error("Transaction failed for DeleteWord",
			slog.Any("error", err),
			slog.String("operation", operation),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return model.ErrInternalServer
	}

	// 全て成功 (またはNotFoundで正常終了扱い)
	// slog で成功ログ
	s.logger.Info("Word deleted successfully (or was already deleted)",
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", wordID.String()),
	)
	return nil
}
