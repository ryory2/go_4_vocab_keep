package webutil

import (
	"encoding/json"
	"errors" // errorsを追加
	"log"
	"net/http"

	"go_1_test_repository/internal/model" // modelパッケージのインポートパスに修正 (go.modに合わせてください)
)

// RespondWithError はエラーレスポンスをJSON形式で返します
func RespondWithError(w http.ResponseWriter, code int, message string) {
	log.Printf("Responding with error: status=%d, message=%s", code, message)
	RespondWithJSON(w, code, model.APIError{Message: message})
}

// RespondWithJSON はJSONレスポンスを返します
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON response: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "Internal Server Error during response generation"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// MapErrorToStatusCode はアプリケーションエラーをHTTPステータスコードにマッピングします
func MapErrorToStatusCode(err error) int {
	switch {
	case errors.Is(err, model.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, model.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, model.ErrConflict):
		return http.StatusConflict // 409 Conflict
	case errors.Is(err, model.ErrForbidden) || errors.Is(err, model.ErrTenantNotFound):
		return http.StatusForbidden
	default:
		// ハンドリングされていないエラーは内部サーバーエラーとして扱う
		log.Printf("Unhandled application error: %v", err) // 予期せぬエラーはログに残す
		return http.StatusInternalServerError
	}
}
