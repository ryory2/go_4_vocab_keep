package handlers_test // テスト対象とは別のパッケージ名にするのが一般的

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_4_vocab_keep/internal/handlers" // テスト対象のハンドラー
	"go_4_vocab_keep/internal/model"

	// モックサービスをインポート (適切なパスに変更してください)
	svc_mocks "go_4_vocab_keep/internal/service/mocks"
	// webutil をインポート
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- ヘルパー: モックハンドラーのセットアップ ---
// モックサービスを引数で受け取るように変更
func setupTestHandler(mockService *svc_mocks.WordService) *handlers.WordHandler {
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil)) // ログ出力を抑制
	handler := handlers.NewWordHandler(mockService, testLogger)
	return handler
}

// --- ヘルパー: JSONボディの作成 ---
func newJsonRequest(t *testing.T, method string, target string, body interface{}) *http.Request {
	var reqBody io.Reader
	if body != nil {
		// 文字列が来た場合はそのまま使う (不正なJSONテスト用)
		if bodyStr, ok := body.(string); ok {
			reqBody = strings.NewReader(bodyStr)
		} else {
			jsonData, err := json.Marshal(body)
			require.NoError(t, err)
			reqBody = bytes.NewBuffer(jsonData)
		}
	}
	req, err := http.NewRequest(method, target, reqBody)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// --- ヘルパー: chi ルーターのセットアップ (テスト用) ---
// 実際にミドルウェアを適用し、ハンドラーをルーティングするヘルパー
func setupTestServer(t *testing.T, handler http.HandlerFunc, method string, path string, wordIDParam string) *httptest.Server {
	r := chi.NewRouter()

	// ミドルウェアを模倣 (テストコード内でのコンテキスト設定のため、実際のミドルウェアは使わない)
	// ハンドラ自体が GetTenantIDFromContext を呼ぶことをテストする
	// r.Use(middleware.TenantAuthMiddleware(mockAuthenticator)) // 実際のミドルウェア適用はここではしない

	// ルーティング設定 (テスト対象のハンドラを登録)
	switch method {
	case http.MethodPost:
		r.Post(path, handler)
	case http.MethodGet:
		if wordIDParam != "" {
			r.Get(path, handler) // 例: /words/{word_id}
		} else {
			r.Get(path, handler) // 例: /words
		}
	case http.MethodPut:
		r.Put(path, handler)
	case http.MethodPatch:
		r.Patch(path, handler)
	case http.MethodDelete:
		r.Delete(path, handler)
	}

	return httptest.NewServer(r)
}

// --- ヘルパー: コンテキストに chi の RouteContext を設定 ---
func contextWithChiURLParam(ctx context.Context, key, value string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

// --- Test PostWord ---
func TestWordHandler_PostWord(t *testing.T) {
	mockService := new(svc_mocks.WordService)
	handler := setupTestHandler(mockService) // モックサービスを渡す

	testTenantID := uuid.New()
	// middleware.TenantIDKey がエクスポートされていない場合は直接文字列を使うなど調整
	ctxWithTenant := context.WithValue(context.Background(), model.TenantIDKey, testTenantID)
	testReqBody := model.PostWordRequest{Term: "test", Definition: "def"}
	expectedWord := &model.Word{WordID: uuid.New(), TenantID: testTenantID, Term: "test", Definition: "def"}

	tests := []struct {
		name           string
		reqBody        interface{}
		setupContext   func() context.Context // コンテキスト設定用関数
		setupMock      func()                 // モック設定用関数
		expectedStatus int
		expectedBody   string
	}{
		{
			name:         "正常系",
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				// mock.AnythingOfType("*context.valueCtx") などでコンテキストを柔軟にマッチング
				mockService.On("PostWord", mock.Anything, testTenantID, &testReqBody).Return(expectedWord, nil).Once()
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `"word_id":"` + expectedWord.WordID.String() + `"`,
		},
		{
			name:           "異常系: 認証エラー (TenantIDなし)",
			reqBody:        testReqBody,
			setupContext:   func() context.Context { return context.Background() }, // TenantIDなし
			setupMock:      func() { /* サービスは呼ばれない */ },
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Tenant ID not found in context", // GetTenantIDFromContext が返すエラーメッセージ
		},
		{
			name:           "異常系: 不正なリクエストボディ (JSONデコードエラー)",
			reqBody:        `{"invalid json`, // 不正なJSON文字列
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() { /* サービスは呼ばれない */ },
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid request body",
		},
		{
			name: "異常系: バリデーションエラー (Term空)",
			reqBody: &model.PostWordRequest{
				Term:       "", // Term が空
				Definition: "def",
			},
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() { /* サービスは呼ばれない */ },
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Term and definition are required",
		},
		{
			name:         "異常系: サービスエラー (Conflict)",
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PostWord", mock.Anything, testTenantID, &testReqBody).Return(nil, model.ErrConflict).Once()
			},
			expectedStatus: http.StatusConflict, // MapErrorToStatusCode で変換される想定
			expectedBody:   "Failed to create word",
		},
		{
			name:         "異常系: サービスエラー (Internal)",
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PostWord", mock.Anything, testTenantID, &testReqBody).Return(nil, errors.New("internal service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError, // MapErrorToStatusCode で変換される想定
			expectedBody:   "Failed to create word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 各テスト実行前にモックの状態をリセット
			mockService.Mock = mock.Mock{}
			// モックを設定
			tt.setupMock()

			// リクエスト作成
			req := newJsonRequest(t, http.MethodPost, "/words", tt.reqBody) // パスはダミーでOK
			// コンテキストを設定
			req = req.WithContext(tt.setupContext())

			rr := httptest.NewRecorder()
			// ハンドラ関数を直接呼び出し
			handler.PostWord(rr, req)

			// 結果検証
			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			// モックの呼び出し検証
			mockService.AssertExpectations(t)
		})
	}
}

// --- Test GetWords ---
func TestWordHandler_GetWords(t *testing.T) {
	mockService := new(svc_mocks.WordService)
	handler := setupTestHandler(mockService)

	testTenantID := uuid.New()
	ctxWithTenant := context.WithValue(context.Background(), model.TenantIDKey, testTenantID)
	expectedWords := []*model.Word{
		{WordID: uuid.New(), TenantID: testTenantID, Term: "word1"},
		{WordID: uuid.New(), TenantID: testTenantID, Term: "word2"},
	}

	tests := []struct {
		name           string
		setupContext   func() context.Context
		setupMock      func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:         "正常系: 複数件取得",
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetWords", mock.Anything, testTenantID).Return(expectedWords, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[{"word_id":"`, // 配列で始まる
		},
		{
			name:         "正常系: 0件取得",
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetWords", mock.Anything, testTenantID).Return([]*model.Word{}, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[]`, // 空の配列
		},
		{
			name:         "正常系: サービスがnilを返す",
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetWords", mock.Anything, testTenantID).Return(nil, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[]`, // ハンドラで空配列に変換
		},
		{
			name:           "異常系: 認証エラー",
			setupContext:   func() context.Context { return context.Background() },
			setupMock:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Tenant ID not found in context",
		},
		{
			name:         "異常系: サービスエラー",
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetWords", mock.Anything, testTenantID).Return(nil, errors.New("internal service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to list words",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.Mock = mock.Mock{}
			tt.setupMock()

			req := newJsonRequest(t, http.MethodGet, "/words", nil) // ボディなし
			req = req.WithContext(tt.setupContext())

			rr := httptest.NewRecorder()
			handler.GetWords(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

// --- Test GetWord ---
func TestWordHandler_GetWord(t *testing.T) {
	mockService := new(svc_mocks.WordService)
	handler := setupTestHandler(mockService)

	testTenantID := uuid.New()
	testWordID := uuid.New()
	validWordIDStr := testWordID.String()
	invalidWordIDStr := "invalid-uuid"
	ctxWithTenant := context.WithValue(context.Background(), model.TenantIDKey, testTenantID)
	expectedWord := &model.Word{WordID: testWordID, TenantID: testTenantID, Term: "found", Definition: "def"}

	tests := []struct {
		name           string
		wordIDParam    string                 // URLパラメータの値
		setupContext   func() context.Context // ベースコンテキストの設定
		setupMock      func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:         "正常系",
			wordIDParam:  validWordIDStr,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetWord", mock.Anything, testTenantID, testWordID).Return(expectedWord, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"word_id":"` + validWordIDStr + `"`,
		},
		{
			name:           "異常系: 認証エラー",
			wordIDParam:    validWordIDStr,
			setupContext:   func() context.Context { return context.Background() }, // TenantIDなし
			setupMock:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Tenant ID not found in context",
		},
		{
			name:           "異常系: 不正なWordID形式",
			wordIDParam:    invalidWordIDStr, // 不正なUUID文字列
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid word ID format",
		},
		{
			name:         "異常系: サービスエラー (NotFound)",
			wordIDParam:  validWordIDStr,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetWord", mock.Anything, testTenantID, testWordID).Return(nil, model.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Failed to get word",
		},
		{
			name:         "異常系: サービスエラー (Internal)",
			wordIDParam:  validWordIDStr,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetWord", mock.Anything, testTenantID, testWordID).Return(nil, errors.New("internal service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to get word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.Mock = mock.Mock{}
			tt.setupMock()

			// ベースコンテキスト設定
			baseCtx := tt.setupContext()
			// chi ルートコンテキストを追加
			chiCtx := contextWithChiURLParam(baseCtx, "word_id", tt.wordIDParam)

			req := newJsonRequest(t, http.MethodGet, "/words/"+tt.wordIDParam, nil)
			req = req.WithContext(chiCtx) // chiコンテキストを含んだものをセット

			rr := httptest.NewRecorder()
			handler.GetWord(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

// --- Test PutWord ---
func TestWordHandler_PutWord(t *testing.T) {
	mockService := new(svc_mocks.WordService)
	handler := setupTestHandler(mockService)

	testTenantID := uuid.New()
	testWordID := uuid.New()
	validWordIDStr := testWordID.String()
	ctxWithTenant := context.WithValue(context.Background(), model.TenantIDKey, testTenantID)
	testReqBody := model.PutWordRequest{Term: "updated", Definition: "updated def"} // 値型DTO
	expectedWord := &model.Word{WordID: testWordID, TenantID: testTenantID, Term: "updated", Definition: "updated def"}

	tests := []struct {
		name           string
		wordIDParam    string
		reqBody        interface{}
		setupContext   func() context.Context
		setupMock      func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:         "正常系",
			wordIDParam:  validWordIDStr,
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PutWord", mock.Anything, testTenantID, testWordID, &testReqBody).Return(expectedWord, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"word_id":"` + validWordIDStr + `"`,
		},
		{
			name:           "異常系: 認証エラー",
			wordIDParam:    validWordIDStr,
			reqBody:        testReqBody,
			setupContext:   func() context.Context { return context.Background() },
			setupMock:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Tenant ID not found in context",
		},
		{
			name:           "異常系: 不正なWordID形式",
			wordIDParam:    "invalid-uuid",
			reqBody:        testReqBody,
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid word ID format",
		},
		{
			name:           "異常系: 不正なリクエストボディ",
			wordIDParam:    validWordIDStr,
			reqBody:        `{"invalid`,
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid request body",
		},
		{
			name:         "異常系: サービスエラー (NotFound)",
			wordIDParam:  validWordIDStr,
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PutWord", mock.Anything, testTenantID, testWordID, &testReqBody).Return(nil, model.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Failed to update word",
		},
		{
			name:         "異常系: サービスエラー (Conflict)",
			wordIDParam:  validWordIDStr,
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PutWord", mock.Anything, testTenantID, testWordID, &testReqBody).Return(nil, model.ErrConflict).Once()
			},
			expectedStatus: http.StatusConflict,
			expectedBody:   "Failed to update word",
		},
		{
			name:         "異常系: サービスエラー (Internal)",
			wordIDParam:  validWordIDStr,
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PutWord", mock.Anything, testTenantID, testWordID, &testReqBody).Return(nil, errors.New("internal service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to update word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.Mock = mock.Mock{}
			tt.setupMock()

			baseCtx := tt.setupContext()
			chiCtx := contextWithChiURLParam(baseCtx, "word_id", tt.wordIDParam)

			req := newJsonRequest(t, http.MethodPut, "/words/"+tt.wordIDParam, tt.reqBody)
			req = req.WithContext(chiCtx)

			rr := httptest.NewRecorder()
			handler.PutWord(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

// --- Test PatchWord ---
func TestWordHandler_PatchWord(t *testing.T) {
	mockService := new(svc_mocks.WordService)
	handler := setupTestHandler(mockService)

	testTenantID := uuid.New()
	testWordID := uuid.New()
	validWordIDStr := testWordID.String()
	ctxWithTenant := context.WithValue(context.Background(), model.TenantIDKey, testTenantID)
	newTerm := "patched term"
	newDef := "patched def"
	testReqBody := model.PatchWordRequest{Term: &newTerm} // ポインタ型DTO
	expectedWord := &model.Word{WordID: testWordID, TenantID: testTenantID, Term: newTerm, Definition: "original def"}

	tests := []struct {
		name           string
		wordIDParam    string
		reqBody        interface{}
		setupContext   func() context.Context
		setupMock      func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:         "正常系: Termのみ更新",
			wordIDParam:  validWordIDStr,
			reqBody:      &model.PatchWordRequest{Term: &newTerm},
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				argMatcher := mock.MatchedBy(func(req *model.PatchWordRequest) bool {
					return req.Term != nil && *req.Term == newTerm && req.Definition == nil
				})
				mockService.On("PatchWord", mock.Anything, testTenantID, testWordID, argMatcher).Return(expectedWord, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"term":"` + newTerm + `"`,
		},
		{
			name:         "正常系: Definitionのみ更新",
			wordIDParam:  validWordIDStr,
			reqBody:      &model.PatchWordRequest{Definition: &newDef},
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				argMatcher := mock.MatchedBy(func(req *model.PatchWordRequest) bool {
					return req.Term == nil && req.Definition != nil && *req.Definition == newDef
				})
				updatedWord := &model.Word{WordID: testWordID, TenantID: testTenantID, Term: "original term", Definition: newDef}
				mockService.On("PatchWord", mock.Anything, testTenantID, testWordID, argMatcher).Return(updatedWord, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"definition":"` + newDef + `"`,
		},
		{
			name:           "異常系: 認証エラー",
			wordIDParam:    validWordIDStr,
			reqBody:        testReqBody,
			setupContext:   func() context.Context { return context.Background() },
			setupMock:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Tenant ID not found in context",
		},
		{
			name:           "異常系: 不正なWordID形式",
			wordIDParam:    "invalid-uuid",
			reqBody:        testReqBody,
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid word ID format",
		},
		{
			name:           "異常系: 不正なリクエストボディ",
			wordIDParam:    validWordIDStr,
			reqBody:        `{"invalid`,
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid request body",
		},
		{
			name:           "異常系: 更新フィールドなし",
			wordIDParam:    validWordIDStr,
			reqBody:        &model.PatchWordRequest{Term: nil, Definition: nil}, // 更新フィールドなし
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {}, // バリデーションで弾かれる
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "No fields provided for update",
		},
		{
			name:         "異常系: サービスエラー (NotFound)",
			wordIDParam:  validWordIDStr,
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PatchWord", mock.Anything, testTenantID, testWordID, mock.AnythingOfType("*model.PatchWordRequest")).Return(nil, model.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Failed to patch word",
		},
		{
			name:         "異常系: サービスエラー (Conflict)",
			wordIDParam:  validWordIDStr,
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PatchWord", mock.Anything, testTenantID, testWordID, mock.AnythingOfType("*model.PatchWordRequest")).Return(nil, model.ErrConflict).Once()
			},
			expectedStatus: http.StatusConflict,
			expectedBody:   "Failed to patch word",
		},
		{
			name:         "異常系: サービスエラー (Internal)",
			wordIDParam:  validWordIDStr,
			reqBody:      testReqBody,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("PatchWord", mock.Anything, testTenantID, testWordID, mock.AnythingOfType("*model.PatchWordRequest")).Return(nil, errors.New("internal service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to patch word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.Mock = mock.Mock{}
			tt.setupMock()

			baseCtx := tt.setupContext()
			chiCtx := contextWithChiURLParam(baseCtx, "word_id", tt.wordIDParam)

			req := newJsonRequest(t, http.MethodPatch, "/words/"+tt.wordIDParam, tt.reqBody)
			req = req.WithContext(chiCtx)

			rr := httptest.NewRecorder()
			handler.PatchWord(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

// --- Test DeleteWord ---
func TestWordHandler_DeleteWord(t *testing.T) {
	mockService := new(svc_mocks.WordService)
	handler := setupTestHandler(mockService)

	testTenantID := uuid.New()
	testWordID := uuid.New()
	validWordIDStr := testWordID.String()
	ctxWithTenant := context.WithValue(context.Background(), model.TenantIDKey, testTenantID)

	tests := []struct {
		name           string
		wordIDParam    string
		setupContext   func() context.Context
		setupMock      func()
		expectedStatus int
		expectedBody   string // 基本的に空のはず
	}{
		{
			name:         "正常系",
			wordIDParam:  validWordIDStr,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("DeleteWord", mock.Anything, testTenantID, testWordID).Return(nil).Once()
			},
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:         "正常系: 削除対象が見つからない (冪等性)",
			wordIDParam:  validWordIDStr,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				// DeleteWordサービスはErrNotFoundでもnilを返す仕様を前提
				mockService.On("DeleteWord", mock.Anything, testTenantID, testWordID).Return(nil).Once()
			},
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:           "異常系: 認証エラー",
			wordIDParam:    validWordIDStr,
			setupContext:   func() context.Context { return context.Background() },
			setupMock:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Tenant ID not found in context",
		},
		{
			name:           "異常系: 不正なWordID形式",
			wordIDParam:    "invalid-uuid",
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid word ID format",
		},
		{
			name:         "異常系: サービスエラー (Internal)",
			wordIDParam:  validWordIDStr,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("DeleteWord", mock.Anything, testTenantID, testWordID).Return(errors.New("internal service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to delete word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.Mock = mock.Mock{}
			tt.setupMock()

			baseCtx := tt.setupContext()
			chiCtx := contextWithChiURLParam(baseCtx, "word_id", tt.wordIDParam)

			req := newJsonRequest(t, http.MethodDelete, "/words/"+tt.wordIDParam, nil)
			req = req.WithContext(chiCtx)

			rr := httptest.NewRecorder()
			handler.DeleteWord(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			} else {
				assert.Empty(t, rr.Body.String()) // 204 No Content はボディ空
			}

			mockService.AssertExpectations(t)
		})
	}
}
