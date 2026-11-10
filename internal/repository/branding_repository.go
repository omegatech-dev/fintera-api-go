package repository

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// BrandingRepository defines the interface for branding data access
type BrandingRepository interface {
	Get(ctx context.Context) (*models.Branding, error)
	Upsert(ctx context.Context, branding *models.Branding) error
}

type brandingRepository struct {
	db *gorm.DB
}

// NewBrandingRepository creates a new branding repository
func NewBrandingRepository(db *gorm.DB) BrandingRepository {
	return &brandingRepository{db: db}
}

// Get returns the singleton branding row. It returns gorm.ErrRecordNotFound
// when the row does not exist yet.
func (r *brandingRepository) Get(ctx context.Context) (*models.Branding, error) {
	var branding models.Branding
	err := r.db.WithContext(ctx).First(&branding, 1).Error
	if err != nil {
		return nil, err
	}
	return &branding, nil
}

// Upsert inserts or updates the singleton branding row
func (r *brandingRepository) Upsert(ctx context.Context, branding *models.Branding) error {
	branding.ID = 1
	return r.db.WithContext(ctx).Save(branding).Error
}
