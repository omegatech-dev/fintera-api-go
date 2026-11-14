package repository

import (
	"context"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// SellerRepository groups the seller-specific queries that do not belong to the
// generic user repository: referral GUIDs and score fields.
type SellerRepository interface {
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindActiveByReferralGUID(ctx context.Context, guid string) (*models.User, error)
	ListActive(ctx context.Context, search string) ([]models.User, error)
	FindAllSellers(ctx context.Context) ([]models.User, error)
	UpdateReferralGUID(ctx context.Context, id uint, guid string) error
	UpdateScore(ctx context.Context, id uint, score int, level string) error
	SetScoreOverride(ctx context.Context, id uint, score int, level, reason string, actorID uint, at time.Time) error
	ClearScoreOverride(ctx context.Context, id uint) error
}

type sellerRepository struct {
	db *gorm.DB
}

// NewSellerRepository creates the seller repository.
func NewSellerRepository(db *gorm.DB) SellerRepository {
	return &sellerRepository{db: db}
}

func (r *sellerRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("discarded_at IS NULL").First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *sellerRepository) FindActiveByReferralGUID(ctx context.Context, guid string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("referral_guid = ? AND role = ? AND status = ? AND discarded_at IS NULL",
			guid, models.RoleSeller, models.StatusActive).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *sellerRepository) ListActive(ctx context.Context, search string) ([]models.User, error) {
	var users []models.User
	db := r.db.WithContext(ctx).
		Where("role = ? AND status = ? AND discarded_at IS NULL", models.RoleSeller, models.StatusActive)
	if search != "" {
		term := "%" + search + "%"
		db = db.Where("full_name ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR identity ILIKE ?",
			term, term, term, term)
	}
	err := db.Order("full_name ASC").Find(&users).Error
	return users, err
}

func (r *sellerRepository) FindAllSellers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).
		Where("role = ? AND discarded_at IS NULL", models.RoleSeller).
		Order("id ASC").
		Find(&users).Error
	return users, err
}

func (r *sellerRepository) UpdateReferralGUID(ctx context.Context, id uint, guid string) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("referral_guid", guid).Error
}

func (r *sellerRepository) UpdateScore(ctx context.Context, id uint, score int, level string) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"score":       score,
			"score_level": level,
		}).Error
}

func (r *sellerRepository) SetScoreOverride(ctx context.Context, id uint, score int, level, reason string, actorID uint, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"score":                 score,
			"score_level":           level,
			"score_override":        score,
			"score_override_reason": reason,
			"score_override_by":     actorID,
			"score_override_at":     at,
		}).Error
}

func (r *sellerRepository) ClearScoreOverride(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"score_override":        nil,
			"score_override_reason": nil,
			"score_override_by":     nil,
			"score_override_at":     nil,
		}).Error
}
