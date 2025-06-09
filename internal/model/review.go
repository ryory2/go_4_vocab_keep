// internal/model/review.go
package model

import "github.com/google/uuid"

// ReviewWordResponse は復習単語リストのレスポンスDTO
type ReviewWordResponse struct {
	WordID     uuid.UUID `json:"word_id"`
	Term       string    `json:"term"`
	Definition string    `json:"definition"` // 正解表示用に含める
	Level      int       `json:"level"`
}

// SubmitReviewRequest は復習結果送信リクエストのDTO
type SubmitReviewRequest struct {
	IsCorrect *bool `json:"is_correct" validate:"required"`
}
