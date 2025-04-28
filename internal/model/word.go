// internal/model/word.go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Word は単語とその定義を表します
type Word struct {
	WordID     uuid.UUID      `gorm:"type:uuid;primaryKey" json:"word_id"`
	TenantID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"-"`
	Term       string         `gorm:"not null" json:"term"`       // 単語
	Definition string         `gorm:"not null" json:"definition"` // 単語の定義
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"` // 論理削除用

	// 関連 (Preload用)
	LearningProgress *LearningProgress `gorm:"foreignKey:WordID;references:WordID" json:"-"`
}

func (Word) TableName() string {
	return "words"
}

// CreateWordRequest は単語作成リクエストのDTO
type CreateWordRequest struct {
	Term       string `json:"term" validate:"required"`
	Definition string `json:"definition" validate:"required"`
}

// UpdateWordRequest は単語更新リクエストのDTO
type UpdateWordRequest struct {
	Term       *string `json:"term"`
	Definition *string `json:"definition"`
}
