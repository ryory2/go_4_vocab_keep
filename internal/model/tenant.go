package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ユーザーの基本情報
type Tenant struct {
	TenantID  uuid.UUID      `gorm:"type:uuid;primaryKey" json:"tenant_id"`
	Name      string         `gorm:"not null" json:"name"`
	Email     string         `gorm:"unique;not null" json:"email"`
	IsActive  bool           `json:"is_active" gorm:"default:false"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// GORM用のリレーション (JSONには含めない)
	Identities []Identity `gorm:"foreignKey:TenantID" json:"-"`
}

func (Tenant) TableName() string {
	return "tenants"
}

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

// UserResponse はクライアントに返すユーザー情報の構造体
type TenantResponse struct {
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}
