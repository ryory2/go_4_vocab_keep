// internal/webutil/response.go
package webutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"go_4_vocab_keep/internal/model" // プロジェクトのモジュールパスに合わせてください

	"github.com/go-playground/validator/v10"
)

// HandleError はエラーを解釈し、適切なJSONエラーレスポンスを返します。
// これがアプリケーションのエラーハンドリングの中心となります。
func HandleError(w http.ResponseWriter, err error) {
	// エラーの根本原因に基づいてHTTPステータスコードを決定
	statusCode := MapErrorToStatusCode(err)

	var errResp model.APIErrorResponse
	var appErr *model.AppError

	// エラーがカスタムエラー型 AppError かどうかを判定
	if errors.As(err, &appErr) {
		// AppError の場合、その詳細情報をレスポンスとして使用
		errResp = model.APIErrorResponse{Error: appErr.Detail}
	} else {
		// AppError ではない、予期せぬエラーの場合
		// ログには詳細なエラーを出力
		log.Printf("Unhandled error: %+v", err)

		// クライアントには汎用的なエラーメッセージを返す
		errResp = model.APIErrorResponse{
			Error: model.ErrorDetail{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "サーバー内部でエラーが発生しました。",
			},
		}
	}

	RespondWithJSON(w, statusCode, errResp)
}

// MapErrorToStatusCode はアプリケーションエラーをHTTPステータスコードにマッピングします
// (この関数はご提示の通り、非常に良いのでそのまま利用します)
func MapErrorToStatusCode(err error) int {
	var appErr *model.AppError
	// AppErrorの場合は、ラップされたエラーで判定する
	if errors.As(err, &appErr) {
		err = appErr.Unwrap()
	}

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
		return http.StatusInternalServerError
	}
}

// RespondWithJSON はJSONレスポンスを返します
// (この関数もそのまま利用します)
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON response: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"code":"INTERNAL_SERVER_ERROR", "message":"レスポンス生成中にエラーが発生しました。"}}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

/*
// --- 以下の関数は HandleError に役割を統合したため、不要になる可能性があります ---
// RespondWithError はエラーレスポンスをJSON形式で返します

	func RespondWithError(w http.ResponseWriter, code int, message string) {
		// この関数の役割は HandleError が担うため、直接呼び出す場面は少なくなるかもしれません。
		// シンプルなエラーを返したい場合に残しておくことも可能です。
		errResp := model.APIErrorResponse{
			Error: model.ErrorDetail{
				Message: message,
				// CodeやFieldが不定のため、HandleErrorの使用を推奨
			},
		}
		RespondWithJSON(w, code, errResp)
	}
*/

func NewValidationErrorResponse(errs validator.ValidationErrors) *model.AppError {
	var fields []string
	var messages []string

	for _, err := range errs {
		field := err.Field()
		// ここでタグに応じた日本語メッセージを生成できます。
		message := fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag())
		fields = append(fields, field)
		messages = append(messages, message)
	}

	return model.NewAppError(
		"VALIDATION_ERROR",
		strings.Join(messages, "; "),
		strings.Join(fields, ","),
		model.ErrInvalidInput,
	)
}
