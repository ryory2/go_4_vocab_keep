// helpers_test.go
package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// httpRequestDetails はHTTPリクエストの送信に必要な情報をまとめます。
type httpRequestDetails struct {
	Method  string
	Path    string
	Body    interface{}
	Headers map[string]string
}

// httpResponseExpectations はHTTPレスポンスの検証に必要な期待値をまとめます。
type httpResponseExpectations struct {
	ExpectedCode     int
	ExpectedErrorMsg string
	// 成功時のレスポンスボディの具体的な型はテスト毎に異なるため、
	// 検証は呼び出し側で行うか、より汎用的な検証関数を別途用意します。
	// ここではステータスとエラーメッセージの検証に留めます。
}

// sendRequest はHTTPリクエストを送信し、基本的なレスポンス情報を返します。
// ステータスコードのアサーションもここで行います。
func sendRequest(t *testing.T, server *httptest.Server, details httpRequestDetails, expectations httpResponseExpectations) (int, []byte) {
	t.Helper()

	var reqBodyReader io.Reader
	if details.Body != nil {
		if strPayload, ok := details.Body.(string); ok {
			reqBodyReader = strings.NewReader(strPayload)
		} else {
			reqBodyBytes, err := json.Marshal(details.Body)
			require.NoError(t, err, "Failed to marshal request body")
			reqBodyReader = bytes.NewBuffer(reqBodyBytes)
		}
	}

	req, err := http.NewRequest(details.Method, server.URL+details.Path, reqBodyReader)
	require.NoError(t, err, "Failed to create request")

	// デフォルトヘッダー
	if details.Body != nil && reqBodyReader != nil { // ボディがある場合のみデフォルトでJSONを設定
		req.Header.Set("Content-Type", "application/json")
	}
	// カスタムヘッダー
	for key, value := range details.Headers {
		req.Header.Set(key, value)
	}

	client := server.Client()
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to execute request")
	defer resp.Body.Close()

	assert.Equal(t, expectations.ExpectedCode, resp.StatusCode, "Status code mismatch")

	respBodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	return resp.StatusCode, respBodyBytes
}

// verifyErrorResponse はエラーレスポンスのボディを検証します。
func verifyErrorResponse(t *testing.T, logger *slog.Logger, bodyBytes []byte, expectedErrorMsgPart string, tcName string) {
	t.Helper()
	if expectedErrorMsgPart == "" {
		return // 期待するエラーメッセージがない場合は何もしない
	}

	var errResp APIErrorResponse // APIErrorResponseはテスト対象のパッケージで定義されている想定
	err := json.Unmarshal(bodyBytes, &errResp)
	if err == nil {
		assert.True(t, strings.Contains(errResp.Message, expectedErrorMsgPart),
			"Expected error msg part '%s' in JSON msg '%s' for test case '%s'", expectedErrorMsgPart, errResp.Message, tcName)
	} else {
		logger.Warn("Error response body not valid APIErrorResponse JSON.",
			slog.String("test_case", tcName),
			slog.Any("unmarshal_error", err),
			slog.String("raw_body", string(bodyBytes)),
		)
		assert.True(t, strings.Contains(string(bodyBytes), expectedErrorMsgPart),
			"Expected error msg part '%s' in raw body '%s' for test case '%s'", expectedErrorMsgPart, string(bodyBytes), tcName)
	}
}

// clearTable は指定されたモデルのテーブルデータをクリアします。
// GORMモデルをinterface{}として受け取ることで、任意のテーブルに対応できます。
func clearTable(t *testing.T, db *gorm.DB, modelInstance interface{}) {
	t.Helper()
	err := db.Unscoped().Where("1 = 1").Delete(modelInstance).Error
	require.NoError(t, err, fmt.Sprintf("Failed to clear table for model %T", modelInstance))
}

// minInt は2つのintのうち小さい方を返します。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
