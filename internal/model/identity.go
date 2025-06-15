package model

import "github.com/google/uuid"

const (
	AuthProviderLocal  = "local"
	AuthProviderGoogle = "google"
)

// Identity は認証情報を表します
type Identity struct {
	ID       uint      `gorm:"primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index"` // tenantsテーブルへの外部キー

	// どのプロバイダで、どのIDかを示す複合キー
	AuthProvider string `gorm:"type:varchar(50);not null;uniqueIndex:uq_identity_provider"`
	ProviderID   string `gorm:"not null;uniqueIndex:uq_identity_provider"` // localの場合はemail, googleの場合はsub

	// パスワードハッシュは local プロバイダの場合のみ使用
	PasswordHash *string `gorm:"default:null"`
}

func (Identity) TableName() string {
	return "identities"
}
