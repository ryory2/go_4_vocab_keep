// internal/handlers/word_handler_test.go
package handlers_test

import (
	"bytes" // context をインポート
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time" // time をインポート

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"go_4_vocab_keep/internal/handlers"   // プロジェクト名修正
	"go_4_vocab_keep/internal/middleware" // middleware をインポート
	"go_4_vocab_keep/internal/model"      // プロジェクト名修正

	// repository をインポート
	// service をインポート
	"go_4_vocab_keep/internal/service/mocks" // プロジェクト名修正
	// "go_4_vocab_keep/internal/webutil" // 不要な場合がある
)

// --- TestMain は main_test.go に記述 ---

// --- テスト関数 ---

func TestWordHandler_CreateWord(t *testing.T) {
	// --- セットアップ ---
	clearTables(t)                    // テーブルクリア
	testTenant := createTestTenant(t) // このテスト用のテナント作成
	currentTestTenantID := testTenant.TenantID

	mockWordService := mocks.NewMockWordService(t)
	wordHandler := handlers.NewWordHandler(mockWordService)
	router := chi.NewRouter()
	router.Use(middleware.DevTenantContextMiddleware) // 開発用認証ミドルウェア
	router.Post("/api/v1/words", wordHandler.CreateWord)
	// ------------------

	validReqBody := model.CreateWordRequest{
		Term:       "test-term",
		Definition: "test-definition",
	}
	// 期待される結果 (Serviceから返る想定)
	expectedWord := &model.Word{
		WordID:     uuid.New(), // UUIDは動的なので任意の値で良い
		TenantID:   currentTestTenantID,
		Term:       validReqBody.Term,
		Definition: validReqBody.Definition,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	tests := []struct {
		name           string
		tenantID       *uuid.UUID
		body           interface{}
		setupMock      func()
		expectedStatus int
		expectError    bool
		expectedBody   *model.Word
	}{
		{
			name:     "Success - Valid request",
			tenantID: &currentTestTenantID,
			body:     validReqBody,
			setupMock: func() {
				// CreateWord が適切な引数で呼ばれ、成功結果を返す
				mockWordService.On("CreateWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, &validReqBody).
					Return(expectedWord, nil).Once()
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
			expectedBody:   expectedWord, // 比較用
		},
		{
			name:           "Fail - Missing tenant ID",
			tenantID:       nil, // ヘッダーなし
			body:           validReqBody,
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name:           "Fail - Invalid request body (missing term)",
			tenantID:       &currentTestTenantID,
			body:           model.CreateWordRequest{Definition: "def only"}, // Termがない
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusBadRequest, // ハンドラレベルのバリデーションで弾かれる想定
			expectError:    true,
		},
		{
			name:     "Fail - Service returns conflict",
			tenantID: &currentTestTenantID,
			body:     validReqBody,
			setupMock: func() {
				mockWordService.On("CreateWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, &validReqBody).
					Return(nil, model.ErrConflict).Once()
			},
			expectedStatus: http.StatusConflict,
			expectError:    true,
		},
		// TODO: 他のエラーケース (Service内部エラーなど)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			req := createRequest(t, "POST", "/api/v1/words", tc.body, tc.tenantID)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req) // このテスト用に作成したルーターを使用

			assert.Equal(t, tc.expectedStatus, rr.Code)

			if tc.expectedBody != nil && !tc.expectError {
				var respWord model.Word
				err := json.Unmarshal(rr.Body.Bytes(), &respWord)
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedBody.Term, respWord.Term)
				assert.Equal(t, tc.expectedBody.Definition, respWord.Definition)
				assert.NotEqual(t, uuid.Nil, respWord.WordID) // UUIDが生成されているか
			} else if tc.expectError {
				var errResp model.APIError
				err := json.Unmarshal(rr.Body.Bytes(), &errResp)
				assert.NoError(t, err, "Failed to unmarshal error response body")
				assert.NotEmpty(t, errResp.Message, "Error message should not be empty")
			}

			mockWordService.AssertExpectations(t)
		})
	}
}

func TestWordHandler_ListWords(t *testing.T) {
	// --- セットアップ ---
	clearTables(t)
	testTenant := createTestTenant(t)
	currentTestTenantID := testTenant.TenantID
	anotherTenant := createTestTenant(t) // 別のテナント

	// GORMを使って直接DBにテストデータを作成
	word1 := &model.Word{WordID: uuid.New(), TenantID: currentTestTenantID, Term: "word1", Definition: "def1"}
	word2 := &model.Word{WordID: uuid.New(), TenantID: currentTestTenantID, Term: "word2", Definition: "def2"}
	otherWord := &model.Word{WordID: uuid.New(), TenantID: anotherTenant.TenantID, Term: "other", Definition: "other"}
	assert.NoError(t, testDB.Create(word1).Error)
	assert.NoError(t, testDB.Create(word2).Error)
	assert.NoError(t, testDB.Create(otherWord).Error) // 別テナントのデータ

	mockWordService := mocks.NewMockWordService(t)
	wordHandler := handlers.NewWordHandler(mockWordService)
	router := chi.NewRouter()
	router.Use(middleware.DevTenantContextMiddleware)
	router.Get("/api/v1/words", wordHandler.ListWords)
	// ------------------

	expectedWords := []*model.Word{word1, word2} // このテナントの単語のみ

	tests := []struct {
		name           string
		tenantID       *uuid.UUID
		setupMock      func()
		expectedStatus int
		expectedCount  int // 期待される単語数
	}{
		{
			name:     "Success - List words for tenant",
			tenantID: &currentTestTenantID,
			setupMock: func() {
				// ListWords が呼ばれ、正しいテナントのデータ(2件)を返す
				mockWordService.On("ListWords", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID).
					Return(expectedWords, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:     "Success - List words for tenant with no words",
			tenantID: func() *uuid.UUID { id := createTestTenant(t).TenantID; return &id }(), // データがない新しいテナントID
			setupMock: func() {
				mockWordService.On("ListWords", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("uuid.UUID")). // tenantIDは何でも良い
																		Return([]*model.Word{}, nil).Once() // 空を返す
			},
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "Fail - Missing tenant ID",
			tenantID:       nil,
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:     "Fail - Service returns error",
			tenantID: &currentTestTenantID,
			setupMock: func() {
				mockWordService.On("ListWords", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID).
					Return(nil, errors.New("internal DB error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()
			req := createRequest(t, "GET", "/api/v1/words", nil, tc.tenantID)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)

			if tc.expectedStatus == http.StatusOK {
				var respWords []model.Word
				err := json.Unmarshal(rr.Body.Bytes(), &respWords)
				assert.NoError(t, err)
				assert.Len(t, respWords, tc.expectedCount)
			}
			mockWordService.AssertExpectations(t)
		})
	}
}

func TestWordHandler_GetWord(t *testing.T) {
	// --- セットアップ ---
	clearTables(t)
	testTenant := createTestTenant(t)
	currentTestTenantID := testTenant.TenantID
	anotherTenant := createTestTenant(t)

	wordToGet := &model.Word{WordID: uuid.New(), TenantID: currentTestTenantID, Term: "target", Definition: "def"}
	otherTenantWord := &model.Word{WordID: uuid.New(), TenantID: anotherTenant.TenantID, Term: "other", Definition: "other"}
	assert.NoError(t, testDB.Create(wordToGet).Error)
	assert.NoError(t, testDB.Create(otherTenantWord).Error)

	mockWordService := mocks.NewMockWordService(t)
	wordHandler := handlers.NewWordHandler(mockWordService)
	router := chi.NewRouter()
	router.Use(middleware.DevTenantContextMiddleware)
	router.Get("/api/v1/words/{word_id}", wordHandler.GetWord) // URLパラメータを使うルート
	// ------------------

	tests := []struct {
		name           string
		tenantID       *uuid.UUID
		wordIDParam    string // URLパラメータ
		setupMock      func()
		expectedStatus int
		expectedBody   *model.Word // 成功時に期待するBody
	}{
		{
			name:        "Success - Get existing word",
			tenantID:    &currentTestTenantID,
			wordIDParam: wordToGet.WordID.String(),
			setupMock: func() {
				mockWordService.On("GetWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, wordToGet.WordID).
					Return(wordToGet, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   wordToGet,
		},
		{
			name:        "Fail - Word not found",
			tenantID:    &currentTestTenantID,
			wordIDParam: uuid.New().String(), // 存在しない UUID
			setupMock: func() {
				mockWordService.On("GetWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, mock.AnythingOfType("uuid.UUID")).
					Return(nil, model.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "Fail - Get word from another tenant",
			tenantID:    &currentTestTenantID,
			wordIDParam: otherTenantWord.WordID.String(), // 別のテナントのWord ID
			setupMock: func() {
				// Serviceが ErrNotFound を返すはず (テナントIDで絞り込むため)
				mockWordService.On("GetWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, otherTenantWord.WordID).
					Return(nil, model.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound, // 見つからない扱い
		},
		{
			name:           "Fail - Invalid UUID format",
			tenantID:       &currentTestTenantID,
			wordIDParam:    "not-a-uuid",
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Missing Tenant ID",
			tenantID:       nil,
			wordIDParam:    wordToGet.WordID.String(),
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusUnauthorized,
		},
		// TODO: Service内部エラーケース (500)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()
			url := fmt.Sprintf("/api/v1/words/%s", tc.wordIDParam)
			req := createRequest(t, "GET", url, nil, tc.tenantID)

			// --- URLパラメータを持つリクエストをルーターで処理する場合 ---
			// httptest.NewRecorder と router.ServeHTTP を使うのが最も簡単
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req) // ルーターがURLパラメータを解析してくれる
			// -----------------------------------------------------------

			assert.Equal(t, tc.expectedStatus, rr.Code)

			if tc.expectedBody != nil && tc.expectedStatus == http.StatusOK {
				var respWord model.Word
				err := json.Unmarshal(rr.Body.Bytes(), &respWord)
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedBody.WordID, respWord.WordID)
				assert.Equal(t, tc.expectedBody.Term, respWord.Term)
			}
			mockWordService.AssertExpectations(t)
		})
	}
}

// ============================================================
// TODO: TestWordHandler_UpdateWord 関数のテストケースを追加
// ============================================================
func TestWordHandler_UpdateWord(t *testing.T) {
	// --- セットアップ ---
	clearTables(t)
	testTenant := createTestTenant(t)
	currentTestTenantID := testTenant.TenantID

	wordToUpdate := &model.Word{WordID: uuid.New(), TenantID: currentTestTenantID, Term: "original", Definition: "orig-def"}
	assert.NoError(t, testDB.Create(wordToUpdate).Error)
	existingTermWord := &model.Word{WordID: uuid.New(), TenantID: currentTestTenantID, Term: "existing", Definition: "exist-def"}
	assert.NoError(t, testDB.Create(existingTermWord).Error)

	mockWordService := mocks.NewMockWordService(t)
	wordHandler := handlers.NewWordHandler(mockWordService)
	router := chi.NewRouter()
	router.Use(middleware.DevTenantContextMiddleware)
	router.Put("/api/v1/words/{word_id}", wordHandler.UpdateWord) // PUTメソッド
	// ------------------

	updateReq := model.UpdateWordRequest{
		Term:       func() *string { s := "updated"; return &s }(),
		Definition: func() *string { s := "updated-def"; return &s }(),
	}
	// 更新後の期待値
	updatedWord := &model.Word{
		WordID:     wordToUpdate.WordID,
		TenantID:   currentTestTenantID,
		Term:       *updateReq.Term,
		Definition: *updateReq.Definition,
		// CreatedAt は変わらず、UpdatedAt は更新されるはず (Serviceの戻り値で確認)
	}

	tests := []struct {
		name           string
		tenantID       *uuid.UUID
		wordIDParam    string
		requestBody    interface{}
		setupMock      func()
		expectedStatus int
		expectedBody   *model.Word // 成功時に期待するBody
	}{
		{
			name:        "Success - Update word",
			tenantID:    &currentTestTenantID,
			wordIDParam: wordToUpdate.WordID.String(),
			requestBody: updateReq,
			setupMock: func() {
				// UpdateWord が呼ばれ、更新後の Word オブジェクトと nil を返す
				mockWordService.On("UpdateWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, wordToUpdate.WordID, &updateReq).
					Return(updatedWord, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   updatedWord,
		},
		{
			name:        "Fail - Word not found",
			tenantID:    &currentTestTenantID,
			wordIDParam: uuid.New().String(), // 存在しないID
			requestBody: updateReq,
			setupMock: func() {
				mockWordService.On("UpdateWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, mock.AnythingOfType("uuid.UUID"), &updateReq).
					Return(nil, model.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "Fail - Update with existing term",
			tenantID:    &currentTestTenantID,
			wordIDParam: wordToUpdate.WordID.String(),
			requestBody: model.UpdateWordRequest{Term: &existingTermWord.Term}, // 既存のTermに更新しようとする
			setupMock: func() {
				mockWordService.On("UpdateWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, wordToUpdate.WordID, mock.AnythingOfType("*model.UpdateWordRequest")).
					Return(nil, model.ErrConflict).Once() // ServiceがConflictを返す想定
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Fail - Invalid UUID format",
			tenantID:       &currentTestTenantID,
			wordIDParam:    "invalid-uuid",
			requestBody:    updateReq,
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Invalid request body (bad json)",
			tenantID:       &currentTestTenantID,
			wordIDParam:    wordToUpdate.WordID.String(),
			requestBody:    `{"term": "bad json`,
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Missing tenant ID",
			tenantID:       nil,
			wordIDParam:    wordToUpdate.WordID.String(),
			requestBody:    updateReq,
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusUnauthorized,
		},
		// TODO: Service内部エラーケース (500)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			var reqBodyBytes []byte
			var err error
			if reqStr, ok := tc.requestBody.(string); ok {
				reqBodyBytes = []byte(reqStr)
			} else {
				reqBodyBytes, err = json.Marshal(tc.requestBody)
				assert.NoError(t, err)
			}

			url := fmt.Sprintf("/api/v1/words/%s", tc.wordIDParam)
			req := createRequest(t, "PUT", url, bytes.NewBuffer(reqBodyBytes), tc.tenantID) // PUTメソッド、Bodyあり
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)

			if tc.expectedBody != nil && tc.expectedStatus == http.StatusOK {
				var respWord model.Word
				err := json.Unmarshal(rr.Body.Bytes(), &respWord)
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedBody.WordID, respWord.WordID)
				assert.Equal(t, tc.expectedBody.Term, respWord.Term)
				assert.Equal(t, tc.expectedBody.Definition, respWord.Definition)
			}
			mockWordService.AssertExpectations(t)
		})
	}
}

// ============================================================
// TODO: TestWordHandler_DeleteWord 関数のテストケースを追加
// ============================================================
func TestWordHandler_DeleteWord(t *testing.T) {
	// --- セットアップ ---
	clearTables(t)
	testTenant := createTestTenant(t)
	currentTestTenantID := testTenant.TenantID

	wordToDelete := &model.Word{WordID: uuid.New(), TenantID: currentTestTenantID, Term: "to-delete", Definition: "del-def"}
	assert.NoError(t, testDB.Create(wordToDelete).Error)

	mockWordService := mocks.NewMockWordService(t)
	wordHandler := handlers.NewWordHandler(mockWordService)
	router := chi.NewRouter()
	router.Use(middleware.DevTenantContextMiddleware)
	router.Delete("/api/v1/words/{word_id}", wordHandler.DeleteWord) // DELETEメソッド
	// ------------------

	tests := []struct {
		name           string
		tenantID       *uuid.UUID
		wordIDParam    string
		setupMock      func()
		expectedStatus int
	}{
		{
			name:        "Success - Delete existing word",
			tenantID:    &currentTestTenantID,
			wordIDParam: wordToDelete.WordID.String(),
			setupMock: func() {
				mockWordService.On("DeleteWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, wordToDelete.WordID).
					Return(nil).Once()
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:        "Fail - Word not found",
			tenantID:    &currentTestTenantID,
			wordIDParam: uuid.New().String(), // 存在しないID
			setupMock: func() {
				mockWordService.On("DeleteWord", mock.AnythingOfType("*context.valueCtx"), currentTestTenantID, mock.AnythingOfType("uuid.UUID")).
					Return(model.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Fail - Invalid UUID format",
			tenantID:       &currentTestTenantID,
			wordIDParam:    "invalid-uuid",
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Missing Tenant ID",
			tenantID:       nil,
			wordIDParam:    wordToDelete.WordID.String(),
			setupMock:      func() { /* Serviceは呼ばれない */ },
			expectedStatus: http.StatusUnauthorized,
		},
		// TODO: Service内部エラーケース (500)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()
			url := fmt.Sprintf("/api/v1/words/%s", tc.wordIDParam)
			req := createRequest(t, "DELETE", url, nil, tc.tenantID) // DELETEメソッド、Bodyなし
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			if tc.expectedStatus == http.StatusNoContent {
				assert.Empty(t, rr.Body.Bytes())
			}
			mockWordService.AssertExpectations(t)

			// 削除されたかDBを確認 (任意だがより確実)
			if tc.expectedStatus == http.StatusNoContent {
				var count int64
				wordID, _ := uuid.Parse(tc.wordIDParam)
				// GORMはデフォルトで論理削除済みを除外するので Unscoped() を使う
				testDB.Model(&model.Word{}).Unscoped().Where("word_id = ?", wordID).Count(&count)
				assert.EqualValues(t, 1, count, "Word should still exist physically") // レコード自体は残る

				var word model.Word
				result := testDB.Unscoped().Model(&model.Word{}).Where("word_id = ?", wordID).First(&word)
				assert.NoError(t, result.Error, "Failed to find logically deleted word record")
				assert.True(t, word.DeletedAt.Valid, "DeletedAt should be set (Valid should be true)")
				assert.NotNil(t, word.DeletedAt, "DeletedAt should be set") // DeletedAtが設定されているはず
			}
		})
	}
}
