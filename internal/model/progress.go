// internal/model/progress.go
package model

import (
	"time"

	"github.com/google/uuid"
)

type ProgressLevel int

const (
	Level1 ProgressLevel = iota + 1 // 1
	Level2                          // 2
	Level3                          // 3
)

// LearningProgress は単語の学習進捗を表します
type LearningProgress struct {
	ProgressID     uuid.UUID     `gorm:"type:uuid;primaryKey"`
	TenantID       uuid.UUID     `gorm:"type:uuid;not null;index:idx_tenant_word,unique"` // 複合ユニークインデックスの一部
	WordID         uuid.UUID     `gorm:"type:uuid;not null;index:idx_tenant_word,unique"` // 複合ユニークインデックスの一部
	Level          ProgressLevel `gorm:"not null;default:1"`
	NextReviewDate time.Time     `gorm:"not null;index"`
	LastReviewedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// GORMのDeletedAtは不要 (Wordの削除に追従)

	// 関連 (Preload用)
	Word *Word `gorm:"foreignKey:WordID;references:WordID" json:"-"`
}

func (LearningProgress) TableName() string {
	return "learning_progress"
}
