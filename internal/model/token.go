package model

import (
	"time"

	"github.com/google/uuid"
)

// UserVerificationToken はアカウント有効化用のトークン情報を保持します
type UserVerificationToken struct {
	Token     string    `gorm:"primaryKey"`
	TenantID  uuid.UUID `gorm:"type:uuid;not null"`
	ExpiresAt time.Time `gorm:"not null"`
}

func (UserVerificationToken) TableName() string {
	return "user_verification_tokens"
}

type PasswordResetToken struct {
	Token     string    `gorm:"primaryKey"`
	TenantID  uuid.UUID `gorm:"type:uuid;not null"`
	ExpiresAt time.Time `gorm:"not null"`
}

func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}
