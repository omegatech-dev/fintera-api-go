package repository

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// GeneralSettingsRepository defines data access for the settings singleton.
type GeneralSettingsRepository interface {
	Get(ctx context.Context) (*models.GeneralSettings, error)
	Upsert(ctx context.Context, settings *models.GeneralSettings) error
}

type generalSettingsRepository struct {
	db *gorm.DB
}

// NewGeneralSettingsRepository creates the settings repository.
func NewGeneralSettingsRepository(db *gorm.DB) GeneralSettingsRepository {
	return &generalSettingsRepository{db: db}
}

// Get returns the singleton row, or gorm.ErrRecordNotFound when it is absent.
func (r *generalSettingsRepository) Get(ctx context.Context) (*models.GeneralSettings, error) {
	var settings models.GeneralSettings
	if err := r.db.WithContext(ctx).First(&settings, 1).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

// Upsert saves the singleton row.
func (r *generalSettingsRepository) Upsert(ctx context.Context, settings *models.GeneralSettings) error {
	settings.ID = 1
	return r.db.WithContext(ctx).Save(settings).Error
}
