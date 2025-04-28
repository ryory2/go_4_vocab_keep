// internal/handlers/tenant_handler_test.go
package handlers_test // _test パッケージ

import (
	"bytes" // バイト列操作のため
	"io"
	"log/slog"
	"strings"

	// context パッケージ
	"encoding/json"     // JSONエンコード/デコードのため
	"errors"            // エラー作成のため
	"net/http"          // HTTP関連の定数や型のため
	"net/http/httptest" // HTTPテストユーティリティ
	"testing"           // Goのテストフレームワーク
	"time"              // 時間関連 (モックの戻り値用)

	// chi ルーター (テスト対象ハンドラが使う想定)
	// ハンドラが特定のフレームワークに依存しない場合は不要
	"github.com/go-chi/chi/v5"

	"github.com/google/uuid"              // UUID生成のため
	"github.com/stretchr/testify/assert"  // アサーションライブラリ
	"github.com/stretchr/testify/mock"    // モックライブラリ
	"github.com/stretchr/testify/require" // 必須アサーションライブラリ

	// テスト対象のパッケージ
	"go_4_vocab_keep/internal/handlers"
	"go_4_vocab_keep/internal/model"

	// 生成したサービス層のモック
	"go_4_vocab_keep/internal/service/mocks" // モックのパス
	// webutil はテスト内で直接使わないことが多いが、ハンドラが内部で使う
	// webutil の挙動（例: MapErrorToStatusCode）をテスト結果から推測する
)

func TestTenantHandler_CreateTenant(t *testing.T) {
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// --- モックとハンドラのセットアップ ---
	mockTenantService := mocks.NewTenantService(t)                            // モックインスタンス作成 (testify v1.8+)
	tenantHandler := handlers.NewTenantHandler(mockTenantService, testLogger) // モックを注入してハンドラ作成

	// --- テスト用のルーターセットアップ ---
	// chi を使う例。ハンドラが標準の http.HandlerFunc であればルーターは不要な場合もある
	router := chi.NewRouter()
	router.Post("/tenants", tenantHandler.CreateTenant) // ハンドラメソッドを登録

	// --- テストケースで使用する共通データ ---
	validTenantName := "New Valid Tenant"
	validRequest := handlers.CreateTenantRequest{Name: validTenantName}
	// expectedTenantID := uuid.New()
	// サービスが成功時に返すであろうテナント情報の期待値
	// expectedTenant := &model.Tenant{
	// 	TenantID:  expectedTenantID,
	// 	Name:      validTenantName,
	// 	CreatedAt: time.Now(), // 実際には比較が難しいのでアサーションで工夫する
	// 	UpdatedAt: time.Now(),
	// }

	// --- テーブル駆動テストの定義 ---
	tests := []struct {
		name             string      // テストケース名
		requestBody      interface{} // リクエストボディとして送るデータ
		setupMock        func()      // 各テスト前のモック設定関数
		expectedStatus   int         // 期待されるHTTPステータスコード
		expectBody       bool        // レスポンスボディ(Tenant情報)を期待するかどうか
		expectedRespName string      // 期待されるテナント名 (成功時)
		expectedErrMsg   string      // 期待されるエラーメッセージの部分文字列 (エラー時)
	}{
		{
			name:        "正常系: 新規テナント作成成功（1文字）",
			requestBody: handlers.CreateTenantRequest{Name: "a"}, // 1文字のリクエスト ("a")
			setupMock: func() {
				// --- このテストケース専用の期待する引数と戻り値 ---
				testName := "a"
				mockReturnTenant := &model.Tenant{ // サービスが返すであろうTenant
					TenantID:  uuid.New(), // UUIDは任意でOK
					Name:      testName,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				// サービス層のCreateTenantが特定の引数で呼ばれ、
				// 成功時のテナント情報とnilエラーを返すように設定
				mockTenantService.On("CreateTenant",
					mock.AnythingOfType("*context.valueCtx"), // context は型だけチェック
					testName,                                 // 名前は完全一致でチェック
				).Return(mockReturnTenant, nil).Once() // 期待する戻り値と呼び出し回数
			},
			expectedStatus:   http.StatusCreated, // 201 Created
			expectBody:       true,               // Tenant情報が返ることを期待
			expectedRespName: "a",                // 返されるTenantの名前が正しいか
		},
		{
			name:        "正常系: 新規テナント作成成功（100文字）",
			requestBody: handlers.CreateTenantRequest{Name: strings.Repeat("b", 100)},
			setupMock: func() {
				// --- このテストケース専用の期待する引数と戻り値 ---
				testName := strings.Repeat("b", 100)
				mockReturnTenant := &model.Tenant{ // サービスが返すであろうTenant
					TenantID:  uuid.New(), // UUIDは任意でOK
					Name:      testName,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				// サービス層のCreateTenantが特定の引数で呼ばれ、
				// 成功時のテナント情報とnilエラーを返すように設定
				mockTenantService.On("CreateTenant",
					mock.AnythingOfType("*context.valueCtx"), // context は型だけチェック
					testName,                                 // 名前は完全一致でチェック
				).Return(mockReturnTenant, nil).Once() // 期待する戻り値と呼び出し回数
			},
			expectedStatus:   http.StatusCreated,       // 201 Created
			expectBody:       true,                     // Tenant情報が返ることを期待
			expectedRespName: strings.Repeat("b", 100), // 返されるTenantの名前が正しいか
		},
		{
			name:           "異常系: JSON構文エラー (閉じ括弧なし)",    // テスト名をより具体的に
			requestBody:    `{"name": "incomplete json"`, // ★ 閉じ括弧 '}' がない不正なJSON ★
			setupMock:      func() { /* サービス層は呼ばれないので設定不要 */ },
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
			// エラーメッセージは webutil.DecodeJSONBody や json.Unmarshal が返すものに依存
			// "unexpected end of JSON input" などが含まれる可能性がある
			expectedErrMsg: "Invalid request body", // ハンドラが返すエラーメッセージの一部
		},
		{
			name:           "異常系: JSONデータ型エラー (nameが数値)",
			requestBody:    `{"name": 123}`, // ★ name フィールドが文字列ではなく数値 ★
			setupMock:      func() { /* サービス層は呼ばれないので設定不要 */ },
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
			// エラーメッセージは json.UnmarshalTypeError などを含む可能性がある
			expectedErrMsg: "Invalid request body",
		},
		{
			name: "異常系: バリデーションエラー（nameが空）",
			// バリデータが Name="" をエラーとするため
			requestBody:    handlers.CreateTenantRequest{Name: ""},
			setupMock:      func() { /* サービス層は呼ばれないので設定不要 */ },
			expectedStatus: http.StatusBadRequest, // 400 Bad Request
			expectBody:     false,
			expectedErrMsg: "Validation failed", // バリデーションエラーを示すメッセージ
		},
		{
			name:           "異常系: バリデーションエラー（nameが101文字）",
			requestBody:    handlers.CreateTenantRequest{Name: string(make([]byte, 101))}, // 101文字 (max=100違反)
			setupMock:      func() { /* サービス層は呼ばれないので設定不要 */ },
			expectedStatus: http.StatusBadRequest, // 400 Bad Request
			expectBody:     false,
			expectedErrMsg: "Validation failed",
		},
		{
			name:        "異常系: サービスエラー（重複エラー）",
			requestBody: validRequest,
			setupMock: func() {
				// サービス層のCreateTenantが呼ばれ、重複エラー (ErrConflict) を返すように設定
				mockTenantService.On("CreateTenant",
					mock.AnythingOfType("*context.valueCtx"),
					validTenantName,
				).Return(nil, model.ErrConflict).Once() // ErrConflict を返す
			},
			expectedStatus: http.StatusConflict, // 409 Conflict (webutil.MapErrorToStatusCodeの結果)
			expectBody:     false,
			expectedErrMsg: model.ErrConflict.Error(), // エラーメッセージが一致するか
		},
		{
			name:        "異常系: サービスエラー（予期せぬエラー）",
			requestBody: validRequest,
			setupMock: func() {
				// サービス層のCreateTenantが呼ばれ、予期せぬ内部エラーを返すように設定
				mockTenantService.On("CreateTenant",
					mock.AnythingOfType("*context.valueCtx"),
					validTenantName,
				).Return(nil, errors.New("unexpected database error")).Once()
			},
			expectedStatus: http.StatusInternalServerError, // 500 Internal Server Error
			expectBody:     false,
			expectedErrMsg: "unexpected database error",
		},
	}

	// --- テストケースの実行ループ ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. モックの設定を実行
			tt.setupMock()

			// 2. リクエストボディの準備
			var reqBodyBytes []byte
			var err error
			// interface{} の型に応じて処理を分岐
			if bodyStr, ok := tt.requestBody.(string); ok {
				reqBodyBytes = []byte(bodyStr) // 文字列ならそのままバイト列に
			} else {
				reqBodyBytes, err = json.Marshal(tt.requestBody)          // 構造体ならJSONに変換
				require.NoError(t, err, "Failed to marshal request body") // 事前条件
			}

			// 3. HTTPリクエストを作成
			req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBuffer(reqBodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// 4. レスポンスレコーダーを作成
			rr := httptest.NewRecorder()

			// 5. ハンドラを実行 (ルーター経由)
			router.ServeHTTP(rr, req)

			// 6. レスポンスの検証
			assert.Equal(t, tt.expectedStatus, rr.Code, "Status code mismatch")

			// 6a. 成功時のレスポンスボディ検証
			if tt.expectBody {
				// Content-Type ヘッダーのチェック (任意だが推奨)
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Content-Type mismatch")

				var respTenant model.Tenant
				err = json.Unmarshal(rr.Body.Bytes(), &respTenant)
				require.NoError(t, err, "Failed to unmarshal success response body") // ボディがJSONとして正しいか

				assert.Equal(t, tt.expectedRespName, respTenant.Name, "Tenant name mismatch")    // 名前が一致するか
				assert.NotEqual(t, uuid.Nil, respTenant.TenantID, "Tenant ID should not be nil") // UUIDが生成されているか
				// CreatedAt/UpdatedAt は厳密な比較が難しいため、ここでは省略 or IsZero() でないか確認する程度
				assert.False(t, respTenant.CreatedAt.IsZero(), "CreatedAt should not be zero")
				assert.False(t, respTenant.UpdatedAt.IsZero(), "UpdatedAt should not be zero")

			} else { // 6b. エラー時のレスポンスボディ検証
				// エラーレスポンスがJSON形式で、特定のメッセージを含むことを確認
				// Content-Type ヘッダーのチェック (任意)
				// assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Content-Type mismatch for error")

				// webutil.RespondWithError がどのようなJSONを返すかによって調整が必要
				// ここでは {"message": "エラーメッセージ"} を想定
				var errResp map[string]string // または model.APIError などの構造体を定義
				err = json.Unmarshal(rr.Body.Bytes(), &errResp)
				// 必須ではないが、エラーレスポンスがJSON形式であることの確認
				if assert.NoError(t, err, "Failed to unmarshal error response body") {
					// "message" フィールドが存在し、期待するエラーメッセージを含むか
					assert.Contains(t, errResp["message"], tt.expectedErrMsg, "Error message mismatch")
				} else {
					// JSONデコードに失敗した場合、生のボディがメッセージを含むか確認 (代替)
					assert.Contains(t, rr.Body.String(), tt.expectedErrMsg, "Raw error message mismatch")
				}
			}

			// 7. モックの呼び出しが期待通りだったか検証
			mockTenantService.AssertExpectations(t)
		})
	}
}
