// internal/handlers/tenant_handler_test.go
package handlers_test // _test サフィックス

import (
	"bytes"
	"encoding/json"
	"errors" // errors パッケージ
	"net/http"
	"net/http/httptest"
	"testing"
	"time" // time パッケージ (レスポンス検証用)

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert" // testify/assert
	"github.com/stretchr/testify/mock"   // testify/mock

	"go_4_vocab_keep/internal/handlers"   // テスト対象ハンドラ
	"go_4_vocab_keep/internal/model"      // モデル
	// "go_4_vocab_keep/internal/service"    // サービスインターフェース
	"go_4_vocab_keep/internal/service/mocks" // 生成したモック
	// "go_4_vocab_keep/internal/webutil"    // webutil
)

func TestTenantHandler_CreateTenant(t *testing.T) {
	// --- モックとハンドラのセットアップ ---
	// モック TenantService を作成
	mockTenantService := mocks.NewMockTenantService(t) // testify v1.8+ の場合、t を渡す

	// テスト対象のハンドラを作成し、モックサービスを注入
	tenantHandler := handlers.NewTenantHandler(mockTenantService)

	// テスト用のルーターをセットアップ
	router := chi.NewRouter()
	router.Post("/api/v1/tenants", tenantHandler.CreateTenant) // ハンドラを登録

	// --- テストケースの定義 ---
	validTenantName := "Valid Tenant"
	validRequestBody := handlers.CreateTenantRequest{Name: validTenantName}
	expectedTenantUUID := uuid.New() // 成功時に返されると期待するUUID
	expectedTenant := &model.Tenant{ // 成功時にServiceから返されると期待するTenantオブジェクト
		TenantID:  expectedTenantUUID,
		Name:      validTenantName,
		CreatedAt: time.Now(), // 実際の値と比較するのは難しいため、ここでは型のみ確認することが多い
		UpdatedAt: time.Now(),
	}

	tests := []struct {
		name           string          // テストケース名
		requestBody    interface{}     // リクエストボディ
		setupMock      func()          // モックの設定を行う関数
		expectedStatus int             // 期待されるHTTPステータスコード
		expectedBody   *model.Tenant   // 期待されるレスポンスボディ (成功時)
		expectedErrorMsg string        // 期待されるエラーメッセージ (エラー時)
	}{
		{
			name:        "Success - Tenant created",
			requestBody: validRequestBody,
			setupMock: func() {
				// mockTenantService の CreateTenant が呼ばれることを期待
				// 引数として context.Context と validTenantName が渡される
				// 戻り値として expectedTenant と nil エラーを返すように設定
				mockTenantService.On("CreateTenant", mock.AnythingOfType("*context.valueCtx"), validTenantName).
					Return(expectedTenant, nil).
					Once() // このメソッドが1回だけ呼ばれることを期待
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   expectedTenant,
		},
		{
			name:        "Fail - Invalid JSON body",
			requestBody: `{"name": "invalid json"`, // 不正なJSON文字列
			setupMock: func() {
				// このケースでは Service のメソッドは呼ばれないはず
				// On(...) の設定は不要
			},
			expectedStatus:   http.StatusBadRequest,
			expectedErrorMsg: "Invalid request body", // webutil.DecodeJSONBody が返すエラーに基づく部分的なメッセージ
		},
		{
			name:        "Fail - Missing name in body",
			requestBody: handlers.CreateTenantRequest{Name: ""}, // name が空
			setupMock: func() {
				// Service のメソッドは呼ばれないはず
			},
			expectedStatus:   http.StatusBadRequest,
			expectedErrorMsg: "Tenant name is required", // ハンドラ内のバリデーションメッセージ
		},
		{
			name:        "Fail - Service returns Conflict error",
			requestBody: validRequestBody,
			setupMock: func() {
				// CreateTenant が呼ばれ、model.ErrConflict エラーを返すように設定
				mockTenantService.On("CreateTenant", mock.AnythingOfType("*context.valueCtx"), validTenantName).
					Return(nil, model.ErrConflict). // ErrConflict を返す
					Once()
			},
			expectedStatus:   http.StatusConflict, // MapErrorToStatusCode で 409 になるはず
			expectedErrorMsg: model.ErrConflict.Error(), // Service から返されたエラーメッセージ
		},
		{
			name:        "Fail - Service returns Internal Server error",
			requestBody: validRequestBody,
			setupMock: func() {
				// CreateTenant が呼ばれ、その他のエラー (DBエラーなど) を返すように設定
				mockTenantService.On("CreateTenant", mock.AnythingOfType("*context.valueCtx"), validTenantName).
					Return(nil, errors.New("some internal error")). // 任意の内部エラー
					Once()
			},
			expectedStatus:   http.StatusInternalServerError, // MapErrorToStatusCode で 500 になるはず
			expectedErrorMsg: "some internal error",        // Service から返されたエラーメッセージ
		},
	}

	// --- テストケースの実行 ---
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 1. モックの設定を実行
			tc.setupMock()

			// 2. リクエストボディの準備
			var reqBodyBytes []byte
			var err error
			// 文字列の場合はそのまま使う、構造体の場合はJSONにマーシャル
			if reqStr, ok := tc.requestBody.(string); ok {
				reqBodyBytes = []byte(reqStr)
			} else {
				reqBodyBytes, err = json.Marshal(tc.requestBody)
				assert.NoError(t, err, "Failed to marshal request body for test")
			}

			// 3. HTTPリクエストを作成
			req := httptest.NewRequest("POST", "/api/v1/tenants", bytes.NewBuffer(reqBodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// 4. レスポンスレコーダーを作成
			rr := httptest.NewRecorder()

			// 5. ハンドラを実行 (ルーター経由で)
			router.ServeHTTP(rr, req)

			// 6. レスポンスの検証
			assert.Equal(t, tc.expectedStatus, rr.Code, "Unexpected status code")

			if tc.expectedBody != nil { // 成功ケースのボディ検証
				var respBody model.Tenant
				err := json.Unmarshal(rr.Body.Bytes(), &respBody)
				assert.NoError(t, err, "Failed to unmarshal response body")
				// UUIDと時間は動的なので、それ以外のフィールドを比較
				assert.Equal(t, tc.expectedBody.Name, respBody.Name, "Tenant name mismatch")
				assert.NotEqual(t, uuid.Nil, respBody.TenantID, "Tenant ID should not be nil")
			} else if tc.expectedErrorMsg != "" { // エラーケースのボディ検証
				var errResp model.APIError
				err := json.Unmarshal(rr.Body.Bytes(), &errResp)
				assert.NoError(t, err, "Failed to unmarshal error response body")
				// エラーメッセージの一部が含まれているかなどで検証しても良い
				assert.Contains(t, errResp.Message, tc.expectedErrorMsg, "Error message mismatch")
			}

			// 7. モックの期待値が満たされたか検証
			mockTenantService.AssertExpectations(t)
		})
	}
}