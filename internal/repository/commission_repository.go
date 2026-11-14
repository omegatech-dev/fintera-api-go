package repository

import (
	"context"
	"errors"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommissionRepository owns the global commission settings (stored on the
// general_settings singleton) and the per-seller rate exceptions.
type CommissionRepository interface {
	GetSettings(ctx context.Context) (*models.CommissionSettings, error)
	UpdateSettings(ctx context.Context, settings *models.CommissionSettings) error
	FindSellerRate(ctx context.Context, sellerID uint) (*models.SellerCommissionRate, error)
	FindSellerRateByID(ctx context.Context, id uint) (*models.SellerCommissionRate, error)
	ListSellerRates(ctx context.Context) ([]models.SellerCommissionRate, error)
	UpsertSellerRate(ctx context.Context, rate *models.SellerCommissionRate) error
	DeleteSellerRate(ctx context.Context, id uint) error
}

type commissionRepository struct {
	db *gorm.DB
}

// NewCommissionRepository creates the commission repository.
func NewCommissionRepository(db *gorm.DB) CommissionRepository {
	return &commissionRepository{db: db}
}

// GetSettings returns the singleton rates. When the row is absent it returns
// the documented defaults instead of an error.
func (r *commissionRepository) GetSettings(ctx context.Context) (*models.CommissionSettings, error) {
	var settings models.CommissionSettings
	if err := r.db.WithContext(ctx).First(&settings, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			defaults := models.DefaultCommissionSettings()
			return &defaults, nil
		}
		return nil, err
	}
	return &settings, nil
}

func (r *commissionRepository) UpdateSettings(ctx context.Context, settings *models.CommissionSettings) error {
	if settings.ID == 0 {
		settings.ID = 1
	}
	rateUpdates := map[string]interface{}{
		"commission_default_rate":            settings.CommissionDefaultRate,
		"commission_unassigned_rate":         settings.CommissionUnassignedRate,
		"commission_acceptance_window_hours": settings.CommissionAcceptanceWindowHours,
	}
	result := r.db.WithContext(ctx).
		Model(&models.CommissionSettings{}).
		Where("id = ?", settings.ID).
		Updates(rateUpdates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	// The singleton row is normally seeded. Insert it when a fresh database
	// skipped the seed so the rates are never silently lost.
	return r.db.WithContext(ctx).Create(&models.CommissionSettings{
		ID:                       settings.ID,
		CommissionDefaultRate:    settings.CommissionDefaultRate,
		CommissionUnassignedRate: settings.CommissionUnassignedRate,
	}).Error
}

func (r *commissionRepository) FindSellerRate(ctx context.Context, sellerID uint) (*models.SellerCommissionRate, error) {
	var rate models.SellerCommissionRate
	if err := r.db.WithContext(ctx).Where("seller_id = ?", sellerID).First(&rate).Error; err != nil {
		return nil, err
	}
	return &rate, nil
}

func (r *commissionRepository) FindSellerRateByID(ctx context.Context, id uint) (*models.SellerCommissionRate, error) {
	var rate models.SellerCommissionRate
	if err := r.db.WithContext(ctx).Preload("Seller").First(&rate, id).Error; err != nil {
		return nil, err
	}
	return &rate, nil
}

func (r *commissionRepository) ListSellerRates(ctx context.Context) ([]models.SellerCommissionRate, error) {
	var rates []models.SellerCommissionRate
	err := r.db.WithContext(ctx).
		Preload("Seller").
		Order("id ASC").
		Find(&rates).Error
	return rates, err
}

func (r *commissionRepository) UpsertSellerRate(ctx context.Context, rate *models.SellerCommissionRate) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "seller_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rate", "updated_at"}),
	}).Create(rate).Error
}

func (r *commissionRepository) DeleteSellerRate(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&models.SellerCommissionRate{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
