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
	UserIDKey   ContextKey = "userID" // tenantIDと同じだが、意味合いを明確にする
)
