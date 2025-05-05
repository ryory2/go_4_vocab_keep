// internal/service/word_service_test.go
package service

import (
	"context"
	"errors"
	"io"       // io.Discard のためにインポート
	"log/slog" // slog をインポート
	"testing"
	"time"

	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository/mocks" // モックリポジトリのパス

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- テストヘルパー関数 ---
func setupTestDBWord() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// GORMのデフォルトのトランザクションを使うので、SkipDefaultTransaction は false のまま
	})
	if err != nil {
		panic("failed to connect database for testing: " + err.Error())
	}
	// 必要に応じてマイグレーションを実行
	// db.AutoMigrate(&model.Word{}, &model.LearningProgress{})
	return db
}

func setupWordServiceWithMocks() (WordService, *gorm.DB, *mocks.WordRepository, *mocks.ProgressRepository) {
	db := setupTestDBWord()
	mockWordRepo := new(mocks.WordRepository)
	mockProgRepo := new(mocks.ProgressRepository)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil)) // 出力を捨てるロガー
	wordService := NewWordService(db, mockWordRepo, mockProgRepo, testLogger)
	return wordService, db, mockWordRepo, mockProgRepo
}

// --- Test PostWord ---
func Test_wordService_PostWord(t *testing.T) {
	ctx := context.Background()
	wordService, _, mockWordRepo, mockProgRepo := setupWordServiceWithMocks()

	tenantID := uuid.New()
	testTerm := "test_term"
	testDefinition := "test_definition"

	tests := []struct {
		name      string
		req       *model.PostWordRequest
		setupMock func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository)
		wantErr   error
		wantWord  bool
	}{
		{
			name: "正常系: 単語の作成成功",
			req: &model.PostWordRequest{
				Term:       testTerm,
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				// 1. CheckTermExists (重複なし)
				wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
					Return(false, nil).Once()
				// 2. wordRepo.Create (成功)
				wordRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.Word")).
					Run(func(args mock.Arguments) {
						word := args.Get(2).(*model.Word)
						assert.Equal(t, tenantID, word.TenantID)
						assert.Equal(t, testTerm, word.Term)
						assert.Equal(t, testDefinition, word.Definition)
						assert.NotEqual(t, uuid.Nil, word.WordID)
					}).Return(nil).Once()
				// // 3. progRepo.Create (成功) - 元コードでコメントアウトされているためここもコメントアウト
				// progRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.LearningProgress")).
				// 	Run(func(args mock.Arguments) {
				// 		prog := args.Get(2).(*model.LearningProgress)
				// 		assert.Equal(t, tenantID, prog.TenantID)
				// 		assert.NotEqual(t, uuid.Nil, prog.WordID)
				// 		assert.Equal(t, model.Level1, prog.Level)
				// 		assert.WithinDuration(t, time.Now().AddDate(0, 0, 1), prog.NextReviewDate, time.Second*5)
				// 	}).Return(nil).Once()
			},
			wantErr:  nil,
			wantWord: true,
		},
		{
			name: "異常系: Termが空",
			req: &model.PostWordRequest{
				Term:       "",
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {},
			wantErr:   model.ErrInvalidInput,
			wantWord:  false,
		},
		{
			name: "異常系: Definitionが空",
			req: &model.PostWordRequest{
				Term:       testTerm,
				Definition: "",
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {},
			wantErr:   model.ErrInvalidInput,
			wantWord:  false,
		},
		{
			name: "異常系: Termが重複",
			req: &model.PostWordRequest{
				Term:       testTerm,
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
					Return(true, nil).Once()
			},
			wantErr:  model.ErrConflict,
			wantWord: false,
		},
		{
			name: "異常系: CheckTermExistsでDBエラー",
			req: &model.PostWordRequest{
				Term:       testTerm,
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
					Return(false, errors.New("db error on check")).Once()
			},
			wantErr:  model.ErrInternalServer,
			wantWord: false,
		},
		{
			name: "異常系: wordRepo.CreateでDBエラー",
			req: &model.PostWordRequest{
				Term:       testTerm,
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
					Return(false, nil).Once()
				wordRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.Word")).
					Return(errors.New("db error on create word")).Once()
			},
			wantErr:  model.ErrInternalServer,
			wantWord: false,
		},
		// { // LearningProgress作成部分が有効な場合のテストケース例
		// 	name: "異常系: progRepo.CreateでDBエラー",
		// 	req: &model.PostWordRequest{
		// 		Term:       testTerm,
		// 		Definition: testDefinition,
		// 	},
		// 	setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
		// 		wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
		// 			Return(false, nil).Once()
		// 		wordRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.Word")).
		// 			Return(nil).Once()
		// 		progRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.LearningProgress")).
		// 			Return(errors.New("db error on create progress")).Once()
		// 	},
		// 	wantErr:  model.ErrInternalServer,
		// 	wantWord: false,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モックのリセットと再設定
			mockWordRepo.Mock = mock.Mock{}
			mockProgRepo.Mock = mock.Mock{}
			if tt.setupMock != nil {
				tt.setupMock(mockWordRepo, mockProgRepo)
			}

			createdWord, err := wordService.PostWord(ctx, tenantID, tt.req)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, createdWord)
			} else {
				require.NoError(t, err)
				require.NotNil(t, createdWord)
				assert.Equal(t, tt.req.Term, createdWord.Term)
				assert.Equal(t, tt.req.Definition, createdWord.Definition)
				assert.Equal(t, tenantID, createdWord.TenantID)
				assert.NotEqual(t, uuid.Nil, createdWord.WordID)
				// 成功時の wantWord フラグは不要だったので削除 (assert で確認するため)
			}

			mockWordRepo.AssertExpectations(t)
			// LearningProgress 作成が有効な場合はこちらも AssertExpectations
			// mockProgRepo.AssertExpectations(t)
		})
	}
}

// --- Test GetWord ---
func Test_wordService_GetWord(t *testing.T) {
	ctx := context.Background()
	wordService, db, mockWordRepo, mockProgRepo := setupWordServiceWithMocks() // db はリポジトリに渡すため必要

	tenantID := uuid.New()
	wordID := uuid.New()
	expectedWord := &model.Word{
		WordID:     wordID,
		TenantID:   tenantID,
		Term:       "get_term",
		Definition: "get_def",
		CreatedAt:  time.Now().Add(-time.Hour),
		UpdatedAt:  time.Now().Add(-time.Hour),
	}

	tests := []struct {
		name      string
		inputWID  uuid.UUID
		setupMock func(m *mocks.WordRepository)
		wantErr   error
		wantWord  *model.Word
	}{
		{
			name:     "正常系: 単語取得成功",
			inputWID: wordID,
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, db, tenantID, wordID).
					Return(expectedWord, nil).Once()
			},
			wantErr:  nil,
			wantWord: expectedWord,
		},
		{
			name:     "異常系: 単語が見つからない",
			inputWID: wordID,
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, db, tenantID, wordID).
					Return(nil, model.ErrNotFound).Once()
			},
			wantErr:  model.ErrNotFound,
			wantWord: nil,
		},
		{
			name:     "異常系: リポジトリでDBエラー",
			inputWID: wordID,
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, db, tenantID, wordID).
					Return(nil, errors.New("internal server error")).Once() // リポジトリが返すエラー
			},
			wantErr:  model.ErrInternalServer, // サービスが ErrInternalServer を返すことを期待
			wantWord: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWordRepo.Mock = mock.Mock{}
			if tt.setupMock != nil {
				tt.setupMock(mockWordRepo)
			}

			word, err := wordService.GetWord(ctx, tenantID, tt.inputWID)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, word)
			} else {
				require.NoError(t, err)
				require.NotNil(t, word)
				assert.Equal(t, tt.wantWord, word)
			}

			mockWordRepo.AssertExpectations(t)
			mockProgRepo.AssertNotCalled(t, mock.Anything)
		})
	}
}

// --- Test GetWords ---
func Test_wordService_GetWords(t *testing.T) {
	ctx := context.Background()
	wordService, db, mockWordRepo, mockProgRepo := setupWordServiceWithMocks()

	tenantID := uuid.New()
	expectedWords := []*model.Word{
		{WordID: uuid.New(), TenantID: tenantID, Term: "term1", Definition: "def1"},
		{WordID: uuid.New(), TenantID: tenantID, Term: "term2", Definition: "def2"},
	}

	tests := []struct {
		name      string
		setupMock func(m *mocks.WordRepository)
		wantErr   error
		wantWords []*model.Word
		wantLen   int
	}{
		{
			name: "正常系: 複数件取得成功",
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByTenant", ctx, db, tenantID).
					Return(expectedWords, nil).Once()
			},
			wantErr:   nil,
			wantWords: expectedWords,
			wantLen:   2,
		},
		{
			name: "正常系: 0件取得成功",
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByTenant", ctx, db, tenantID).
					Return([]*model.Word{}, nil).Once()
			},
			wantErr:   nil,
			wantWords: []*model.Word{},
			wantLen:   0,
		},
		{
			name: "異常系: リポジトリでDBエラー",
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByTenant", ctx, db, tenantID).
					Return(nil, errors.New("db error on find by tenant")).Once()
			},
			wantErr:   model.ErrInternalServer, // サービスが変換する
			wantWords: nil,
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWordRepo.Mock = mock.Mock{}
			if tt.setupMock != nil {
				tt.setupMock(mockWordRepo)
			}

			words, err := wordService.GetWords(ctx, tenantID)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr) // GetWords は ErrInternalServer をそのまま返す想定
				assert.Nil(t, words)
			} else {
				require.NoError(t, err)
				require.NotNil(t, words)
				assert.Len(t, words, tt.wantLen)
				assert.Equal(t, tt.wantWords, words)
			}

			mockWordRepo.AssertExpectations(t)
			mockProgRepo.AssertNotCalled(t, mock.Anything)
		})
	}
}

// --- Test PutWord ---
func Test_wordService_PutWord(t *testing.T) {
	ctx := context.Background()
	wordService, _, mockWordRepo, mockProgRepo := setupWordServiceWithMocks() // db は Transaction 用

	tenantID := uuid.New()
	wordID := uuid.New()
	originalTerm := "original_term_put" // 他のテストと区別
	originalDef := "original_def_put"
	newTerm := "new_term_put"
	newDef := "new_def_put"

	originalWord := &model.Word{
		WordID:     wordID,
		TenantID:   tenantID,
		Term:       originalTerm,
		Definition: originalDef,
	}

	tests := []struct {
		name            string
		inputWID        uuid.UUID
		req             *model.PutWordRequest // 値型フィールドを持つDTO
		setupMock       func(m *mocks.WordRepository)
		wantErr         error
		wantUpdatedTerm string
		wantUpdatedDef  string
	}{
		{
			name:     "正常系: TermとDefinitionを更新 (PUT)",
			inputWID: wordID,
			req: &model.PutWordRequest{
				Term:       newTerm, // 値を直接渡す
				Definition: newDef,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm, "Definition": newDef}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
				updatedWord := &model.Word{WordID: wordID, TenantID: tenantID, Term: newTerm, Definition: newDef}
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(updatedWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: newTerm,
			wantUpdatedDef:  newDef,
		},
		{
			name:     "正常系: 更新内容が既存と同じ (PUT)",
			inputWID: wordID,
			req: &model.PutWordRequest{
				Term:       originalTerm, // 元と同じ値
				Definition: originalDef,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				// CheckTermExists や Update は呼ばれない
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: originalTerm,
			wantUpdatedDef:  originalDef,
		},
		{
			name:     "異常系: 更新対象が見つからない (PUT)",
			inputWID: wordID,
			req: &model.PutWordRequest{
				Term:       newTerm,
				Definition: newDef,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(nil, model.ErrNotFound).Once()
			},
			wantErr: model.ErrNotFound,
		},
		{
			name:     "異常系: 新しいTermが重複 (PUT)",
			inputWID: wordID,
			req: &model.PutWordRequest{
				Term:       newTerm,
				Definition: newDef,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(true, nil).Once()
			},
			wantErr: model.ErrConflict,
		},
		{
			name:     "異常系: UpdateでDBエラー (PUT)",
			inputWID: wordID,
			req: &model.PutWordRequest{
				Term:       newTerm,
				Definition: newDef,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm, "Definition": newDef}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(errors.New("db error on update")).Once()
			},
			wantErr: model.ErrInternalServer,
		},
		// バリデーションエラーのテストはサービス層ではなく、ハンドラー層やバリデーションミドルウェアで行うのが一般的
		// {
		// 	name:     "異常系: Term が空 (バリデーション想定)",
		// 	inputWID: wordID,
		// 	req: &model.PutWordRequest{
		// 		Term:       "",
		// 		Definition: newDef,
		// 	},
		// 	setupMock: nil, // バリデーションで弾かれる想定
		// 	wantErr: model.ErrInvalidInput,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWordRepo.Mock = mock.Mock{}
			if tt.setupMock != nil {
				tt.setupMock(mockWordRepo)
			}

			updatedWord, err := wordService.PutWord(ctx, tenantID, tt.inputWID, tt.req)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, updatedWord)
			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedWord)
				assert.Equal(t, tt.wantUpdatedTerm, updatedWord.Term)
				assert.Equal(t, tt.wantUpdatedDef, updatedWord.Definition)
				assert.Equal(t, tenantID, updatedWord.TenantID)
				assert.Equal(t, tt.inputWID, updatedWord.WordID)
			}

			mockWordRepo.AssertExpectations(t)
			mockProgRepo.AssertNotCalled(t, mock.Anything)
		})
	}
}

// --- Test PatchWord ---
func Test_wordService_PatchWord(t *testing.T) {
	ctx := context.Background()
	wordService, _, mockWordRepo, mockProgRepo := setupWordServiceWithMocks() // db は Transaction 用に必要

	tenantID := uuid.New()
	wordID := uuid.New()
	originalTerm := "original_term_patch" // 他のテストと区別
	originalDef := "original_def_patch"
	newTerm := "new_term_patch"
	newDef := "new_def_patch"
	emptyStr := ""

	originalWord := &model.Word{
		WordID:     wordID,
		TenantID:   tenantID,
		Term:       originalTerm,
		Definition: originalDef,
		CreatedAt:  time.Now().Add(-time.Hour),
		UpdatedAt:  time.Now().Add(-time.Hour),
	}

	tests := []struct {
		name            string
		req             *model.PatchWordRequest // ポインタ型フィールドを持つDTO
		setupMock       func(wordRepo *mocks.WordRepository)
		wantErr         error
		wantUpdatedTerm string
		wantUpdatedDef  string
	}{
		{
			name: "正常系: TermとDefinitionを更新 (PATCH)",
			req: &model.PatchWordRequest{
				Term:       &newTerm, // ポインタを渡す
				Definition: &newDef,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm, "Definition": newDef}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
				updatedWord := &model.Word{WordID: wordID, TenantID: tenantID, Term: newTerm, Definition: newDef}
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(updatedWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: newTerm,
			wantUpdatedDef:  newDef,
		},
		{
			name: "正常系: Termのみ更新 (PATCH)",
			req: &model.PatchWordRequest{
				Term: &newTerm,
				// Definitionはnil
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
				updatedWord := &model.Word{WordID: wordID, TenantID: tenantID, Term: newTerm, Definition: originalDef}
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(updatedWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: newTerm,
			wantUpdatedDef:  originalDef,
		},
		{
			name: "正常系: Definitionのみ更新 (PATCH)",
			req: &model.PatchWordRequest{
				Definition: &newDef,
				// Termはnil
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				expectedUpdates := map[string]interface{}{"Definition": newDef}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
				updatedWord := &model.Word{WordID: wordID, TenantID: tenantID, Term: originalTerm, Definition: newDef}
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(updatedWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: originalTerm,
			wantUpdatedDef:  newDef,
		},
		{
			name: "正常系: 更新内容がない (リクエストで指定なし) (PATCH)",
			req: &model.PatchWordRequest{
				Term:       nil,
				Definition: nil,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once() // 2回目のFindByID
			},
			wantErr:         nil,
			wantUpdatedTerm: originalTerm,
			wantUpdatedDef:  originalDef,
		},
		{
			name: "正常系: 更新内容がない (指定された値が既存と同じ) (PATCH)",
			req: &model.PatchWordRequest{
				Term:       &originalTerm, // 既存と同じ値
				Definition: &originalDef,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once() // 2回目のFindByID
			},
			wantErr:         nil,
			wantUpdatedTerm: originalTerm,
			wantUpdatedDef:  originalDef,
		},
		{
			name: "正常系: Termを空文字に更新 (PATCH)",
			req: &model.PatchWordRequest{
				Term: &emptyStr, // 空文字へのポインタ
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, emptyStr, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": emptyStr}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
				updatedWord := &model.Word{WordID: wordID, TenantID: tenantID, Term: emptyStr, Definition: originalDef}
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(updatedWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: emptyStr,
			wantUpdatedDef:  originalDef,
		},
		{
			name: "異常系: 更新対象が見つからない (PATCH)",
			req: &model.PatchWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(nil, model.ErrNotFound).Once()
			},
			wantErr: model.ErrNotFound,
		},
		{
			name: "異常系: 最初のFindByIDでDBエラー (PATCH)",
			req: &model.PatchWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(nil, model.ErrInternalServer).Once()
			},
			wantErr: model.ErrInternalServer,
		},
		{
			name: "異常系: 新しいTermが重複 (PATCH)",
			req: &model.PatchWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(true, nil).Once()
			},
			wantErr: model.ErrConflict,
		},
		{
			name: "異常系: CheckTermExistsでDBエラー (PATCH)",
			req: &model.PatchWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).
					Return(false, errors.New("db error on check")).Once()
			},
			wantErr: model.ErrInternalServer,
		},
		{
			name: "異常系: UpdateでDBエラー (PATCH)",
			req: &model.PatchWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).
					Return(errors.New("db error on update")).Once()
			},
			wantErr: model.ErrInternalServer,
		},
		{
			name: "異常系: UpdateでNotFoundエラー (競合) (PATCH)",
			req: &model.PatchWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).
					Return(model.ErrNotFound).Once() // Update が NotFound を返す
			},
			wantErr: model.ErrNotFound,
		},
		{
			name: "異常系: 更新後のFindByIDでDBエラー (PATCH)",
			req: &model.PatchWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
					Return(nil, model.ErrInternalServer).Once() // 2回目のFindByIDでエラー
			},
			wantErr: model.ErrInternalServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWordRepo.Mock = mock.Mock{}
			if tt.setupMock != nil {
				tt.setupMock(mockWordRepo)
			}

			updatedWord, err := wordService.PatchWord(ctx, tenantID, wordID, tt.req)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, updatedWord)
			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedWord)
				assert.Equal(t, tt.wantUpdatedTerm, updatedWord.Term)
				assert.Equal(t, tt.wantUpdatedDef, updatedWord.Definition)
				assert.Equal(t, tenantID, updatedWord.TenantID)
				assert.Equal(t, wordID, updatedWord.WordID)
			}

			mockWordRepo.AssertExpectations(t)
			mockProgRepo.AssertNotCalled(t, mock.Anything)
		})
	}
}

// --- Test DeleteWord ---
func Test_wordService_DeleteWord(t *testing.T) {
	ctx := context.Background()
	wordService, _, mockWordRepo, mockProgRepo := setupWordServiceWithMocks()

	tenantID := uuid.New()
	wordID := uuid.New()
	invalidUUID := uuid.Nil

	tests := []struct {
		name      string
		inputTID  uuid.UUID
		inputWID  uuid.UUID
		setupMock func(m *mocks.WordRepository)
		wantErr   error
	}{
		{
			name:     "正常系: 削除成功",
			inputTID: tenantID,
			inputWID: wordID,
			setupMock: func(m *mocks.WordRepository) {
				m.On("Delete", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
					Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:     "正常系: 削除対象が見つからない (冪等性)",
			inputTID: tenantID,
			inputWID: wordID,
			setupMock: func(m *mocks.WordRepository) {
				m.On("Delete", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
					Return(model.ErrNotFound).Once()
			},
			wantErr: nil, // サービスは ErrNotFound を受け取っても nil を返す仕様
		},
		{
			name:     "異常系: リポジトリでDBエラー",
			inputTID: tenantID,
			inputWID: wordID,
			setupMock: func(m *mocks.WordRepository) {
				m.On("Delete", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
					Return(errors.New("some internal db error")).Once()
			},
			wantErr: model.ErrInternalServer,
		},
		{
			name:      "異常系: tenantID が無効 (Nil UUID)",
			inputTID:  invalidUUID,
			inputWID:  wordID,
			setupMock: nil, // リポジトリは呼ばれない
			wantErr:   model.ErrInvalidInput,
		},
		{
			name:      "異常系: wordID が無効 (Nil UUID)",
			inputTID:  tenantID,
			inputWID:  invalidUUID,
			setupMock: nil, // リポジトリは呼ばれない
			wantErr:   model.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWordRepo.Mock = mock.Mock{}
			if tt.setupMock != nil {
				tt.setupMock(mockWordRepo)
			}

			err := wordService.DeleteWord(ctx, tt.inputTID, tt.inputWID)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.setupMock != nil {
				mockWordRepo.AssertExpectations(t)
			} else {
				mockWordRepo.AssertNotCalled(t, mock.Anything) // バリデーションで弾かれるケース
			}
			mockProgRepo.AssertNotCalled(t, mock.Anything)
		})
	}
}
