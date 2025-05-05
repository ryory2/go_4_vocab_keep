// internal/service/review_service_test.go
package service

import (
	"context"
	"errors"
	"io"       // io.Discard のためにインポート
	"log/slog" // slog をインポート
	"testing"
	"time"

	"go_4_vocab_keep/internal/config" // config パッケージをインポート
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository/mocks" // モックリポジトリのパス

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite" // テスト用にsqliteを使用
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- テストヘルパー関数 (インメモリDBセットアップ) ---
func setupTestDBReview() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // テスト中はログを抑制
	})
	if err != nil {
		panic("failed to connect database for review service testing: " + err.Error())
	}
	// SubmitReviewResult 内の .First のためにマイグレーションが必要
	err = db.AutoMigrate(&model.Word{}, &model.LearningProgress{})
	if err != nil {
		panic("failed to migrate database for review service testing: " + err.Error())
	}
	return db
}

// --- Test GetReviewWords ---
func Test_reviewService_GetReviewWords(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBReview() // サービス内でDB接続を使うためセットアップ
	mockProgRepo := new(mocks.ProgressRepository)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	testConfig := config.Config{ // テスト用の設定
		App: config.AppConfig{
			ReviewLimit: 10, // テスト用のリミット値
		},
	}
	reviewService := NewReviewService(db, mockProgRepo, testConfig, testLogger)

	tenantID := uuid.New()
	wordID1 := uuid.New()
	wordID2 := uuid.New()

	// リポジトリが返す進捗データの準備
	mockProgresses := []*model.LearningProgress{
		{
			ProgressID: uuid.New(), TenantID: tenantID, WordID: wordID1, Level: model.Level1,
			Word: &model.Word{WordID: wordID1, TenantID: tenantID, Term: "review1", Definition: "def1"},
		},
		{
			ProgressID: uuid.New(), TenantID: tenantID, WordID: wordID2, Level: model.Level2,
			Word: &model.Word{WordID: wordID2, TenantID: tenantID, Term: "review2", Definition: "def2"},
		},
		// Wordがnilのケース
		{
			ProgressID: uuid.New(), TenantID: tenantID, WordID: uuid.New(), Level: model.Level1,
			Word: nil,
		},
		// Wordが論理削除されているケース (FindReviewableByTenantで除外されるはずだが念のため)
		// {
		// 	ProgressID: uuid.New(), TenantID: tenantID, WordID: wordID3, Level: model.Level1,
		// 	Word: &model.Word{WordID: wordID3, TenantID: tenantID, Term: "deleted_term", Definition: "def_del", Model: gorm.Model{DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}},
		// },
	}

	tests := []struct {
		name          string
		setupMock     func(m *mocks.ProgressRepository)
		wantErr       error
		wantRespCount int      // 期待するレスポンスの数
		wantRespTerms []string // 期待するレスポンスのTerm (順序依存)
	}{
		{
			name: "正常系: 複数件のレビュー対象単語取得成功",
			setupMock: func(m *mocks.ProgressRepository) {
				m.On("FindReviewableByTenant", ctx, db, tenantID, mock.AnythingOfType("time.Time"), testConfig.App.ReviewLimit).
					Return(mockProgresses, nil).Once()
			},
			wantErr:       nil,
			wantRespCount: 2, // Wordがnilのものはスキップされる
			wantRespTerms: []string{"review1", "review2"},
		},
		{
			name: "正常系: レビュー対象単語が0件",
			setupMock: func(m *mocks.ProgressRepository) {
				m.On("FindReviewableByTenant", ctx, db, tenantID, mock.AnythingOfType("time.Time"), testConfig.App.ReviewLimit).
					Return([]*model.LearningProgress{}, nil).Once() // 空のスライスを返す
			},
			wantErr:       nil,
			wantRespCount: 0,
			wantRespTerms: []string{},
		},
		{
			name: "異常系: リポジトリでDBエラー",
			setupMock: func(m *mocks.ProgressRepository) {
				m.On("FindReviewableByTenant", ctx, db, tenantID, mock.AnythingOfType("time.Time"), testConfig.App.ReviewLimit).
					Return(nil, errors.New("db error finding progresses")).Once()
			},
			wantErr:       model.ErrInternalServer, // サービスが変換
			wantRespCount: 0,
			wantRespTerms: nil, // エラー時はnil期待
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProgRepo.Mock = mock.Mock{} // モックをリセット
			if tt.setupMock != nil {
				tt.setupMock(mockProgRepo)
			}

			responses, err := reviewService.GetReviewWords(ctx, tenantID)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, responses)
			} else {
				require.NoError(t, err)
				require.NotNil(t, responses)
				assert.Len(t, responses, tt.wantRespCount)
				if tt.wantRespCount > 0 {
					respTerms := make([]string, len(responses))
					for i, r := range responses {
						respTerms[i] = r.Term
					}
					assert.Equal(t, tt.wantRespTerms, respTerms)
				}
			}
			mockProgRepo.AssertExpectations(t)
		})
	}
}

// --- Test SubmitReviewResult ---
func Test_reviewService_SubmitReviewResult(t *testing.T) {
	ctx := context.Background()
	db := setupTestDBReview() // トランザクション内の .First でDBを使う
	mockProgRepo := new(mocks.ProgressRepository)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	testConfig := config.Config{} // このテストでは使わない
	reviewService := NewReviewService(db, mockProgRepo, testConfig, testLogger)

	tenantID := uuid.New()
	wordID := uuid.New()
	progressID := uuid.New()

	now := time.Now() // 時間比較用に現在時刻を取得

	tests := []struct {
		name           string
		inputWID       uuid.UUID
		inputIsCorrect bool
		setupDB        func(db *gorm.DB) // DBの事前準備
		setupMock      func(m *mocks.ProgressRepository, expectedProgress *model.LearningProgress)
		wantErr        error
	}{
		{
			name:           "正常系: 正解 -> Level1からLevel2へ",
			inputWID:       wordID,
			inputIsCorrect: true,
			setupDB: func(db *gorm.DB) {
				// 事前に進捗と単語をDBに登録
				word := model.Word{WordID: wordID, TenantID: tenantID, Term: "term1", Definition: "def1"}
				progress := model.LearningProgress{ProgressID: progressID, TenantID: tenantID, WordID: wordID, Level: model.Level1, NextReviewDate: now.Add(-time.Hour)}
				require.NoError(t, db.Create(&word).Error)
				require.NoError(t, db.Create(&progress).Error)
			},
			setupMock: func(m *mocks.ProgressRepository, expectedProgress *model.LearningProgress) {
				expectedProgress.Level = model.Level2
				expectedProgress.NextReviewDate = now.AddDate(0, 0, 3) // isCorrect=true, Level1->Level2
				expectedProgress.LastReviewedAt = &now

				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), mock.MatchedBy(func(p *model.LearningProgress) bool {
					assert.Equal(t, progressID, p.ProgressID)
					assert.Equal(t, model.Level2, p.Level)
					// NextReviewDateの比較 (誤差許容)
					assert.WithinDuration(t, now.AddDate(0, 0, 3), p.NextReviewDate, time.Second*5)
					assert.NotNil(t, p.LastReviewedAt)
					assert.WithinDuration(t, now, *p.LastReviewedAt, time.Second*5)
					return true
				})).Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:           "正常系: 正解 -> Level2からLevel3へ",
			inputWID:       wordID,
			inputIsCorrect: true,
			setupDB: func(db *gorm.DB) {
				word := model.Word{WordID: wordID, TenantID: tenantID, Term: "term1", Definition: "def1"}
				progress := model.LearningProgress{ProgressID: progressID, TenantID: tenantID, WordID: wordID, Level: model.Level2, NextReviewDate: now.Add(-time.Hour)}
				require.NoError(t, db.Create(&word).Error)
				require.NoError(t, db.Create(&progress).Error)
			},
			setupMock: func(m *mocks.ProgressRepository, expectedProgress *model.LearningProgress) {
				expectedProgress.Level = model.Level3
				expectedProgress.NextReviewDate = now.AddDate(0, 0, 7) // isCorrect=true, Level2->Level3
				expectedProgress.LastReviewedAt = &now

				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), mock.MatchedBy(func(p *model.LearningProgress) bool {
					assert.Equal(t, model.Level3, p.Level)
					assert.WithinDuration(t, now.AddDate(0, 0, 7), p.NextReviewDate, time.Second*5)
					assert.WithinDuration(t, now, *p.LastReviewedAt, time.Second*5)
					return true
				})).Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:           "正常系: 正解 -> Level3からLevel3 (レビュー期間延長)",
			inputWID:       wordID,
			inputIsCorrect: true,
			setupDB: func(db *gorm.DB) {
				word := model.Word{WordID: wordID, TenantID: tenantID, Term: "term1", Definition: "def1"}
				progress := model.LearningProgress{ProgressID: progressID, TenantID: tenantID, WordID: wordID, Level: model.Level3, NextReviewDate: now.Add(-time.Hour)}
				require.NoError(t, db.Create(&word).Error)
				require.NoError(t, db.Create(&progress).Error)
			},
			setupMock: func(m *mocks.ProgressRepository, expectedProgress *model.LearningProgress) {
				expectedProgress.Level = model.Level3
				expectedProgress.NextReviewDate = now.AddDate(0, 0, 14) // isCorrect=true, Level3->Level3 (期間延長)
				expectedProgress.LastReviewedAt = &now

				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), mock.MatchedBy(func(p *model.LearningProgress) bool {
					assert.Equal(t, model.Level3, p.Level)
					assert.WithinDuration(t, now.AddDate(0, 0, 14), p.NextReviewDate, time.Second*5)
					assert.WithinDuration(t, now, *p.LastReviewedAt, time.Second*5)
					return true
				})).Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:           "正常系: 不正解 -> Level2からLevel1へ",
			inputWID:       wordID,
			inputIsCorrect: false,
			setupDB: func(db *gorm.DB) {
				word := model.Word{WordID: wordID, TenantID: tenantID, Term: "term1", Definition: "def1"}
				progress := model.LearningProgress{ProgressID: progressID, TenantID: tenantID, WordID: wordID, Level: model.Level2, NextReviewDate: now.Add(-time.Hour)}
				require.NoError(t, db.Create(&word).Error)
				require.NoError(t, db.Create(&progress).Error)
			},
			setupMock: func(m *mocks.ProgressRepository, expectedProgress *model.LearningProgress) {
				expectedProgress.Level = model.Level1
				expectedProgress.NextReviewDate = now.AddDate(0, 0, 1) // isCorrect=false
				expectedProgress.LastReviewedAt = &now

				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), mock.MatchedBy(func(p *model.LearningProgress) bool {
					assert.Equal(t, model.Level1, p.Level)
					assert.WithinDuration(t, now.AddDate(0, 0, 1), p.NextReviewDate, time.Second*5)
					assert.WithinDuration(t, now, *p.LastReviewedAt, time.Second*5)
					return true
				})).Return(nil).Once()
			},
			wantErr: nil,
		},
		{
			name:           "異常系: 進捗が見つからない",
			inputWID:       uuid.New(), // 存在しないWordID
			inputIsCorrect: true,
			setupDB: func(db *gorm.DB) {
				// DBには何も登録しない
			},
			setupMock: func(m *mocks.ProgressRepository, expectedProgress *model.LearningProgress) {
				// Update は呼ばれない
			},
			wantErr: model.ErrNotFound,
		},
		{
			name:           "異常系: 関連する単語が削除済み",
			inputWID:       wordID,
			inputIsCorrect: true,
			setupDB: func(db *gorm.DB) {
				// 単語を論理削除状態で登録
				deletedTime := time.Now().Add(-time.Hour)
				// ★ 修正点: Modelフィールドを削除
				word := model.Word{WordID: wordID, TenantID: tenantID, Term: "term1", Definition: "def1"}
				// CreateしてからSaveでDeletedAtを設定
				require.NoError(t, db.Create(&word).Error)
				word.DeletedAt = gorm.DeletedAt{Time: deletedTime, Valid: true} // DeletedAtフィールドを直接設定
				require.NoError(t, db.Save(&word).Error)                        // SaveでDeletedAtを更新

				progress := model.LearningProgress{ProgressID: progressID, TenantID: tenantID, WordID: wordID, Level: model.Level1, NextReviewDate: now.Add(-time.Hour)}
				require.NoError(t, db.Create(&progress).Error)
			},
			setupMock: func(m *mocks.ProgressRepository, expectedProgress *model.LearningProgress) {
				// Update は呼ばれない
			},
			wantErr: model.ErrNotFound,
		},
		{
			name:           "異常系: UpdateでDBエラー",
			inputWID:       wordID,
			inputIsCorrect: true,
			setupDB: func(db *gorm.DB) {
				word := model.Word{WordID: wordID, TenantID: tenantID, Term: "term1", Definition: "def1"}
				progress := model.LearningProgress{ProgressID: progressID, TenantID: tenantID, WordID: wordID, Level: model.Level1, NextReviewDate: now.Add(-time.Hour)}
				require.NoError(t, db.Create(&word).Error)
				require.NoError(t, db.Create(&progress).Error)
			},
			setupMock: func(m *mocks.ProgressRepository, expectedProgress *model.LearningProgress) {
				// Update がDBエラーを返す
				m.On("Update", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.LearningProgress")).
					Return(errors.New("db error on update progress")).Once()
			},
			wantErr: model.ErrInternalServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 各テスト前にDBテーブルをクリア & セットアップ
			require.NoError(t, db.Exec("DELETE FROM learning_progress").Error)
			require.NoError(t, db.Exec("DELETE FROM words").Error)
			if tt.setupDB != nil {
				tt.setupDB(db)
			}

			mockProgRepo.Mock = mock.Mock{} // モックリセット
			// モックセットアップ（期待されるProgressオブジェクトを渡す）
			// .Firstで取得されるであろうオブジェクトを準備（DBに依存）
			var expectedProgressForMock *model.LearningProgress
			if tt.wantErr == nil { // エラーが期待されない場合のみUpdateが呼ばれる
				// 実際のDBから取得した値をベースに期待値を設定
				var initialProgress model.LearningProgress
				err := db.Where("word_id = ?", tt.inputWID).First(&initialProgress).Error
				if err == nil { // DBに存在するはずの場合
					expectedProgressForMock = &initialProgress
				}
			}
			if tt.setupMock != nil {
				tt.setupMock(mockProgRepo, expectedProgressForMock)
			}

			err := reviewService.SubmitReviewResult(ctx, tenantID, tt.inputWID, tt.inputIsCorrect)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			mockProgRepo.AssertExpectations(t)
		})
	}
}
