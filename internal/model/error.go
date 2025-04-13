// internal/model/error.go
package model

import "errors"

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
type APIError struct {
	Message string `json:"message"`
}
