// internal/model/error.go
package model

import (
	"errors"
	"fmt"
)

// アプリケーション固有のエラー型。errors.Newの代わりに独自の型を定義することもありますが、
// errors.Isやerrors.Asで判定できるため、ここではシンプルなセンチネルエラーとしておきます。
type sentinelError string

func (e sentinelError) Error() string {
	return string(e)
}

// アプリケーション固有のエラー
var (
	ErrNotFound       = errors.New("resource not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrInternalServer = errors.New("internal server error")
	ErrForbidden      = errors.New("forbidden")
	ErrTenantNotFound = errors.New("tenant not found or invalid")
	ErrConflict       = errors.New("resource conflict") // 重複エラー用
)

// APIError はAPIエラーレスポンスの構造体
type APIErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"` // 任意
}

type AppError struct {
	// APIレスポンスに含める詳細情報
	Detail ErrorDetail
	// エラーの根本原因（ラップしたエラー）
	UnwrapErr error
}

// Error はerrorインターフェースを実装するためのメソッド
func (e *AppError) Error() string {
	return fmt.Sprintf("code: %s, message: %s, field: %s", e.Detail.Code, e.Detail.Message, e.Detail.Field)
}

// Unwrap はエラーチェーンのためにラップしたエラーを返すメソッド (Go 1.13+)
func (e *AppError) Unwrap() error {
	return e.UnwrapErr
}

// NewAppError は新しいAppErrorを生成するためのヘルパー関数
func NewAppError(code, message, field string, unwrapErr error) *AppError {
	return &AppError{
		Detail: ErrorDetail{
			Code:    code,
			Message: message,
			Field:   field,
		},
		UnwrapErr: unwrapErr,
	}
}
