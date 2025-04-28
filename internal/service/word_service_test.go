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

// --- テストヘルパー関数 (tenant_service_test.go と同様) ---
func setupTestDBWord() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// GORMのデフォルトのトランザクションを使うので、SkipDefaultTransaction は false のまま
	})
	if err != nil {
		panic("failed to connect database for testing: " + err.Error())
	}
	// トランザクションテストのためにマイグレーションが必要な場合がある
	// err = db.AutoMigrate(&model.Word{}, &model.LearningProgress{})
	// if err != nil {
	// 	panic("failed to migrate database for testing: " + err.Error())
	// }
	return db
}

// --- Test CreateWord ---
func Test_wordService_CreateWord(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBWord() // トランザクション用DB (インメモリ)
	mockWordRepo := new(mocks.WordRepository)
	mockProgRepo := new(mocks.ProgressRepository)
	// テスト用ロガーの作成 (出力を捨てる)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wordService := NewWordService(db, mockWordRepo, mockProgRepo, testLogger) // ロガーを渡す

	tenantID := uuid.New()
	testTerm := "test_term"
	testDefinition := "test_definition"

	tests := []struct {
		name      string
		req       *model.CreateWordRequest
		setupMock func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository)
		wantErr   error
		wantWord  bool // Wordが返されることを期待するか
	}{
		{
			name: "正常系: 単語と進捗の作成成功",
			req: &model.CreateWordRequest{
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
						assert.NotEqual(t, uuid.Nil, word.WordID) // IDがセットされるはず
					}).Return(nil).Once()
				// 3. progRepo.Create (成功)
				progRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.LearningProgress")).
					Run(func(args mock.Arguments) {
						prog := args.Get(2).(*model.LearningProgress)
						assert.Equal(t, tenantID, prog.TenantID)
						assert.NotEqual(t, uuid.Nil, prog.WordID)
						assert.Equal(t, model.Level1, prog.Level)
						// NextReviewDate の厳密なチェックは難しいので、型やおおよその範囲で確認
						assert.WithinDuration(t, time.Now().AddDate(0, 0, 1), prog.NextReviewDate, time.Second*5)
					}).Return(nil).Once()
			},
			wantErr:  nil,
			wantWord: true,
		},
		{
			name: "異常系: Termが空",
			req: &model.CreateWordRequest{
				Term:       "",
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				// リポジトリは呼ばれないはず
			},
			wantErr:  model.ErrInvalidInput,
			wantWord: false,
		},
		{
			name: "異常系: Definitionが空",
			req: &model.CreateWordRequest{
				Term:       testTerm,
				Definition: "",
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				// リポジトリは呼ばれないはず
			},
			wantErr:  model.ErrInvalidInput,
			wantWord: false,
		},
		{
			name: "異常系: Termが重複",
			req: &model.CreateWordRequest{
				Term:       testTerm,
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				// 1. CheckTermExists (重複あり)
				wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
					Return(true, nil).Once()
				// wordRepo.Create や progRepo.Create は呼ばれない
			},
			wantErr:  model.ErrConflict,
			wantWord: false,
		},
		{
			name: "異常系: CheckTermExistsでDBエラー",
			req: &model.CreateWordRequest{
				Term:       testTerm,
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				// 1. CheckTermExists (DBエラー)
				wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
					Return(false, errors.New("db error on check")).Once()
			},
			wantErr:  model.ErrInternalServer,
			wantWord: false,
		},
		{
			name: "異常系: wordRepo.CreateでDBエラー",
			req: &model.CreateWordRequest{
				Term:       testTerm,
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				// 1. CheckTermExists (重複なし)
				wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
					Return(false, nil).Once()
				// 2. wordRepo.Create (DBエラー)
				wordRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.Word")).
					Return(errors.New("db error on create word")).Once()
				// progRepo.Create は呼ばれない
			},
			wantErr:  model.ErrInternalServer,
			wantWord: false,
		},
		{
			name: "異常系: progRepo.CreateでDBエラー",
			req: &model.CreateWordRequest{
				Term:       testTerm,
				Definition: testDefinition,
			},
			setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
				// 1. CheckTermExists (重複なし)
				wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).
					Return(false, nil).Once()
				// 2. wordRepo.Create (成功)
				wordRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.Word")).
					Return(nil).Once()
				// 3. progRepo.Create (DBエラー)
				progRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.LearningProgress")).
					Return(errors.New("db error on create progress")).Once()
			},
			wantErr:  model.ErrInternalServer,
			wantWord: false,
		},
		// GORMドライバによっては重複キー制約エラーが返るケースもテスト可能
		// {
		// 	name: "異常系: progRepo.Createで重複キーエラー",
		// 	req: &model.CreateWordRequest{
		// 		Term:       testTerm,
		// 		Definition: testDefinition,
		// 	},
		// 	setupMock: func(wordRepo *mocks.WordRepository, progRepo *mocks.ProgressRepository) {
		// 		// ... wordRepo設定 ...
		// 		wordRepo.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, testTerm, (*uuid.UUID)(nil)).Return(false, nil).Once()
		// 		wordRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.Word")).Return(nil).Once()
		// 		// 3. progRepo.Create (GORMの重複エラーをシミュレート)
		// 		progRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.LearningProgress")).
		// 			Return(gorm.ErrDuplicatedKey).Once() // gorm.ErrDuplicatedKey を返す
		// 	},
		// 	wantErr:  model.ErrConflict, // サービスが ErrConflict に変換することを期待
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

			createdWord, err := wordService.CreateWord(ctx, tenantID, tt.req)

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
			}

			mockWordRepo.AssertExpectations(t)
			mockProgRepo.AssertExpectations(t)
		})
	}
}

// --- Test GetWord ---
func Test_wordService_GetWord(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBWord() // GetWordはDB接続をリポジトリに渡す
	mockWordRepo := new(mocks.WordRepository)
	mockProgRepo := new(mocks.ProgressRepository) // 使わないが NewWordService に必要
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wordService := NewWordService(db, mockWordRepo, mockProgRepo, testLogger) // ロガーを渡す

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
					Return(nil, model.ErrInternalServer).Once() // リポジトリが変換したエラー
			},
			wantErr:  model.ErrInternalServer,
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
			// mockProgRepo は呼ばれないはずなので AssertNotCalled も可能
			mockProgRepo.AssertNotCalled(t, mock.Anything)
		})
	}
}

// --- Test ListWords ---
func Test_wordService_ListWords(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBWord()
	mockWordRepo := new(mocks.WordRepository)
	mockProgRepo := new(mocks.ProgressRepository)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wordService := NewWordService(db, mockWordRepo, mockProgRepo, testLogger) // ロガーを渡す

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
		wantLen   int // 期待するリストの長さ
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
					Return([]*model.Word{}, nil).Once() // 空のスライスを返す
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

			words, err := wordService.ListWords(ctx, tenantID)

			if tt.wantErr != nil {
				require.Error(t, err)
				// ListWordsは直接InternalServerErrorを返すのでErrorIsは使わない
				// assert.ErrorIs(t, err, tt.wantErr)
				// エラーメッセージで比較するか、型で比較する
				assert.Equal(t, tt.wantErr.Error(), err.Error()) // 簡単な比較
				assert.Nil(t, words)
			} else {
				require.NoError(t, err)
				require.NotNil(t, words)
				assert.Len(t, words, tt.wantLen)
				// 中身を比較する場合は assert.Equal(t, tt.wantWords, words)
				// ポインタのスライスなので要素ごとに比較する方が安全かもしれない
				assert.Equal(t, tt.wantWords, words)
			}

			mockWordRepo.AssertExpectations(t)
			mockProgRepo.AssertNotCalled(t, mock.Anything)
		})
	}
}

// --- Test UpdateWord ---
func Test_wordService_UpdateWord(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBWord()
	mockWordRepo := new(mocks.WordRepository)
	mockProgRepo := new(mocks.ProgressRepository) // 使わない
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wordService := NewWordService(db, mockWordRepo, mockProgRepo, testLogger) // ロガーを渡す

	tenantID := uuid.New()
	wordID := uuid.New()
	originalTerm := "original_term"
	originalDef := "original_def"
	newTerm := "new_term"
	newDef := "new_def"

	originalWord := &model.Word{
		WordID:     wordID,
		TenantID:   tenantID,
		Term:       originalTerm,
		Definition: originalDef,
		// CreatedAt/UpdatedAt はテストに影響しない
	}

	tests := []struct {
		name            string
		inputWID        uuid.UUID
		req             *model.UpdateWordRequest
		setupMock       func(m *mocks.WordRepository)
		wantErr         error
		wantUpdatedTerm string // 更新後のTermを期待
		wantUpdatedDef  string // 更新後のDefinitionを期待
	}{
		{
			name:     "正常系: TermとDefinitionを更新",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Term:       &newTerm,
				Definition: &newDef,
			},
			setupMock: func(m *mocks.WordRepository) {
				// 1. FindByID (更新対象取得)
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
					Return(originalWord, nil).Once()
				// 2. CheckTermExists (新しいTermの重複チェック)
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).
					Return(false, nil).Once()
				// 3. Update (Term と Definition を更新)
				expectedUpdates := map[string]interface{}{"Term": newTerm, "Definition": newDef}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).
					Return(nil).Once()
				// 4. FindByID (更新後のデータを取得) - ここで更新後のデータを返す
				updatedWord := &model.Word{WordID: wordID, TenantID: tenantID, Term: newTerm, Definition: newDef}
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
					Return(updatedWord, nil).Once() // 2回目のFindByID
			},
			wantErr:         nil,
			wantUpdatedTerm: newTerm,
			wantUpdatedDef:  newDef,
		},
		{
			name:     "正常系: Termのみ更新",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Term: &newTerm,
				// Definitionはnil
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
				updatedWord := &model.Word{WordID: wordID, TenantID: tenantID, Term: newTerm, Definition: originalDef} // Defは元のまま
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(updatedWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: newTerm,
			wantUpdatedDef:  originalDef, // 元のまま
		},
		{
			name:     "正常系: Definitionのみ更新",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Definition: &newDef,
				// Termはnil
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				// CheckTermExists は呼ばれない
				expectedUpdates := map[string]interface{}{"Definition": newDef}
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
				updatedWord := &model.Word{WordID: wordID, TenantID: tenantID, Term: originalTerm, Definition: newDef} // Termは元のまま
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(updatedWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: originalTerm, // 元のまま
			wantUpdatedDef:  newDef,
		},
		{
			name:     "正常系: 更新内容がない (Term, Defが同じ)",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Term:       &originalTerm, // 元と同じ
				Definition: &originalDef,  // 元と同じ
			},
			setupMock: func(m *mocks.WordRepository) {
				// 1. FindByID (更新対象取得)
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				// CheckTermExists や Update は呼ばれない
				// 2. FindByID (更新後のデータを取得) - 変更ないので元のデータを返す
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
			},
			wantErr:         nil,
			wantUpdatedTerm: originalTerm,
			wantUpdatedDef:  originalDef,
		},
		{
			name:     "正常系: 更新内容がない (リクエストがnil)",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Term:       nil,
				Definition: nil,
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
			name:     "異常系: 更新対象が見つからない",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				// 1. FindByID (見つからない)
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
					Return(nil, model.ErrNotFound).Once()
				// 他のメソッドは呼ばれない
			},
			wantErr: model.ErrNotFound,
		},
		{
			name:     "異常系: 最初のFindByIDでDBエラー",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				// 1. FindByID (DBエラー)
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
					Return(nil, model.ErrInternalServer).Once()
			},
			wantErr: model.ErrInternalServer,
		},
		{
			name:     "異常系: 新しいTermが重複",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				// 1. FindByID (成功)
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				// 2. CheckTermExists (重複あり)
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).
					Return(true, nil).Once()
				// Update や 2回目の FindByID は呼ばれない
			},
			wantErr: model.ErrConflict,
		},
		{
			name:     "異常系: CheckTermExistsでDBエラー",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
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
			name:     "異常系: UpdateでDBエラー",
			inputWID: wordID,
			req: &model.UpdateWordRequest{
				Term: &newTerm,
			},
			setupMock: func(m *mocks.WordRepository) {
				m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
				m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
				expectedUpdates := map[string]interface{}{"Term": newTerm}
				// 3. Update (DBエラー)
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).
					Return(errors.New("db error on update")).Once()
				// 2回目の FindByID は呼ばれない
			},
			wantErr: model.ErrInternalServer,
		},
		// {
		// 	name:     "異常系: UpdateでNotFoundエラー (レアケース)",
		// 	inputWID: wordID,
		// 	req: &model.UpdateWordRequest{
		// 		Term: &newTerm,
		// 	},
		// 	setupMock: func(m *mocks.WordRepository) {
		// 		m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
		// 		m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
		// 		expectedUpdates := map[string]interface{}{"Term": newTerm}
		// 		// 3. Update (NotFoundエラー)
		// 		m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).
		// 			Return(model.ErrNotFound).Once()
		// 	},
		// 	wantErr: model.ErrNotFound, // サービスが ErrNotFound をそのまま返す
		// },
		// {
		// 	name:     "異常系: 更新後のFindByIDでDBエラー (レアケース)",
		// 	inputWID: wordID,
		// 	req: &model.UpdateWordRequest{
		// 		Term: &newTerm,
		// 	},
		// 	setupMock: func(m *mocks.WordRepository) {
		// 		m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).Return(originalWord, nil).Once()
		// 		m.On("CheckTermExists", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, newTerm, &wordID).Return(false, nil).Once()
		// 		expectedUpdates := map[string]interface{}{"Term": newTerm}
		// 		m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID, expectedUpdates).Return(nil).Once()
		// 		// 4. FindByID (DBエラー)
		// 		m.On("FindByID", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
		// 			Return(nil, model.ErrInternalServer).Once() // 2回目のFindByID
		// 	},
		// 	wantErr: model.ErrInternalServer,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWordRepo.Mock = mock.Mock{}
			if tt.setupMock != nil {
				tt.setupMock(mockWordRepo)
			}

			updatedWord, err := wordService.UpdateWord(ctx, tenantID, tt.inputWID, tt.req)

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

// // --- Test DeleteWord ---
// // 注意: DeleteWord の実装が GORM の tx.Delete を直接呼んでいます。
// // モックベースのテストでは、このGORMの挙動（レコードが見つからない場合にエラーを返さないなど）を
// // 正確にシミュレートするのは難しい場合があります。
// // ここでは、サービス内のロジック（存在チェック、トランザクション管理、エラーハンドリング）を
// // 主にテストします。
// func Test_wordService_DeleteWord(t *testing.T) {
// 	ctx := context.Background()
// 	// db := setupTestDBWord()
// 	// mockWordRepo := new(mocks.WordRepository)
// 	// mockProgRepo := new(mocks.ProgressRepository) // 使わない
// 	// wordService := NewWordService(db, mockWordRepo, mockProgRepo)

// 	tenantID := uuid.New()
// 	wordID := uuid.New()

// 	// --- Test DeleteWord ---
// 	// 実際の DeleteWord は GORM の Delete を呼ぶため、リポジトリモックは不要
// 	// しかし、テスト容易性を考えると、リポジトリ層に Delete メソッドを用意し、
// 	// サービス層はそのリポジトリメソッドを呼ぶ方がユニットテストしやすいです。
// 	//
// 	// 現状の実装 (tx.First & tx.Delete) をテストする場合:
// 	// - モックは使わず、実際のインメモリDBでテストする
// 	// - または、GORM の tx.First / tx.Delete の挙動を理解した上で
// 	//   テストケースを設計する（エラーハンドリング中心）
// 	//
// 	// ここでは、*もしリポジトリ層に Delete メソッドがあったと仮定した場合* の
// 	// テストコード構造を示します。
// 	// (WordRepository に Delete(ctx, tx, tenantID, wordID) error がある想定)

// 	tests_with_repo_delete := []struct {
// 		name      string
// 		inputWID  uuid.UUID
// 		setupMock func(m *mocks.WordRepository)
// 		wantErr   error
// 	}{
// 		{
// 			name:     "正常系: 削除成功 (リポジトリ使用想定)",
// 			inputWID: wordID,
// 			setupMock: func(m *mocks.WordRepository) {
// 				// DeleteWord の実装がリポジトリを呼ぶように修正されている場合
// 				// m.On("Delete", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
// 				// 	Return(nil).Once()
// 			},
// 			wantErr: nil,
// 			// 現状の実装をテストする場合、モックは使わず、DB操作の結果を検証する
// 			// setupMock は nil にし、DBにデータを事前投入する
// 		},
// 		{
// 			name:     "異常系: 削除対象が見つからない (リポジトリ使用想定)",
// 			inputWID: wordID,
// 			setupMock: func(m *mocks.WordRepository) {
// 				// DeleteWord の実装がリポジトリを呼ぶように修正されている場合
// 				// m.On("Delete", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
// 				// 	Return(model.ErrNotFound).Once()
// 			},
// 			wantErr: model.ErrNotFound,
// 			// 現状の実装をテストする場合、setupMock は nil にし、DBにデータがない状態で実行
// 		},
// 		{
// 			name:     "異常系: 削除中にDBエラー (リポジトリ使用想定)",
// 			inputWID: wordID,
// 			setupMock: func(m *mocks.WordRepository) {
// 				// DeleteWord の実装がリポジトリを呼ぶように修正されている場合
// 				// m.On("Delete", ctx, mock.AnythingOfType("*gorm.DB"), tenantID, wordID).
// 				// 	Return(model.ErrInternalServer).Once()
// 			},
// 			wantErr: model.ErrInternalServer,
// 			// 現状の実装をテストする場合、GORMのエラーハンドリングをテストする
// 		},
// 	}

// 	// 現状の DeleteWord 実装 (tx.First, tx.Delete) をテストする例
// 	t.Run("現状実装: 正常系 削除成功", func(t *testing.T) {
// 		// 1. 事前データの準備 (インメモリDBに実際に登録)
// 		db := setupTestDBWord() // 新しいDBインスタンス
// 		err := db.AutoMigrate(&model.Word{})
// 		require.NoError(t, err)
// 		wordService := NewWordService(db, nil, nil) // モックは使わない
// 		wordToCreate := &model.Word{WordID: wordID, TenantID: tenantID, Term: "to_delete", Definition: "def"}
// 		result := db.Create(wordToCreate)
// 		require.NoError(t, result.Error)
// 		require.EqualValues(t, 1, result.RowsAffected)

// 		// 2. 削除実行
// 		err = wordService.DeleteWord(ctx, tenantID, wordID)

// 		// 3. 検証
// 		require.NoError(t, err)

// 		// 4. 削除されたか確認 (論理削除なので find で見つからないはず)
// 		var foundWord model.Word
// 		err = db.Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&foundWord).Error
// 		assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "Word should not be found after delete")

// 		// 論理削除されているか確認 (Unscopedを使う)
// 		var deletedWord model.Word
// 		err = db.Unscoped().Where("tenant_id = ? AND word_id = ?", tenantID, wordID).First(&deletedWord).Error
// 		require.NoError(t, err, "Deleted word should be found with Unscoped")
// 		assert.NotNil(t, deletedWord.DeletedAt, "DeletedAt should be set")
// 	})

// 	t.Run("現状実装: 異常系 削除対象が見つからない", func(t *testing.T) {
// 		db := setupTestDBWord()
// 		err := db.AutoMigrate(&model.Word{})
// 		require.NoError(t, err)
// 		wordService := NewWordService(db, nil, nil)
// 		nonExistentID := uuid.New()

// 		err = wordService.DeleteWord(ctx, tenantID, nonExistentID)

// 		require.Error(t, err)
// 		assert.ErrorIs(t, err, model.ErrNotFound)
// 	})

// 	// 注意: GORMのDBエラーをシミュレートするのは、インメモリDBでは難しい
// 	// このテストは、DB接続が切れているなどの状況をモックDBでシミュレートする場合に有効
// 	// t.Run("現状実装: 異常系 DBエラー (First)", ...)
// 	// t.Run("現状実装: 異常系 DBエラー (Delete)", ...)

// 	// // モックベースでテストする場合のループ (参考)
// 	// for _, tt := range tests_with_repo_delete {
// 	// 	// このループは現状の DeleteWord 実装では意図通りに動作しません
// 	// 	// t.Run(tt.name, func(t *testing.T) {
// 	// 	// 	mockWordRepo.Mock = mock.Mock{}
// 	// 	// 	if tt.setupMock != nil {
// 	// 	// 		tt.setupMock(mockWordRepo)
// 	// 	// 	}

// 	// 	// 	err := wordService.DeleteWord(ctx, tenantID, tt.inputWID)

// 	// 	// 	if tt.wantErr != nil {
// 	// 	// 		require.Error(t, err)
// 	// 	// 		assert.ErrorIs(t, err, tt.wantErr)
// 	// 	// 	} else {
// 	// 	// 		require.NoError(t, err)
// 	// 	// 	}

// 	// 	// 	// DeleteWord がリポジトリを呼ぶ実装なら AssertExpectations が有効
// 	// 	// 	// mockWordRepo.AssertExpectations(t)
// 	// 	// 	mockProgRepo.AssertNotCalled(t, mock.Anything)
// 	// 	// })
// 	// }
// }
