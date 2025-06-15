//go:generate mockery --name IdentityRepository --output ./mocks --outpkg mocks --case=underscore
package repository

import (
	"context"
	"errors"
	"fmt"
	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"

	"gorm.io/gorm"
)

type IdentityRepository interface {
	Create(ctx context.Context, db *gorm.DB, identity *model.Identity) error
	FindByProvider(ctx context.Context, db *gorm.DB, authProvider string, providerID string) (*model.Identity, error)
}

type gormIdentityRepository struct{}

func NewGormIdentityRepository() IdentityRepository {
	return &gormIdentityRepository{}
}

func (r *gormIdentityRepository) Create(ctx context.Context, db *gorm.DB, identity *model.Identity) error {
	logger := middleware.GetLogger(ctx)
	result := db.WithContext(ctx).Create(identity)
	if result.Error != nil {
		logger.Error(
			"Error creating identity in DB",
			"error", result.Error,
			"auth_provider", identity.AuthProvider,
			"provider_id", identity.ProviderID,
		)
		return fmt.Errorf("gormIdentityRepository.Create: %w", result.Error)
	}
	return nil
}

func (r *gormIdentityRepository) FindByProvider(ctx context.Context, db *gorm.DB, authProvider string, providerID string) (*model.Identity, error) {
	logger := middleware.GetLogger(ctx)
	var identity model.Identity

	result := db.WithContext(ctx).
		Where("auth_provider = ? AND provider_id = ?", authProvider, providerID).
		First(&identity)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		logger.Error(
			"Error finding identity by provider in DB",
			"error", result.Error,
			"auth_provider", authProvider,
			"provider_id", providerID,
		)
		return nil, fmt.Errorf("gormIdentityRepository.FindByProvider: %w", result.Error)
	}
	return &identity, nil
}
