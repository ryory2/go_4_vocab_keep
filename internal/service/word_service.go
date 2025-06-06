//go:generate mockery --name WordService --srcpkg go_4_vocab_keep/internal/service --output ./mocks --outpkg mocks --case=underscore
package service

import (
	"context"
	"errors"
	"log/slog" // slog パッケージをインポート

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

func (s *wordService) PostWord(ctx context.Context, tenantID uuid.UUID, req *model.PostWordRequest) (*model.Word, error) {
	var createdWord *model.Word
	operation := "PostWord" // ログ用の操作名

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 重複チェック
		exists, err := s.wordRepo.CheckTermExists(ctx, tx, tenantID, req.Term, nil)
		if err != nil {
			// slog でエラーログ
			s.logger.Error("トランザクション内でのTerm存在チェック中にエラー発生",
				slog.Any("error", err),
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("term", req.Term),
			)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部でエラーが発生しました。", "", model.ErrInternalServer)
		}
		if exists {
			// slog で情報ログ (ビジネスロジックによるエラー)
			s.logger.Info("重複エラー：Termが既に存在",
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("term", req.Term),
			)
			// ErrConflictの代わりに、詳細情報を持つAppErrorを返す
			return model.NewAppError(
				"DUPLICATE_TERM", // エラーコード
				"その単語は既に使用されています。", // クライアントに表示するメッセージ
				"term",            // エラーが発生したフィールド
				model.ErrConflict, // 根本のエラー種別
			)
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
			s.logger.Error("トランザクション内での単語作成中にエラー発生",
				slog.Any("error", err),
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", word.WordID.String()), // 生成したIDも記録
			)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部でエラーが発生しました。", "", model.ErrInternalServer)
		}

		createdWord = word
		return nil // コミット
	})

	if err != nil {
		return nil, err
	}

	// slog で成功ログ
	s.logger.Info("Word created successfully", // 成功ログは英語のまま
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", createdWord.WordID.String()),
	)
	return createdWord, nil
}

func (s *wordService) GetWord(ctx context.Context, tenantID, wordID uuid.UUID) (*model.Word, error) {
	word, err := s.wordRepo.FindByID(ctx, s.db, tenantID, wordID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, model.ErrNotFound
		}
		s.logger.Error("予期せぬエラー：単語の取得に失敗しました",
			slog.Any("error", err), // リポジトリから返ったエラー
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return nil, model.ErrInternalServer

	}
	return word, nil
}

func (s *wordService) GetWords(ctx context.Context, tenantID uuid.UUID) ([]*model.Word, error) {
	words, err := s.wordRepo.FindByTenant(ctx, s.db, tenantID)
	if err != nil {
		// slog でエラーログ (リポジトリでもログされている可能性あり)
		s.logger.Error("単語リストの取得中にエラー発生",
			slog.Any("error", err),
			slog.String("tenant_id", tenantID.String()),
		)
		return nil, model.ErrInternalServer
	}
	return words, nil
}

func (s *wordService) PutWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.PutWordRequest) (*model.Word, error) {
	var updatedWord *model.Word
	operation := "WordService.PutWord" // ログ用に操作名を定義

	// --- トランザクション開始 ---
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 存在確認
		word, err := s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			// エラーが ErrNotFound でない場合のみ内部エラーとしてログ出力
			if !errors.Is(err, model.ErrNotFound) {
				s.logger.ErrorContext(ctx, "予期せぬエラー",
					slog.Any("error", err),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
				// FindByID が返す内部エラーは ErrInternalServer にラップするのが一般的
				return model.ErrInternalServer
			}
			// ErrNotFound はクライアントに伝えるべき情報なのでそのまま返す
			s.logger.InfoContext(ctx, "検索エラー：更新対象の単語なし",
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
			return model.ErrNotFound
		}

		// 2. 更新内容の準備と重複チェック
		// PUT なのでリクエストのフィールドで完全に上書きする。
		// PutWordRequest のフィールドは string であり、`validate:"required"` が付与されているため、
		// ここに到達する時点で req.Term と req.Definition が空文字でないことは
		// バリデーション層で保証されていると仮定する。

		updates := make(map[string]interface{})
		performUpdate := false // 変更があったかどうかのフラグ

		// Term の更新チェックと重複確認
		// PutWordRequest.Term は string 型なので nil チェックは不要。
		if req.Term != word.Term { // 既存の値と異なる場合のみ処理
			// Term が変更される場合は重複チェックを行う (自分自身を除外)
			exists, checkErr := s.wordRepo.CheckTermExists(ctx, tx, tenantID, req.Term, &wordID)
			if checkErr != nil {
				s.logger.ErrorContext(ctx, "予期せぬエラー",
					slog.Any("error", checkErr),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("new_term", req.Term),
				)
				return model.ErrInternalServer // 内部エラー
			}
			if exists {
				s.logger.InfoContext(ctx, "重複エラー：単語が存在",
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
					slog.String("new_term", req.Term),
				)
				return model.ErrConflict // 重複エラー
			}
			updates["Term"] = req.Term // ポインタではないので直接代入
			performUpdate = true
		} else {
			// Term が変更されない場合でも、Definition が変更される可能性があるので続ける
		}

		// Definition の更新チェック
		// PutWordRequest.Definition は string 型なので nil チェックは不要。
		if req.Definition != word.Definition { // 既存の値と異なる場合のみ処理
			updates["Definition"] = req.Definition // ポインタではないので直接代入
			performUpdate = true
		}

		// 3. 更新実行
		if performUpdate { // 何か変更がある場合のみDB更新 (DB負荷軽減のため)
			if updateErr := s.wordRepo.Update(ctx, tx, tenantID, wordID, updates); updateErr != nil {
				// 更新時に ErrNotFound が返る可能性も考慮 (例: 削除との競合)
				if errors.Is(updateErr, model.ErrNotFound) {
					// 存在チェック後、更新前に削除された場合など
					s.logger.WarnContext(ctx, "検索エラー：単語が見つからない",
						slog.String("operation", operation),
						slog.String("tenant_id", tenantID.String()),
						slog.String("word_id", wordID.String()),
					)
					return model.ErrNotFound // 更新対象が見つからなかった
				}
				// その他のリポジトリ層エラー
				s.logger.ErrorContext(ctx, "予期せぬエラー",
					slog.Any("error", updateErr),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
				// リポジトリ層の内部エラーは ErrInternalServer にラップ
				return model.ErrInternalServer
			}
			s.logger.InfoContext(ctx, "単語更新完了（変更あり）",
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
				slog.Any("updates", updates), // 更新内容をログ（機密情報に注意）
			)
		} else {
			// 変更がない場合もログ出力
			s.logger.InfoContext(ctx, "単語更新完了（変更なし）",
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
			// 注意: 厳密なPUTでは変更がなくてもUpdatedAtを更新する場合があるが、
			// GORMのUpdateは変更がないとSQLを発行しないことが多い。
			// 仕様として変更なくてもUpdatedAtを更新したい場合は別途考慮が必要。
		}

		// 4. 更新後のデータを取得（変更がなくても現在の状態を取得して返す）
		// 注意: performUpdate が false でも最新の状態を返すために FindByID を呼ぶ
		var fetchErr error
		updatedWord, fetchErr = s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if fetchErr != nil {
			// 更新直後に見つからないケース (例: ほぼ同時に削除された)
			if errors.Is(fetchErr, model.ErrNotFound) {
				s.logger.ErrorContext(ctx, "検索エラー：単語なし",
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
				// このケースは内部的な問題の可能性が高い
				return model.ErrInternalServer
			}
			// その他の取得エラー
			s.logger.ErrorContext(ctx, "予期せぬエラー",
				slog.Any("error", fetchErr),
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
			return model.ErrInternalServer
		}

		// トランザクション成功
		return nil // ここで nil を返すとトランザクションがコミットされる
	})
	// --- トランザクション終了 ---

	// 5. エラーハンドリング (トランザクション後)
	if err != nil {
		// トランザクション関数内で適切にラップまたは分類されたエラーが返ってくる想定
		// (ErrNotFound, ErrConflict, ErrInternalServer)
		// ここでは追加のログは不要かもしれない（トランザクション内でログ済みのため）
		// そのままエラーを返す
		return nil, err
	}

	// 6. 成功レスポンス
	s.logger.InfoContext(ctx, "単語更新（PUT）成功",
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", updatedWord.WordID.String()), // 取得した最新データのIDを使う
	)
	return updatedWord, nil
}

// --- ここから PatchWord メソッドの実装を追記 ---

// PatchWord は PATCH (部分更新) のためのメソッド
// req のフィールドが nil でない場合のみ、そのフィールドを更新対象とする
// model.PatchWordRequest はフィールドがポインタ型 (*string) であると想定
func (s *wordService) PatchWord(ctx context.Context, tenantID, wordID uuid.UUID, req *model.PatchWordRequest) (*model.Word, error) {
	var patchedWord *model.Word
	operation := "PatchWord"

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 存在確認
		word, err := s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if err != nil {
			// エラーログは FindByID 内か、ここで必要に応じて出す
			if !errors.Is(err, model.ErrNotFound) {
				s.logger.Error("部分更新対象単語の検索中にエラー発生 (トランザクション内)",
					slog.Any("error", err),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
			}
			return err // ErrNotFound または ErrInternalServer
		}

		// 2. 更新内容の準備 (PATCH: リクエストで指定されたフィールドのみ)
		// キーが string 型で、値が interface{} 型のマップ
		// interface{}（空インターフェース）はGoの特別な型で、任意の型の値を格納できる
		updates := make(map[string]interface{})
		performUpdate := false // 実際に変更があるかどうかのフラグ

		// Term の更新チェック
		if req.Term != nil { // リクエストで Term が指定されているか
			if *req.Term == "" {
				s.logger.Warn("Term を空文字に更新しようとしています",
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
				// 空文字を許可しない場合はここでエラーを返すこともできる
				// return model.ErrInvalidInput // 例
				// 空文字を許可する場合は重複チェックへ進む
			}

			if *req.Term != word.Term { // 既存の値と異なる場合のみ更新
				// Term の重複チェック
				exists, checkErr := s.wordRepo.CheckTermExists(ctx, tx, tenantID, *req.Term, &wordID)
				if checkErr != nil {
					s.logger.Error("部分更新時のTerm存在チェック中にエラー発生 (トランザクション内)",
						slog.Any("error", checkErr),
						slog.String("operation", operation),
						slog.String("tenant_id", tenantID.String()),
						slog.String("new_term", *req.Term),
					)
					return model.ErrInternalServer
				}
				if exists {
					s.logger.Info("部分更新後のTermが既存のTermと重複します",
						slog.String("operation", operation),
						slog.String("tenant_id", tenantID.String()),
						slog.String("word_id", wordID.String()),
						slog.String("new_term", *req.Term),
					)
					return model.ErrConflict
				}
				// マップ型変数["キー"]で参照
				updates["Term"] = *req.Term
				performUpdate = true
			}
		}

		// Definition の更新チェック
		if req.Definition != nil { // リクエストで Definition が指定されているか
			if *req.Definition == "" {
				s.logger.Warn("Definition を空文字に更新しようとしています",
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
				// 空文字を許可しない場合はエラーを返すことも可能
				// return model.ErrInvalidInput
			}
			if *req.Definition != word.Definition { // 既存の値と異なる場合のみ更新
				updates["Definition"] = *req.Definition
				performUpdate = true
			}
		}

		// 3. 更新実行
		if performUpdate { // 何か変更がある場合のみDB更新
			if updateErr := s.wordRepo.Update(ctx, tx, tenantID, wordID, updates); updateErr != nil {
				// リポジトリ層で ErrNotFound が返る可能性も考慮
				if !errors.Is(updateErr, model.ErrNotFound) {
					s.logger.Error("単語の部分更新処理中にエラー発生 (トランザクション内)",
						slog.Any("error", updateErr),
						slog.String("operation", operation),
						slog.String("tenant_id", tenantID.String()),
						slog.String("word_id", wordID.String()),
					)
				}
				return updateErr // model.ErrNotFound or model.ErrInternalServer
			}
		} else {
			s.logger.Info("No actual changes detected for patch update", // ログメッセージをPatch用に変更
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
		}

		// 更新後のデータを取得（変更がなくても取得して返すのが一般的）
		var fetchErr error
		patchedWord, fetchErr = s.wordRepo.FindByID(ctx, tx, tenantID, wordID)
		if fetchErr != nil {
			// ここで fetchErr が ErrNotFound の場合、更新中に削除された等の競合か？
			s.logger.Error("部分更新後の単語データ取得中にエラー発生 (トランザクション内)",
				slog.Any("error", fetchErr),
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
			return fetchErr // FindByID が返すエラー (ErrNotFound or ErrInternalServer)
		}

		return nil // コミット
	})

	if err != nil {
		// トランザクション内で発生したハンドリング済みのエラー
		if errors.Is(err, model.ErrNotFound) || errors.Is(err, model.ErrConflict) || errors.Is(err, model.ErrInvalidInput) {
			// 既にログされているか、ビジネスロジックエラー
			return nil, err
		}
		// トランザクション自体のエラーなど、予期せぬエラー
		s.logger.Error("単語部分更新トランザクションが失敗しました",
			slog.Any("error", err),
			slog.String("operation", operation),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return nil, model.ErrInternalServer
	}

	// 成功ログ
	s.logger.Info("Word patched successfully", // ログメッセージをPatch用に変更
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", patchedWord.WordID.String()),
	)
	return patchedWord, nil
}

// --- PatchWord メソッドの実装ここまで ---

func (s *wordService) DeleteWord(ctx context.Context, tenantID uuid.UUID, wordID uuid.UUID) error {
	operation := "DeleteWord"
	// バリデーション
	if tenantID == uuid.Nil || wordID == uuid.Nil {
		// slog で警告ログ
		s.logger.Warn("無効なUUIDで単語削除が呼び出されました",
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
					s.logger.Info("削除対象の単語が見つからないか、既に削除されています",
						slog.String("operation", operation),
						slog.String("tenant_id", tenantID.String()),
						slog.String("word_id", wordID.String()),
					)
					return model.ErrNotFound // トランザクション関数からはErrNotFoundを返す
				}
				// slog でエラーログ (リポジトリでもログされている可能性あり)
				s.logger.Error("リポジトリでの単語削除中にエラー発生 (トランザクション内)",
					slog.Any("error", deleteErr),
					slog.String("operation", operation),
					slog.String("tenant_id", tenantID.String()),
					slog.String("word_id", wordID.String()),
				)
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
			s.logger.Info("単語削除操作は対象が見つからず終了しました (冪等)",
				slog.String("operation", operation),
				slog.String("tenant_id", tenantID.String()),
				slog.String("word_id", wordID.String()),
			)
			return nil // クライアントには成功として返す or ErrNotFoundを返すかは仕様次第 (ここではnil)
		}
		// トランザクション制御自体のエラーなど、予期せぬエラー
		s.logger.Error("単語削除トランザクションが失敗しました",
			slog.Any("error", err),
			slog.String("operation", operation),
			slog.String("tenant_id", tenantID.String()),
			slog.String("word_id", wordID.String()),
		)
		return model.ErrInternalServer
	}

	// 全て成功 (またはNotFoundで正常終了扱い)
	s.logger.Info("Word deleted successfully (or was already deleted)", // 成功ログは英語のまま
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
		slog.String("word_id", wordID.String()),
	)
	return nil
}
