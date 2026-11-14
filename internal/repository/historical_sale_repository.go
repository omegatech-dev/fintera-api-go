package repository

import (
	"context"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// HistoricalSaleQuery describes the filters of the historical-sale list.
type HistoricalSaleQuery struct {
	*ListQuery
	CreatorID        uint
	ValidationStatus string
}

// HistoricalSaleRepository stores migrated sales. It never touches lots,
// contracts or assignments.
type HistoricalSaleRepository interface {
	Create(ctx context.Context, sale *models.HistoricalSale) error
	Update(ctx context.Context, sale *models.HistoricalSale) error
	FindByID(ctx context.Context, id uint) (*models.HistoricalSale, error)
	FindByIDWithDetails(ctx context.Context, id uint) (*models.HistoricalSale, error)
	List(ctx context.Context, query *HistoricalSaleQuery) ([]models.HistoricalSale, int64, error)
	ListForReport(ctx context.Context, creatorID *uint, startDate, endDate *time.Time) ([]models.HistoricalSale, error)
	ExistsByIdentity(ctx context.Context, creatorID uint, identity string, saleDate time.Time) (bool, error)
	UpdateValidation(ctx context.Context, id uint, status string, validatedBy uint, at time.Time, reason *string) error
}

type historicalSaleRepository struct {
	db *gorm.DB
}

// NewHistoricalSaleRepository creates the historical-sale repository.
func NewHistoricalSaleRepository(db *gorm.DB) HistoricalSaleRepository {
	return &historicalSaleRepository{db: db}
}

func (r *historicalSaleRepository) Create(ctx context.Context, sale *models.HistoricalSale) error {
	return r.db.WithContext(ctx).Create(sale).Error
}

func (r *historicalSaleRepository) Update(ctx context.Context, sale *models.HistoricalSale) error {
	return r.db.WithContext(ctx).Omit(clause.Associations).Save(sale).Error
}

func (r *historicalSaleRepository) FindByID(ctx context.Context, id uint) (*models.HistoricalSale, error) {
	var sale models.HistoricalSale
	if err := r.db.WithContext(ctx).First(&sale, id).Error; err != nil {
		return nil, err
	}
	return &sale, nil
}

func (r *historicalSaleRepository) FindByIDWithDetails(ctx context.Context, id uint) (*models.HistoricalSale, error) {
	var sale models.HistoricalSale
	err := r.db.WithContext(ctx).
		Preload("Creator").
		Preload("Validator").
		First(&sale, id).Error
	if err != nil {
		return nil, err
	}
	return &sale, nil
}

func (r *historicalSaleRepository) List(ctx context.Context, query *HistoricalSaleQuery) ([]models.HistoricalSale, int64, error) {
	var sales []models.HistoricalSale
	var total int64

	db := r.db.WithContext(ctx).Model(&models.HistoricalSale{})
	if query.CreatorID > 0 {
		db = db.Where("created_by = ?", query.CreatorID)
	}
	if query.ValidationStatus != "" && query.ValidationStatus != "all" {
		db = db.Where("validation_status = ?", query.ValidationStatus)
	}
	if query.Search != "" {
		term := "%" + query.Search + "%"
		db = db.Where("client_name ILIKE ? OR client_identity ILIKE ? OR client_phone ILIKE ? OR project_name ILIKE ?",
			term, term, term, term)
	}
	if query.Filters != nil {
		if from := query.Filters["sale_from"]; from != "" {
			db = db.Where("sale_date >= ?", from)
		}
		if to := query.Filters["sale_to"]; to != "" {
			db = db.Where("sale_date <= ?", to)
		}
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortDir := "DESC"
	if query.SortDir == "asc" {
		sortDir = "ASC"
	}
	switch query.SortBy {
	case "sale_price":
		db = db.Order("sale_price " + sortDir)
	case "created_at":
		db = db.Order("created_at " + sortDir)
	default:
		db = db.Order("sale_date " + sortDir).Order("id " + sortDir)
	}

	if query.PerPage > 0 {
		db = db.Offset((query.Page - 1) * query.PerPage).Limit(query.PerPage)
	}

	err := db.Preload("Creator").Preload("Validator").Find(&sales).Error
	return sales, total, err
}

// ListForReport returns the non-rejected historical sales of a seller (or all
// sellers when creatorID is nil) whose sale date falls inside the period.
func (r *historicalSaleRepository) ListForReport(ctx context.Context, creatorID *uint, startDate, endDate *time.Time) ([]models.HistoricalSale, error) {
	db := r.db.WithContext(ctx).
		Model(&models.HistoricalSale{}).
		Where("validation_status <> ?", models.HistoricalSaleStatusRejected)
	if creatorID != nil {
		db = db.Where("created_by = ?", *creatorID)
	}
	if startDate != nil {
		db = db.Where("sale_date >= ?", *startDate)
	}
	if endDate != nil {
		db = db.Where("sale_date <= ?", *endDate)
	}
	var sales []models.HistoricalSale
	err := db.Preload("Creator").Order("sale_date ASC").Find(&sales).Error
	return sales, err
}

func (r *historicalSaleRepository) ExistsByIdentity(ctx context.Context, creatorID uint, identity string, saleDate time.Time) (bool, error) {
	if identity == "" {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.HistoricalSale{}).
		Where("created_by = ? AND client_identity = ? AND sale_date = ?",
			creatorID, identity, saleDate).
		Count(&count).Error
	return count > 0, err
}

func (r *historicalSaleRepository) UpdateValidation(ctx context.Context, id uint, status string, validatedBy uint, at time.Time, reason *string) error {
	return r.db.WithContext(ctx).
		Model(&models.HistoricalSale{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"validation_status": status,
			"validated_by":      validatedBy,
			"validated_at":      at,
			"rejection_reason":  reason,
		}).Error
}
