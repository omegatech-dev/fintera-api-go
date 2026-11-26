package repository

import (
	"context"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AssignmentRepository stores the assignment history and keeps the mirrored
// seller fields on contracts in sync with the current offer.
type AssignmentRepository interface {
	Create(ctx context.Context, assignment *models.Assignment) error
	FindByID(ctx context.Context, id uint) (*models.Assignment, error)
	FindByIDWithDetails(ctx context.Context, id uint) (*models.Assignment, error)
	FindByIDWithDetailsForUpdate(ctx context.Context, id uint) (*models.Assignment, error)
	FindPendingBySeller(ctx context.Context, sellerID uint) ([]models.Assignment, error)
	FindPendingAll(ctx context.Context) ([]models.Assignment, error)
	FindPendingByContract(ctx context.Context, contractID uint) ([]models.Assignment, error)
	FindByContract(ctx context.Context, contractID uint) ([]models.Assignment, error)
	FindOverduePending(ctx context.Context, now time.Time) ([]models.Assignment, error)
	UpdateStatusIfPending(ctx context.Context, id uint, status string, respondedAt time.Time) (bool, error)
	ExpiredSellerIDsForContract(ctx context.Context, contractID uint) ([]uint, error)
	ListAssignableSellers(ctx context.Context, excludeIDs []uint) ([]models.User, error)
	ListActiveSellers(ctx context.Context, excludeIDs []uint) ([]models.User, error)
	FindUnassignedAutomatic(ctx context.Context, limit int) ([]models.Contract, error)
	PendingLoadBySellerIDs(ctx context.Context, sellerIDs []uint) (map[uint]int64, error)
	LastAssignedAtBySellerIDs(ctx context.Context, sellerIDs []uint) (map[uint]*time.Time, error)
	ScoreOutcomesSince(ctx context.Context, since time.Time) ([]models.ScoreOutcome, error)
	UpdateContractAssignment(ctx context.Context, contractID uint, update ContractAssignmentUpdate) error
	CountSalesBySellerIDs(ctx context.Context, sellerIDs []uint) (map[uint]int64, error)
}

// ContractAssignmentUpdate mirrors an assignment change on the contract. A nil
// SellerID clears the current seller.
type ContractAssignmentUpdate struct {
	SellerID         *uint
	Origin           string
	Status           string
	AssignedAt       *time.Time
	AcceptDeadlineAt *time.Time
	RespondedAt      *time.Time
}

type assignmentRepository struct {
	db *gorm.DB
}

// NewAssignmentRepository creates the assignment repository.
func NewAssignmentRepository(db *gorm.DB) AssignmentRepository {
	return &assignmentRepository{db: db}
}

func (r *assignmentRepository) Create(ctx context.Context, assignment *models.Assignment) error {
	return r.db.WithContext(ctx).Create(assignment).Error
}

func (r *assignmentRepository) FindByID(ctx context.Context, id uint) (*models.Assignment, error) {
	var assignment models.Assignment
	if err := r.db.WithContext(ctx).First(&assignment, id).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *assignmentRepository) FindByIDWithDetails(ctx context.Context, id uint) (*models.Assignment, error) {
	var assignment models.Assignment
	err := r.db.WithContext(ctx).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("Seller").
		First(&assignment, id).Error
	if err != nil {
		return nil, err
	}
	return &assignment, nil
}

// FindByIDWithDetailsForUpdate locks the assignment row after the caller has
// acquired the contract lock, preserving the shared contract-first lock order.
func (r *assignmentRepository) FindByIDWithDetailsForUpdate(ctx context.Context, id uint) (*models.Assignment, error) {
	var locked models.Assignment
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		First(&locked, id).Error; err != nil {
		return nil, err
	}
	return r.FindByIDWithDetails(ctx, locked.ID)
}

func (r *assignmentRepository) FindPendingBySeller(ctx context.Context, sellerID uint) ([]models.Assignment, error) {
	var assignments []models.Assignment
	err := r.db.WithContext(ctx).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("Seller").
		Where("status = ? AND seller_id = ?", models.AssignmentStatusPending, sellerID).
		Where("(accept_deadline_at IS NULL OR accept_deadline_at > ?)", time.Now()).
		Order("accept_deadline_at ASC NULLS LAST").
		Find(&assignments).Error
	return assignments, err
}

func (r *assignmentRepository) FindPendingAll(ctx context.Context) ([]models.Assignment, error) {
	var assignments []models.Assignment
	err := r.db.WithContext(ctx).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("Seller").
		Where("status = ?", models.AssignmentStatusPending).
		Where("(accept_deadline_at IS NULL OR accept_deadline_at > ?)", time.Now()).
		Order("accept_deadline_at ASC NULLS LAST").
		Find(&assignments).Error
	return assignments, err
}

func (r *assignmentRepository) FindOverduePending(ctx context.Context, now time.Time) ([]models.Assignment, error) {
	var assignments []models.Assignment
	err := r.db.WithContext(ctx).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("Seller").
		Where("status = ? AND accept_deadline_at IS NOT NULL AND accept_deadline_at < ?",
			models.AssignmentStatusPending, now).
		Order("accept_deadline_at ASC").
		Find(&assignments).Error
	return assignments, err
}

// UpdateStatusIfPending performs the state transition only when the row is
// still pending. The boolean reports whether this call won the update, which
// keeps accept/reject idempotent and race-free.
func (r *assignmentRepository) UpdateStatusIfPending(ctx context.Context, id uint, status string, respondedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Assignment{}).
		Where("id = ? AND status = ?", id, models.AssignmentStatusPending).
		Updates(map[string]interface{}{
			"status":       status,
			"responded_at": respondedAt,
		})
	return result.RowsAffected > 0, result.Error
}

func (r *assignmentRepository) ExpiredSellerIDsForContract(ctx context.Context, contractID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).
		Model(&models.Assignment{}).
		Where("contract_id = ? AND status IN ?", contractID,
			[]string{models.AssignmentStatusExpired, models.AssignmentStatusSuperseded}).
		Distinct("seller_id").
		Pluck("seller_id", &ids).Error
	return ids, err
}

// FindByContract returns the assignment history of one contract, newest first.
func (r *assignmentRepository) FindByContract(ctx context.Context, contractID uint) ([]models.Assignment, error) {
	var assignments []models.Assignment
	err := r.db.WithContext(ctx).
		Preload("Seller").
		Where("contract_id = ?", contractID).
		Order("id DESC").
		Find(&assignments).Error
	return assignments, err
}

// FindPendingByContract returns the active offers of one contract.
func (r *assignmentRepository) FindPendingByContract(ctx context.Context, contractID uint) ([]models.Assignment, error) {
	var assignments []models.Assignment
	err := r.db.WithContext(ctx).
		Preload("Seller").
		Where("contract_id = ? AND status = ?", contractID, models.AssignmentStatusPending).
		Find(&assignments).Error
	return assignments, err
}

// ListAssignableSellers returns active sellers whose level is high or medium.
// Low performers never enter the automatic pool.
func (r *assignmentRepository) ListAssignableSellers(ctx context.Context, excludeIDs []uint) ([]models.User, error) {
	var sellers []models.User
	db := r.db.WithContext(ctx).
		Where("role = ? AND status = ? AND discarded_at IS NULL", models.RoleSeller, models.StatusActive).
		Where("score_level IN ?", []string{models.ScoreLevelHigh, models.ScoreLevelMedium})
	if len(excludeIDs) > 0 {
		db = db.Where("id NOT IN ?", excludeIDs)
	}
	err := db.Order("full_name ASC").Find(&sellers).Error
	return sellers, err
}

// PendingLoadBySellerIDs returns how many active offers each seller holds.
// ListActiveSellers returns every active seller regardless of score level. It
// is the fallback pool for automatic assignment: keeping low performers out of
// the primary pool is intentional, but excluding them entirely strands
// reservations whenever nobody is high/medium.
func (r *assignmentRepository) ListActiveSellers(ctx context.Context, excludeIDs []uint) ([]models.User, error) {
	var sellers []models.User
	db := r.db.WithContext(ctx).
		Where("role = ? AND status = ? AND discarded_at IS NULL", models.RoleSeller, models.StatusActive)
	if len(excludeIDs) > 0 {
		db = db.Where("id NOT IN ?", excludeIDs)
	}
	err := db.Order("full_name ASC").Find(&sellers).Error
	return sellers, err
}

// FindUnassignedAutomatic returns live reservations that should carry an
// automatic seller but currently have none. Historical or manually reassigned
// contracts are out of scope on purpose.
func (r *assignmentRepository) FindUnassignedAutomatic(ctx context.Context, limit int) ([]models.Contract, error) {
	if limit <= 0 {
		limit = 50
	}
	var contracts []models.Contract
	err := r.db.WithContext(ctx).
		Where("seller_id IS NULL").
		Where("assignment_origin IN ?", []string{models.AssignmentOriginAutomatic, ""}).
		Where("status IN ?", []string{models.ContractStatusPending, models.ContractStatusSubmitted}).
		Order("id ASC").
		Limit(limit).
		Find(&contracts).Error
	return contracts, err
}

func (r *assignmentRepository) PendingLoadBySellerIDs(ctx context.Context, sellerIDs []uint) (map[uint]int64, error) {
	loads := make(map[uint]int64, len(sellerIDs))
	if len(sellerIDs) == 0 {
		return loads, nil
	}
	type row struct {
		SellerID uint
		Count    int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&models.Assignment{}).
		Select("seller_id, count(*) as count").
		Where("status = ? AND seller_id IN ?", models.AssignmentStatusPending, sellerIDs).
		Group("seller_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		loads[item.SellerID] = item.Count
	}
	return loads, nil
}

// LastAssignedAtBySellerIDs returns the rotation cursor for each seller.
func (r *assignmentRepository) LastAssignedAtBySellerIDs(ctx context.Context, sellerIDs []uint) (map[uint]*time.Time, error) {
	last := make(map[uint]*time.Time, len(sellerIDs))
	if len(sellerIDs) == 0 {
		return last, nil
	}
	type row struct {
		SellerID uint
		Last     *time.Time
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&models.Assignment{}).
		Select("seller_id, MAX(assigned_at) as last").
		Where("seller_id IN ?", sellerIDs).
		Group("seller_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		last[item.SellerID] = item.Last
	}
	return last, nil
}

// ScoreOutcomesSince aggregates accepted/expired offers per seller.
func (r *assignmentRepository) ScoreOutcomesSince(ctx context.Context, since time.Time) ([]models.ScoreOutcome, error) {
	type row struct {
		SellerID uint
		Status   string
		Count    int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&models.Assignment{}).
		Select("seller_id, status, count(*) as count").
		Where("assigned_at >= ? AND status IN ? AND origin <> ?", since,
			[]string{models.AssignmentStatusAccepted, models.AssignmentStatusExpired},
			models.AssignmentOriginReferral).
		Group("seller_id, status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	outcomes := make(map[uint]*models.ScoreOutcome)
	order := make([]uint, 0, len(rows))
	for _, item := range rows {
		outcome, ok := outcomes[item.SellerID]
		if !ok {
			outcome = &models.ScoreOutcome{SellerID: item.SellerID}
			outcomes[item.SellerID] = outcome
			order = append(order, item.SellerID)
		}
		if item.Status == models.AssignmentStatusAccepted {
			outcome.Accepted = item.Count
		} else {
			outcome.Expired = item.Count
		}
	}
	result := make([]models.ScoreOutcome, 0, len(order))
	for _, id := range order {
		result = append(result, *outcomes[id])
	}
	return result, nil
}

func (r *assignmentRepository) UpdateContractAssignment(ctx context.Context, contractID uint, update ContractAssignmentUpdate) error {
	return r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Where("id = ?", contractID).
		Updates(map[string]interface{}{
			"seller_id":          update.SellerID,
			"assignment_origin":  update.Origin,
			"assignment_status":  update.Status,
			"assigned_at":        update.AssignedAt,
			"accept_deadline_at": update.AcceptDeadlineAt,
			"responded_at":       update.RespondedAt,
		}).Error
}

// CountSalesBySellerIDs counts closed sales (approved or later) per seller.
func (r *assignmentRepository) CountSalesBySellerIDs(ctx context.Context, sellerIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(sellerIDs))
	if len(sellerIDs) == 0 {
		return counts, nil
	}
	type row struct {
		SellerID uint
		Count    int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Select("seller_id, count(*) as count").
		Where("seller_id IN ?", sellerIDs).
		Where("status IN ?", []string{
			models.ContractStatusApproved,
			models.ContractStatusSigned,
			models.ContractStatusClosed,
		}).
		Group("seller_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		counts[item.SellerID] = item.Count
	}
	return counts, nil
}
