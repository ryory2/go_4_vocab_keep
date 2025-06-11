package repository

import (
	"context"
	"errors"
	"fmt"

	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"

	"gorm.io/gorm"
)

type TokenRepository interface {
	CreateVerificationToken(ctx context.Context, db *gorm.DB, token *model.UserVerificationToken) error
	FindVerificationToken(ctx context.Context, db *gorm.DB, token string) (*model.UserVerificationToken, error)
	DeleteVerificationToken(ctx context.Context, db *gorm.DB, token string) error
	CreatePasswordResetToken(ctx context.Context, db *gorm.DB, token *model.PasswordResetToken) error
	FindPasswordResetToken(ctx context.Context, db *gorm.DB, token string) (*model.PasswordResetToken, error)
	DeletePasswordResetToken(ctx context.Context, db *gorm.DB, token string) error
}

type gormTokenRepository struct{}

func NewGormTokenRepository() TokenRepository {
	return &gormTokenRepository{}
}

func (r *gormTokenRepository) CreateVerificationToken(ctx context.Context, db *gorm.DB, token *model.UserVerificationToken) error {
	logger := middleware.GetLogger(ctx)
	if err := db.WithContext(ctx).Create(token).Error; err != nil {
		logger.Error("Failed to create verification token", "error", err)
		return fmt.Errorf("gormTokenRepository.CreateVerificationToken: %w", err)
	}
	return nil
}

func (r *gormTokenRepository) FindVerificationToken(ctx context.Context, db *gorm.DB, tokenStr string) (*model.UserVerificationToken, error) {
	logger := middleware.GetLogger(ctx)
	var token model.UserVerificationToken
	if err := db.WithContext(ctx).Where("token = ?", tokenStr).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		logger.Error("Failed to find verification token", "error", err)
		return nil, fmt.Errorf("gormTokenRepository.FindVerificationToken: %w", err)
	}
	return &token, nil
}

func (r *gormTokenRepository) DeleteVerificationToken(ctx context.Context, db *gorm.DB, tokenStr string) error {
	logger := middleware.GetLogger(ctx)
	result := db.WithContext(ctx).Where("token = ?", tokenStr).Delete(&model.UserVerificationToken{})
	if result.Error != nil {
		logger.Error("Failed to delete verification token", "error", result.Error)
		return fmt.Errorf("gormTokenRepository.DeleteVerificationToken: %w", result.Error)
	}
	return nil
}

func (r *gormTokenRepository) CreatePasswordResetToken(ctx context.Context, db *gorm.DB, token *model.PasswordResetToken) error {
	logger := middleware.GetLogger(ctx)
	if err := db.WithContext(ctx).Create(token).Error; err != nil {
		logger.Error("Failed to create password reset token", "error", err)
		return fmt.Errorf("gormTokenRepository.CreatePasswordResetToken: %w", err)
	}
	return nil
}

func (r *gormTokenRepository) FindPasswordResetToken(ctx context.Context, db *gorm.DB, tokenStr string) (*model.PasswordResetToken, error) {
	logger := middleware.GetLogger(ctx)
	var token model.PasswordResetToken
	if err := db.WithContext(ctx).Where("token = ?", tokenStr).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		logger.Error("Failed to find password reset token", "error", err)
		return nil, fmt.Errorf("gormTokenRepository.FindPasswordResetToken: %w", err)
	}
	return &token, nil
}

func (r *gormTokenRepository) DeletePasswordResetToken(ctx context.Context, db *gorm.DB, tokenStr string) error {
	logger := middleware.GetLogger(ctx)
	result := db.WithContext(ctx).Where("token = ?", tokenStr).Delete(&model.PasswordResetToken{})
	if result.Error != nil {
		logger.Error("Failed to delete password reset token", "error", result.Error)
		return fmt.Errorf("gormTokenRepository.DeletePasswordResetToken: %w", result.Error)
	}
	return nil
}
