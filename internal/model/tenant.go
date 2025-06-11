// internal/model/tenant.go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Tenant はテナントを表します
type Tenant struct {
	TenantID     uuid.UUID      `gorm:"type:uuid;primaryKey" json:"tenant_id"` // JSONにも含める
	Name         string         `gorm:"unique;not null" json:"name"`
	Email        string         `gorm:"unique;not null" json:"email"`
	PasswordHash string         `gorm:"not null" json:"-"` // json:"-"でレスポンスに含めない
	IsActive     bool           `json:"is_active" gorm:"default:false"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"` // 論理削除用 (JSONには含めない)
}

func (Tenant) TableName() string {
	return "tenants"
}

// ContextKey はコンテキストで使用するキーの型
type ContextKey string

const (
	TenantIDKey ContextKey = "tenantID"
)

// RegisterRequest は新規登録APIのリクエストボディの構造体 (DTO)
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// UserResponse はクライアントに返すユーザー情報の構造体 (パスワードを含まない)
type TenantResponse struct {
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}
