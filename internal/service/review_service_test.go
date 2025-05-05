package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/model"

	// モックリポジトリをインポート (実際のパスに合わせてください)
	"go_4_vocab_keep/internal/repository/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite" // トランザクションテスト用
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- テストヘルパー関数 (DBセットアップ) ---
// UpsertLearningProgressBasedOnReview がトランザクションを使うためDBが必要
func setupTestDBReviewService() *gorm.DB {
	// インメモリSQLiteを使用。実際のDBスキーマに合わせて調整が必要な場合あり。
	// AutoMigrateはテストが依存する最小限のモデルに対して行う。
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // DBログ抑制
		// SkipDefaultTransaction: true, // サービス内で明示的にトランザクションを張るので true でも良いが、デフォルト(false)でも動作するはず
	})
	if err != nil {
		panic("failed to connect database for testing: " + err.Error())
	}
	// サービスのテストではマイグレーションは必須ではないことが多いが、
	// トランザクション内でGORMが参照する可能性があるため、念のため実行
	err = db.AutoMigrate(&model.LearningProgress{}, &model.Word{})
	if err != nil {
		panic("failed to migrate database for testing: " + err.Error())
	}

	return db
}

// --- テストヘルパー関数 (モックサービスセットアップ) ---
func setupReviewServiceWithMocks(cfg config.Config) (ReviewService, *gorm.DB, *mocks.ProgressRepository) {
	db := setupTestDBReviewService() // テスト用DBを取得
	mockProgRepo := new(mocks.ProgressRepository)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil)) // ログ出力を抑制
	reviewService := NewReviewService(db, mockProgRepo, cfg, testLogger)
	return reviewService, db, mockProgRepo
}

// --- Test GetReviewWords ---
func Test_reviewService_GetReviewWords(t *testing.T) {
	testTenantID := uuid.New()
	testCfg := config.Config{App: config.AppConfig{ReviewLimit: 5}} // テスト用の設定
	reviewService, _, mockProgRepo := setupReviewServiceWithMocks(testCfg)
	ctx := context.Background()

	now := time.Now()
	// モックが返すテストデータ
	wordID1 := uuid.New()
	wordID2 := uuid.New()
	wordID3 := uuid.New() // Wordがnilのテスト用
	progressesFromRepo := []*model.LearningProgress{
		{ProgressID: uuid.New(), TenantID: testTenantID, WordID: wordID1, Level: model.Level1, NextReviewDate: now.Add(-time.Hour), Word: &model.Word{WordID: wordID1, Term: "term1", Definition: "def1"}},
		{ProgressID: uuid.New(), TenantID: testTenantID, WordID: wordID2, Level: model.Level2, NextReviewDate: now.Add(-2 * time.Hour), Word: &model.Word{WordID: wordID2, Term: "term2", Definition: "def2"}},
		// データ不整合テスト用: Wordがnil
		{ProgressID: uuid.New(), TenantID: testTenantID, WordID: wordID3, Level: model.Level1, NextReviewDate: now.Add(-3 * time.Hour), Word: nil},
	}

	tests := []struct {
		name             string
		setupMock        func()
		wantErr          error
		expectedResCount int                                                 // 期待するレスポンスの数
		checkResponse    func(t *testing.T, res []*model.ReviewWordResponse) // レスポンス内容チェック用関数
	}{
		{
			name: "正常系: 復習単語を複数件取得 (Word nil除く)",
			setupMock: func() {
				// anyTime := mock.AnythingOfType("time.Time") でも可
				// time.Time の比較は mock.MatchedBy で行う方が確実な場合もある
				mockProgRepo.On("FindReviewableByTenant", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, mock.AnythingOfType("time.Time"), testCfg.App.ReviewLimit).
					Return(progressesFromRepo, nil).Once()
			},
			wantErr:          nil,
			expectedResCount: 2, // Wordがnilのものはスキップされるため
			checkResponse: func(t *testing.T, res []*model.ReviewWordResponse) {
				require.Len(t, res, 2)
				assert.Equal(t, progressesFromRepo[0].WordID, res[0].WordID)
				assert.Equal(t, progressesFromRepo[0].Word.Term, res[0].Term)
				assert.Equal(t, progressesFromRepo[0].Level, res[0].Level)
				assert.Equal(t, progressesFromRepo[1].WordID, res[1].WordID)
				assert.Equal(t, progressesFromRepo[1].Word.Term, res[1].Term)
				assert.Equal(t, progressesFromRepo[1].Level, res[1].Level)
			},
		},
		{
			name: "正常系: 復習単語が0件",
			setupMock: func() {
				mockProgRepo.On("FindReviewableByTenant", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, mock.AnythingOfType("time.Time"), testCfg.App.ReviewLimit).
					Return([]*model.LearningProgress{}, nil).Once() // 空のスライスを返す
			},
			wantErr:          nil,
			expectedResCount: 0,
			checkResponse: func(t *testing.T, res []*model.ReviewWordResponse) {
				assert.Empty(t, res) // 空であることを確認
			},
		},
		{
			name: "異常系: リポジトリでエラー発生",
			setupMock: func() {
				mockProgRepo.On("FindReviewableByTenant", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, mock.AnythingOfType("time.Time"), testCfg.App.ReviewLimit).
					Return(nil, errors.New("db connection error")).Once()
			},
			wantErr:          model.ErrInternalServer, // サービスが変換する
			expectedResCount: 0,
			checkResponse:    nil, // エラーなのでレスポンスチェック不要
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProgRepo.Mock = mock.Mock{} // モックリセット
			tt.setupMock()

			responses, err := reviewService.GetReviewWords(ctx, testTenantID)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, responses)
			} else {
				require.NoError(t, err)
				require.NotNil(t, responses)
				assert.Len(t, responses, tt.expectedResCount)
				if tt.checkResponse != nil {
					tt.checkResponse(t, responses)
				}
			}

			mockProgRepo.AssertExpectations(t) // モック検証
		})
	}
}

// --- Test UpsertLearningProgressBasedOnReview ---
func Test_reviewService_UpsertLearningProgressBasedOnReview(t *testing.T) {
	testTenantID := uuid.New()
	testWordID := uuid.New()
	testProgressID := uuid.New()
	testCfg := config.Config{} // このテストではConfigは直接使わない
	reviewService, _, mockProgRepo := setupReviewServiceWithMocks(testCfg)
	ctx := context.Background()

	// テスト実行時刻を基準にする（time.Now() の代わりに使う）
	// time.Now() を直接使うとテストが不安定になるため
	testTimeNow := time.Now().Truncate(time.Second) // 秒以下を切り捨てて比較しやすくする

	// 既存の進捗データの例 (FindByWordID が返す想定)
	existingWord := &model.Word{WordID: testWordID, TenantID: testTenantID, Term: "existing", Definition: "def"}
	existingProgress := &model.LearningProgress{
		ProgressID:     testProgressID,
		TenantID:       testTenantID,
		WordID:         testWordID,
		Level:          model.Level1, // 初期レベル
		NextReviewDate: testTimeNow.Add(-time.Hour),
		Word:           existingWord, // Preloadされている想定
	}

	tests := []struct {
		name             string
		isCorrect        bool
		setupMock        func(now time.Time) // モック設定 (FindByWordID, Create/Update)
		wantErr          error
		checkExpectation func(t *testing.T, now time.Time) // モック呼び出し検証
	}{
		// --- 新規作成 (Create) のテストケース ---
		{
			name:      "正常系: 新規作成 正解 Level1 -> Level2",
			isCorrect: true,
			setupMock: func(now time.Time) {
				// 1. FindByWordID が ErrNotFound を返す
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(nil, model.ErrNotFound).Once()
				// 2. Create が呼ばれるはず
				mockProgRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.MatchedBy(func(p *model.LearningProgress) bool {
					expectedNextReview := now.AddDate(0, 0, 3)
					return p.TenantID == testTenantID &&
						p.WordID == testWordID &&
						p.Level == model.Level2 && // Level2 になるはず
						p.NextReviewDate.Year() == expectedNextReview.Year() && // 日付比較 (Truncateで比較しても良い)
						p.NextReviewDate.Month() == expectedNextReview.Month() &&
						p.NextReviewDate.Day() == expectedNextReview.Day() &&
						p.LastReviewedAt != nil && !p.LastReviewedAt.IsZero() && time.Since(*p.LastReviewedAt) < time.Second // LastReviewedAtが設定される
				})).Return(nil).Once()
			},
			wantErr: nil,
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t)
			},
		},
		{
			name:      "正常系: 新規作成 不正解 Level1 -> Level1",
			isCorrect: false,
			setupMock: func(now time.Time) {
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(nil, model.ErrNotFound).Once()
				mockProgRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.MatchedBy(func(p *model.LearningProgress) bool {
					expectedNextReview := now.AddDate(0, 0, 1)
					return p.Level == model.Level1 && // Level1 のまま
						p.NextReviewDate.Year() == expectedNextReview.Year() &&
						p.NextReviewDate.Month() == expectedNextReview.Month() &&
						p.NextReviewDate.Day() == expectedNextReview.Day() && // 翌日
						p.LastReviewedAt != nil && !p.LastReviewedAt.IsZero()
				})).Return(nil).Once()
			},
			wantErr: nil,
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t)
			},
		},
		{
			name:      "異常系: 新規作成時 Createエラー",
			isCorrect: true,
			setupMock: func(now time.Time) {
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(nil, model.ErrNotFound).Once()
				// Create がエラーを返す
				mockProgRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.LearningProgress")).
					Return(errors.New("db create error")).Once()
			},
			wantErr: model.ErrInternalServer, // サービスが変換
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t)
			},
		},

		// --- 更新 (Update) のテストケース ---
		{
			name:      "正常系: 更新 正解 Level1 -> Level2",
			isCorrect: true,
			setupMock: func(now time.Time) {
				// 1. FindByWordID が既存データを返す
				//    テスト実行時の時刻を使うため、コピーして渡す
				progressToReturn := *existingProgress // ポインタの参照先をコピー
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(&progressToReturn, nil).Once()
				// 2. Update が呼ばれるはず
				mockProgRepo.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), mock.MatchedBy(func(p *model.LearningProgress) bool {
					expectedNextReview := now.AddDate(0, 0, 3)
					return p.ProgressID == existingProgress.ProgressID && // IDが同じか
						p.Level == model.Level2 && // Level2 に更新
						p.NextReviewDate.Year() == expectedNextReview.Year() &&
						p.NextReviewDate.Month() == expectedNextReview.Month() &&
						p.NextReviewDate.Day() == expectedNextReview.Day() &&
						p.LastReviewedAt != nil && !p.LastReviewedAt.IsZero() && time.Since(*p.LastReviewedAt) < time.Second
				})).Return(nil).Once()
			},
			wantErr: nil,
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t)
			},
		},
		{
			name:      "正常系: 更新 不正解 Level2 -> Level1",
			isCorrect: false,
			setupMock: func(now time.Time) {
				progressToReturn := *existingProgress
				progressToReturn.Level = model.Level2 // テスト用にレベルを変更
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(&progressToReturn, nil).Once()
				mockProgRepo.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), mock.MatchedBy(func(p *model.LearningProgress) bool {
					expectedNextReview := now.AddDate(0, 0, 1)
					return p.ProgressID == existingProgress.ProgressID &&
						p.Level == model.Level1 && // Level1 に戻る
						p.NextReviewDate.Year() == expectedNextReview.Year() &&
						p.NextReviewDate.Month() == expectedNextReview.Month() &&
						p.NextReviewDate.Day() == expectedNextReview.Day() &&
						p.LastReviewedAt != nil && !p.LastReviewedAt.IsZero()
				})).Return(nil).Once()
			},
			wantErr: nil,
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t)
			},
		},
		{
			name:      "異常系: 更新時 FindByWordID エラー (NotFound以外)",
			isCorrect: true,
			setupMock: func(now time.Time) {
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(nil, errors.New("some db error")).Once()
				// Create/Update は呼ばれない
			},
			wantErr: model.ErrInternalServer,
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t) // FindByWordID が呼ばれたことを確認
				mockProgRepo.AssertNotCalled(t, "Create")
				mockProgRepo.AssertNotCalled(t, "Update")
			},
		},
		{
			name:      "異常系: 更新時 Wordがnil",
			isCorrect: true,
			setupMock: func(now time.Time) {
				progressToReturn := *existingProgress
				progressToReturn.Word = nil // Word を nil にする
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(&progressToReturn, nil).Once()
				// Update は呼ばれない
			},
			wantErr: model.ErrNotFound, // Word が無効な場合は NotFound
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t)
				mockProgRepo.AssertNotCalled(t, "Update")
			},
		},
		{
			name:      "異常系: 更新時 Wordが論理削除済み",
			isCorrect: true,
			setupMock: func(now time.Time) {
				progressToReturn := *existingProgress
				deletedWord := *existingWord
				deletedWord.DeletedAt = gorm.DeletedAt{Time: now, Valid: true} // 論理削除
				progressToReturn.Word = &deletedWord
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(&progressToReturn, nil).Once()
				// Update は呼ばれない
			},
			wantErr: model.ErrNotFound,
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t)
				mockProgRepo.AssertNotCalled(t, "Update")
			},
		},
		{
			name:      "異常系: 更新時 Updateエラー",
			isCorrect: true,
			setupMock: func(now time.Time) {
				progressToReturn := *existingProgress
				mockProgRepo.On("FindByWordID", ctx, mock.AnythingOfType("*gorm.DB"), testTenantID, testWordID).
					Return(&progressToReturn, nil).Once()
				// Update がエラーを返す
				mockProgRepo.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.LearningProgress")).
					Return(errors.New("db update error")).Once()
			},
			wantErr: model.ErrInternalServer,
			checkExpectation: func(t *testing.T, now time.Time) {
				mockProgRepo.AssertExpectations(t)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// DBはトランザクション内で使われるが、リポジトリモックへの影響はない
			// DBの状態はテストケース間でクリアされるように setupTestDBReviewService を毎回呼ぶ
			// （ただし、このテストではDB自体への書き込みはテストしない）
			_ = setupTestDBReviewService() // DBの準備（インスタンスはここでは使わない）

			mockProgRepo.Mock = mock.Mock{}
			// テスト実行時の時間を記録
			testStart := time.Now().Truncate(time.Second)
			tt.setupMock(testStart) // モック設定に関数内で使う時間を渡す

			// サービスメソッド呼び出し
			err := reviewService.UpsertLearningProgressBasedOnReview(ctx, testTenantID, testWordID, tt.isCorrect)

			// エラー検証
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			// モック検証
			if tt.checkExpectation != nil {
				tt.checkExpectation(t, testStart)
			} else {
				// デフォルトで期待通りの呼び出しがあったか検証
				mockProgRepo.AssertExpectations(t)
			}
		})
	}
}
