package repository

import (
	"context"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// ReferralRepository stores public link visits and the referral metrics that
// the seller dashboard reads.
type ReferralRepository interface {
	RecordVisit(ctx context.Context, visit *models.ReferralVisit) error
	CountVisitsSince(ctx context.Context, sellerID uint, since time.Time) (int64, error)
	CountAttributedReservationsSince(ctx context.Context, sellerID uint, since time.Time) (int64, error)
	SumGeneratedCommissionSince(ctx context.Context, sellerID uint, since time.Time) (float64, error)
	ListAttributedReservations(ctx context.Context, sellerID uint, limit int) ([]models.Contract, error)
}

type referralRepository struct {
	db *gorm.DB
}

// NewReferralRepository creates the referral repository.
func NewReferralRepository(db *gorm.DB) ReferralRepository {
	return &referralRepository{db: db}
}

func (r *referralRepository) RecordVisit(ctx context.Context, visit *models.ReferralVisit) error {
	return r.db.WithContext(ctx).Create(visit).Error
}

func (r *referralRepository) CountVisitsSince(ctx context.Context, sellerID uint, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ReferralVisit{}).
		Where("seller_id = ? AND visited_at >= ?", sellerID, since).
		Count(&count).Error
	return count, err
}

func (r *referralRepository) CountAttributedReservationsSince(ctx context.Context, sellerID uint, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Where("seller_id = ? AND assignment_origin = ? AND created_at >= ?",
			sellerID, models.AssignmentOriginReferral, since).
		Count(&count).Error
	return count, err
}

// SumGeneratedCommissionSince only counts commission from sales that already
// reached an approved state; pending reservations are not revenue yet.
func (r *referralRepository) SumGeneratedCommissionSince(ctx context.Context, sellerID uint, since time.Time) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Where("seller_id = ? AND assignment_origin = ? AND created_at >= ?",
			sellerID, models.AssignmentOriginReferral, since).
		Where("status IN ?", []string{
			models.ContractStatusApproved,
			models.ContractStatusSigned,
			models.ContractStatusClosed,
		}).
		Select("COALESCE(SUM(commission_amount), 0)").
		Scan(&total).Error
	return total, err
}

func (r *referralRepository) ListAttributedReservations(ctx context.Context, sellerID uint, limit int) ([]models.Contract, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var contracts []models.Contract
	err := r.db.WithContext(ctx).
		Preload("Lot.Project").
		Preload("ApplicantUser").
		Preload("Seller").
		Where("seller_id = ? AND assignment_origin = ?", sellerID, models.AssignmentOriginReferral).
		Order("created_at DESC").
		Limit(limit).
		Find(&contracts).Error
	return contracts, err
}
