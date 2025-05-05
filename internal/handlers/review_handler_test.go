package handlers_test // テスト対象とは別のパッケージ名

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

	"go_4_vocab_keep/internal/handlers" // テスト対象
	"go_4_vocab_keep/internal/model"

	// モックサービスをインポート (適切なパスに変更してください)
	svc_mocks "go_4_vocab_keep/internal/service/mocks"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- ヘルパー: モックハンドラーのセットアップ ---
func setupTestReviewHandler(mockService *svc_mocks.ReviewService) *handlers.ReviewHandler {
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil)) // ログ出力を抑制
	handler := handlers.NewReviewHandler(mockService, testLogger)
	return handler
}

// --- ヘルパー: JSONボディの作成 ---
func newJsonRequestReview(t *testing.T, method string, target string, body interface{}) *http.Request {
	var reqBody io.Reader
	if body != nil {
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
func setupTestServerReview(t *testing.T, handler http.HandlerFunc, method string, path string) *httptest.Server {
	r := chi.NewRouter()
	// ルーティング設定
	switch method {
	case http.MethodGet:
		r.Get(path, handler)
	case http.MethodPost: // SubmitReviewResult は POST かもしれないが、例として POST
		r.Post(path, handler)
	}
	return httptest.NewServer(r)
}

// --- ヘルパー: chi の RouteContext を設定 ---
func contextWithChiURLParamReview(ctx context.Context, key, value string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

// --- Test GetReviewWords ---
func TestReviewHandler_GetReviewWords(t *testing.T) {
	mockService := new(svc_mocks.ReviewService)
	handler := setupTestReviewHandler(mockService)

	testTenantID := uuid.New()
	ctxWithTenant := context.WithValue(context.Background(), model.TenantIDKey, testTenantID)
	expectedReviewWords := []*model.ReviewWordResponse{
		{WordID: uuid.New(), Term: "review1", Definition: "def1", Level: model.Level1},
		{WordID: uuid.New(), Term: "review2", Definition: "def2", Level: model.Level2},
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
				mockService.On("GetReviewWords", mock.Anything, testTenantID).Return(expectedReviewWords, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[{"word_id":"`, // 配列で始まる
		},
		{
			name:         "正常系: 0件取得",
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetReviewWords", mock.Anything, testTenantID).Return([]*model.ReviewWordResponse{}, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[]`, // 空の配列
		},
		{
			name:         "正常系: サービスがnilを返す",
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("GetReviewWords", mock.Anything, testTenantID).Return(nil, nil).Once()
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
				mockService.On("GetReviewWords", mock.Anything, testTenantID).Return(nil, errors.New("internal service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to get review words",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.Mock = mock.Mock{}
			tt.setupMock()

			req := newJsonRequestReview(t, http.MethodGet, "/review/words", nil) // パスは例
			req = req.WithContext(tt.setupContext())

			rr := httptest.NewRecorder()
			handler.GetReviewWords(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			mockService.AssertExpectations(t)
		})
	}
}

// --- Test SubmitReviewResult ---
func TestReviewHandler_SubmitReviewResult(t *testing.T) {
	mockService := new(svc_mocks.ReviewService)
	handler := setupTestReviewHandler(mockService)

	testTenantID := uuid.New()
	testWordID := uuid.New()
	validWordIDStr := testWordID.String()
	ctxWithTenant := context.WithValue(context.Background(), model.TenantIDKey, testTenantID)

	tests := []struct {
		name           string
		wordIDParam    string
		reqBody        interface{}
		isCorrectArg   bool // サービスに渡される isCorrect の期待値
		setupContext   func() context.Context
		setupMock      func()
		expectedStatus int
		expectedBody   string // エラー時のメッセージ
	}{
		{
			name:         "正常系: 正解を送信",
			wordIDParam:  validWordIDStr,
			reqBody:      &model.SubmitReviewRequest{IsCorrect: true},
			isCorrectArg: true,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("SubmitReviewResult", mock.Anything, testTenantID, testWordID, true).Return(nil).Once()
			},
			expectedStatus: http.StatusNoContent, // 成功時は 204
			expectedBody:   "",
		},
		{
			name:         "正常系: 不正解を送信",
			wordIDParam:  validWordIDStr,
			reqBody:      &model.SubmitReviewRequest{IsCorrect: false},
			isCorrectArg: false,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("SubmitReviewResult", mock.Anything, testTenantID, testWordID, false).Return(nil).Once()
			},
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:           "異常系: 認証エラー",
			wordIDParam:    validWordIDStr,
			reqBody:        &model.SubmitReviewRequest{IsCorrect: true},
			setupContext:   func() context.Context { return context.Background() },
			setupMock:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Tenant ID not found in context",
		},
		{
			name:           "異常系: 不正なWordID形式",
			wordIDParam:    "invalid-uuid",
			reqBody:        &model.SubmitReviewRequest{IsCorrect: true},
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid word ID format",
		},
		{
			name:           "異常系: 不正なリクエストボディ (JSON)",
			wordIDParam:    validWordIDStr,
			reqBody:        `{"is_correct":`, // 不正なJSON
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid request body",
		},
		{
			name:           "異常系: 不正なリクエストボディ (フィールド型違いなど)",
			wordIDParam:    validWordIDStr,
			reqBody:        `{"is_correct":"true"}`, // bool ではなく string
			setupContext:   func() context.Context { return ctxWithTenant },
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid request body", // デコードエラーになるはず
		},
		{
			name:         "異常系: サービスエラー (NotFound)",
			wordIDParam:  validWordIDStr,
			reqBody:      &model.SubmitReviewRequest{IsCorrect: true},
			isCorrectArg: true,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("SubmitReviewResult", mock.Anything, testTenantID, testWordID, true).Return(model.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Failed to submit review result",
		},
		{
			name:         "異常系: サービスエラー (Internal)",
			wordIDParam:  validWordIDStr,
			reqBody:      &model.SubmitReviewRequest{IsCorrect: false},
			isCorrectArg: false,
			setupContext: func() context.Context { return ctxWithTenant },
			setupMock: func() {
				mockService.On("SubmitReviewResult", mock.Anything, testTenantID, testWordID, false).Return(errors.New("internal service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to submit review result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.Mock = mock.Mock{}
			tt.setupMock()

			baseCtx := tt.setupContext()
			chiCtx := contextWithChiURLParamReview(baseCtx, "word_id", tt.wordIDParam)

			// SubmitReviewResult は POST か PUT か PATCH が一般的だが、例として POST を使う
			req := newJsonRequestReview(t, http.MethodPost, "/review/words/"+tt.wordIDParam+"/result", tt.reqBody)
			req = req.WithContext(chiCtx)

			rr := httptest.NewRecorder()
			handler.SubmitReviewResult(rr, req)

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
