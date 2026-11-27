package repository

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommissionRepository owns the global commission settings (stored on the
// general_settings singleton), the per-seller rate exceptions and the seller
// commission credits created when a paid commission is reversed.
type CommissionRepository interface {
	GetSettings(ctx context.Context) (*models.CommissionSettings, error)
	UpdateSettings(ctx context.Context, settings *models.CommissionSettings) error
	FindSellerRate(ctx context.Context, sellerID uint) (*models.SellerCommissionRate, error)
	FindSellerRateByID(ctx context.Context, id uint) (*models.SellerCommissionRate, error)
	ListSellerRates(ctx context.Context) ([]models.SellerCommissionRate, error)
	UpsertSellerRate(ctx context.Context, rate *models.SellerCommissionRate) error
	DeleteSellerRate(ctx context.Context, id uint) error
	CreateCredit(ctx context.Context, credit *models.SellerCommissionCredit) error
	FindCreditByID(ctx context.Context, id uint) (*models.SellerCommissionCredit, error)
	ListCredits(ctx context.Context, sellerID uint) ([]models.SellerCommissionCredit, error)
	// ListPendingCreditsBySeller returns the remaining pending credit amount per
	// seller, keyed by seller ID. Remaining is amount - applied_amount, so
	// partially applied credits only contribute their unused balance.
	ListPendingCreditsBySeller(ctx context.Context) (map[uint]float64, error)
	SettleCredit(ctx context.Context, id, actorID uint, settledAt time.Time) (bool, error)
	// ListApplicationsByCredits returns the active applications (reversed_at IS
	// NULL) for the given credits, keyed by credit ID.
	ListApplicationsByCredits(ctx context.Context, creditIDs []uint) (map[uint][]models.CreditApplication, error)
	// ListAppliedTotalByContractIDs returns the total active credit application
	// amount per contract, keyed by contract ID.
	ListAppliedTotalByContractIDs(ctx context.Context, contractIDs []uint) (map[uint]float64, error)
	// PayCommissionWithCredits marks the contract commission paid and consumes
	// the seller's pending credits FIFO, atomically.
	PayCommissionWithCredits(ctx context.Context, contractID, actorID uint, commission float64) (*models.CommissionPayout, error)
	// PreviewCommissionPayout computes the same allocation without writing.
	PreviewCommissionPayout(ctx context.Context, sellerID *uint, commission float64) (*models.CommissionPayoutPreview, error)
	// RestoreAndCreateCredit reverses the contract's active applications and
	// creates the credit for the cash that remains recoverable.
	RestoreAndCreateCredit(ctx context.Context, input CommissionCompensationInput) (*models.CommissionCompensation, error)
}

// CommissionCompensationInput describes the reversal of a paid commission when
// the contract is cancelled or reassigned.
type CommissionCompensationInput struct {
	ContractID uint
	SellerID   uint
	Gross      float64
	Source     string
	ActorID    uint
	ClearPaid  bool
}

// Errors returned by the transactional commission payout.
var (
	ErrCommissionAlreadyPaid = errors.New("la comisión ya fue marcada como pagada")
	ErrCommissionNotPayable  = errors.New("solo se puede pagar la comisión de contratos aprobados o cerrados")
)

type commissionRepository struct {
	db *gorm.DB
}

const maxCreditApplicationsPerCredit = 20

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

func (r *commissionRepository) CreateCredit(ctx context.Context, credit *models.SellerCommissionCredit) error {
	if credit.Status == "" {
		credit.Status = models.CommissionCreditStatusPending
	}
	return r.db.WithContext(ctx).Create(credit).Error
}

func (r *commissionRepository) FindCreditByID(ctx context.Context, id uint) (*models.SellerCommissionCredit, error) {
	var credit models.SellerCommissionCredit
	if err := r.db.WithContext(ctx).
		Preload("Seller").
		Preload("Contract").
		First(&credit, id).Error; err != nil {
		return nil, err
	}
	return &credit, nil
}

// ListCredits returns every credit, or only one seller's credits when sellerID
// is greater than zero.
func (r *commissionRepository) ListCredits(ctx context.Context, sellerID uint) ([]models.SellerCommissionCredit, error) {
	var credits []models.SellerCommissionCredit
	query := r.db.WithContext(ctx).
		Preload("Seller").
		Preload("Contract").
		Order("created_at DESC, id DESC")
	if sellerID > 0 {
		query = query.Where("seller_id = ?", sellerID)
	}
	err := query.Find(&credits).Error
	return credits, err
}

// ListPendingCreditsBySeller aggregates the remaining balance of credits that
// are still pending. It is a single grouped query so the commissions report
// does not need one lookup per seller.
func (r *commissionRepository) ListPendingCreditsBySeller(ctx context.Context) (map[uint]float64, error) {
	var rows []struct {
		SellerID uint
		Total    float64
	}
	err := r.db.WithContext(ctx).
		Model(&models.SellerCommissionCredit{}).
		Select("seller_id AS seller_id, COALESCE(SUM(amount - applied_amount), 0) AS total").
		Where("status = ? AND amount > applied_amount", models.CommissionCreditStatusPending).
		Group("seller_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	credits := make(map[uint]float64, len(rows))
	for _, row := range rows {
		credits[row.SellerID] = row.Total
	}
	return credits, nil
}

// SettleCredit marks a pending credit as settled. The conditional update keeps
// repeated applies idempotent: the second call reports no affected rows.
// applied_amount is set to the full amount so the settled-full constraint holds
// for the legacy manual path as well.
func (r *commissionRepository) SettleCredit(ctx context.Context, id, actorID uint, settledAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.SellerCommissionCredit{}).
		Where("id = ? AND status = ?", id, models.CommissionCreditStatusPending).
		Updates(map[string]interface{}{
			"status":         models.CommissionCreditStatusSettled,
			"applied_amount": gorm.Expr("amount"),
			"settled_at":     settledAt,
			"settled_by":     actorID,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// ListApplicationsByCredits returns the active applications of the given
// credits. One query for the whole page keeps credit listings cheap.
func (r *commissionRepository) ListApplicationsByCredits(ctx context.Context, creditIDs []uint) (map[uint][]models.CreditApplication, error) {
	result := make(map[uint][]models.CreditApplication, len(creditIDs))
	if len(creditIDs) == 0 {
		return result, nil
	}
	type applicationRow struct {
		ID                uint
		CreditID          uint
		ContractID        uint
		Amount            float64
		ActorID           *uint
		CreatedAt         time.Time
		ApplicationsTotal int
		ContractGUID      string
	}
	var rows []applicationRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, credit_id, contract_id, amount, actor_id, created_at,
		       applications_total, contract_guid
		FROM (
			SELECT ca.id, ca.credit_id, ca.contract_id, ca.amount, ca.actor_id,
			       ca.created_at,
			       COUNT(*) OVER (PARTITION BY ca.credit_id) AS applications_total,
			       c.guid AS contract_guid,
			       ROW_NUMBER() OVER (
			           PARTITION BY ca.credit_id
			           ORDER BY ca.created_at DESC, ca.id DESC
			       ) AS application_rank
			FROM credit_applications ca
			JOIN contracts c ON c.id = ca.contract_id
			WHERE ca.credit_id IN ? AND ca.reversed_at IS NULL
		) active_applications
		WHERE application_rank <= ?
		ORDER BY credit_id ASC, created_at DESC, id DESC
	`, creditIDs, maxCreditApplicationsPerCredit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		row := &rows[i]
		result[row.CreditID] = append(result[row.CreditID], models.CreditApplication{
			ID:                row.ID,
			CreditID:          row.CreditID,
			ContractID:        row.ContractID,
			Amount:            row.Amount,
			ActorID:           row.ActorID,
			CreatedAt:         row.CreatedAt,
			ApplicationsTotal: row.ApplicationsTotal,
			Contract:          &models.Contract{ID: row.ContractID, GUID: row.ContractGUID},
		})
	}
	return result, nil
}

// ListAppliedTotalByContractIDs returns the total amount of active credit
// applications per contract, so reports can show how much of a paid commission
// was settled with seller credits instead of cash.
func (r *commissionRepository) ListAppliedTotalByContractIDs(ctx context.Context, contractIDs []uint) (map[uint]float64, error) {
	result := make(map[uint]float64, len(contractIDs))
	if len(contractIDs) == 0 {
		return result, nil
	}
	type appliedRow struct {
		ContractID uint
		Total      float64
	}
	var rows []appliedRow
	err := r.db.WithContext(ctx).
		Model(&models.CreditApplication{}).
		Select("contract_id, COALESCE(SUM(amount), 0) AS total").
		Where("contract_id IN ? AND reversed_at IS NULL", contractIDs).
		Group("contract_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ContractID] = models.RoundMoney(row.Total)
	}
	return result, nil
}

// PayCommissionWithCredits marks the contract commission paid and consumes the
// seller's pending credits in FIFO order inside one transaction. The contract
// row lock makes concurrent payouts serial; the second request sees the paid
// timestamp and fails without touching credits.
func (r *commissionRepository) PayCommissionWithCredits(ctx context.Context, contractID, actorID uint, commission float64) (*models.CommissionPayout, error) {
	payout := &models.CommissionPayout{
		Commission:   models.RoundMoney(commission),
		Applications: []models.CreditApplicationResponse{},
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var contract models.Contract
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "guid", "seller_id", "status", "commission_amount", "commission_paid_at").
			First(&contract, contractID).Error; err != nil {
			return err
		}
		if contract.CommissionPaidAt != nil {
			return ErrCommissionAlreadyPaid
		}
		if contract.Status != models.ContractStatusApproved && contract.Status != models.ContractStatusClosed {
			return ErrCommissionNotPayable
		}
		if contract.SellerID != nil {
			sellerID := *contract.SellerID
			payout.SellerID = &sellerID
		}
		// The service may have loaded the contract before a concurrent
		// reassignment acquired the row lock. Use the amount frozen on the
		// locked contract when available so payout amount and seller always match.
		if contract.CommissionAmount > models.MoneyEpsilon {
			payout.Commission = models.RoundMoney(contract.CommissionAmount)
		}
		if payout.Commission <= models.MoneyEpsilon {
			return ErrCommissionNotPayable
		}

		now := time.Now()
		actor := actorPointer(actorID)

		if contract.SellerID != nil {
			credits, err := lockApplicableCredits(tx, *contract.SellerID)
			if err != nil {
				return err
			}
			available := 0.0
			byID := make(map[uint]*models.SellerCommissionCredit, len(credits))
			for i := range credits {
				available += credits[i].RemainingAmount()
				byID[credits[i].ID] = &credits[i]
			}
			available = models.RoundMoney(available)

			for _, allocation := range models.PlanCreditAllocations(payout.Commission, credits) {
				credit := byID[allocation.CreditID]
				if credit == nil {
					continue
				}
				newApplied := models.RoundMoney(credit.AppliedAmount + allocation.Amount)
				updates := map[string]interface{}{"applied_amount": newApplied}
				if newApplied >= credit.Amount-models.MoneyEpsilon {
					updates["status"] = models.CommissionCreditStatusSettled
					updates["settled_at"] = now
					updates["settled_by"] = actorID
				}
				if err := tx.Model(&models.SellerCommissionCredit{}).
					Where("id = ?", credit.ID).
					Updates(updates).Error; err != nil {
					return err
				}

				application := models.CreditApplication{
					CreditID:   credit.ID,
					ContractID: contract.ID,
					Amount:     allocation.Amount,
					ActorID:    actor,
					CreatedAt:  now,
				}
				if err := tx.Create(&application).Error; err != nil {
					return err
				}
				// The response needs the destination contract GUID.
				application.Contract = &contract

				payout.Applications = append(payout.Applications, application.ToResponse())
				payout.AppliedTotal = models.RoundMoney(payout.AppliedTotal + allocation.Amount)
			}

			if payout.AppliedTotal > payout.Commission {
				payout.AppliedTotal = payout.Commission
			}
			payout.RemainingCredit = models.RoundMoney(available - payout.AppliedTotal)
			if payout.RemainingCredit < 0 {
				payout.RemainingCredit = 0
			}
		}

		payout.NetCash = models.RoundMoney(payout.Commission - payout.AppliedTotal)
		if payout.NetCash < 0 {
			payout.NetCash = 0
		}

		return tx.Model(&models.Contract{}).
			Where("id = ?", contract.ID).
			Updates(map[string]interface{}{
				"commission_paid_at": now,
				"commission_paid_by": actorID,
				"commission_amount":  payout.Commission,
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return payout, nil
}

// PreviewCommissionPayout computes the allocation that PayCommissionWithCredits
// would perform, without writing anything.
func (r *commissionRepository) PreviewCommissionPayout(ctx context.Context, sellerID *uint, commission float64) (*models.CommissionPayoutPreview, error) {
	preview := &models.CommissionPayoutPreview{
		Commission: models.RoundMoney(commission),
	}
	if sellerID == nil {
		preview.NetCash = preview.Commission
		return preview, nil
	}

	var available float64
	if err := r.db.WithContext(ctx).
		Model(&models.SellerCommissionCredit{}).
		Select("COALESCE(SUM(amount - applied_amount), 0)").
		Where("seller_id = ? AND status = ? AND amount > applied_amount",
			*sellerID, models.CommissionCreditStatusPending).
		Scan(&available).Error; err != nil {
		return nil, err
	}

	preview.AvailableCredit = models.RoundMoney(available)
	preview.Applicable = models.RoundMoney(math.Min(preview.AvailableCredit, preview.Commission))
	preview.NetCash = models.RoundMoney(preview.Commission - preview.Applicable)
	preview.RemainingCredit = models.RoundMoney(preview.AvailableCredit - preview.Applicable)
	return preview, nil
}

// RestoreAndCreateCredit reverses the contract's active applications, restores
// the credit balances and creates a new credit for the cash that was actually
// paid. When there were no applications (legacy payouts) the new credit covers
// the full gross amount, matching the old behavior.
func (r *commissionRepository) RestoreAndCreateCredit(ctx context.Context, input CommissionCompensationInput) (*models.CommissionCompensation, error) {
	result := &models.CommissionCompensation{}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the contract first so a concurrent payout and this reversal use
		// the same lock order (contract, then credits).
		var contract models.Contract
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "commission_paid_at").
			First(&contract, input.ContractID).Error; err != nil {
			return err
		}
		// A competing reassignment may have completed while this request was
		// waiting for the contract lock. Its transaction clears the paid marker,
		// so do not restore the same applications or create a duplicate credit.
		if input.ClearPaid && contract.CommissionPaidAt == nil {
			return nil
		}

		var applications []models.CreditApplication
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("contract_id = ? AND reversed_at IS NULL", input.ContractID).
			Order("credit_id ASC, id ASC").
			Find(&applications).Error; err != nil {
			return err
		}

		now := time.Now()
		actor := actorPointer(input.ActorID)
		for i := range applications {
			application := &applications[i]

			var credit models.SellerCommissionCredit
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&credit, application.CreditID).Error; err != nil {
				return err
			}

			newApplied := models.RoundMoney(credit.AppliedAmount - application.Amount)
			if newApplied < 0 {
				newApplied = 0
			}
			updates := map[string]interface{}{"applied_amount": newApplied}
			if newApplied < credit.Amount-models.MoneyEpsilon {
				updates["status"] = models.CommissionCreditStatusPending
				updates["settled_at"] = nil
				updates["settled_by"] = nil
			}
			if err := tx.Model(&models.SellerCommissionCredit{}).
				Where("id = ?", credit.ID).
				Updates(updates).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.CreditApplication{}).
				Where("id = ?", application.ID).
				Updates(map[string]interface{}{
					"reversed_at": now,
					"reversed_by": actor,
				}).Error; err != nil {
				return err
			}

			result.Restored = models.RoundMoney(result.Restored + application.Amount)
		}

		if result.Restored > input.Gross {
			result.Restored = models.RoundMoney(input.Gross)
		}
		result.NetCash = models.RoundMoney(input.Gross - result.Restored)
		if result.NetCash < 0 {
			result.NetCash = 0
		}

		if input.ClearPaid {
			if err := tx.Model(&models.Contract{}).
				Where("id = ?", contract.ID).
				Updates(map[string]interface{}{
					"commission_paid_at": nil,
					"commission_paid_by": nil,
				}).Error; err != nil {
				return err
			}
		}

		if result.NetCash <= models.MoneyEpsilon || input.SellerID == 0 {
			return nil
		}

		source := input.Source
		if source == "" {
			source = models.CommissionCreditSourceCancellation
		}
		credit := &models.SellerCommissionCredit{
			SellerID:   input.SellerID,
			ContractID: input.ContractID,
			Amount:     result.NetCash,
			Status:     models.CommissionCreditStatusPending,
			Source:     source,
		}
		if err := tx.Create(credit).Error; err != nil {
			return err
		}
		result.Credit = credit
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// lockApplicableCredits loads the seller's pending credits with a row lock in
// the order they must be consumed. FIFO uses created_at and id so the order is
// stable across equal timestamps.
func lockApplicableCredits(tx *gorm.DB, sellerID uint) ([]models.SellerCommissionCredit, error) {
	var credits []models.SellerCommissionCredit
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("seller_id = ? AND status = ? AND amount > applied_amount",
			sellerID, models.CommissionCreditStatusPending).
		Order("created_at ASC, id ASC").
		Find(&credits).Error
	return credits, err
}

// actorPointer converts the system actor (0) to NULL so the audit foreign key
// stays valid.
func actorPointer(actorID uint) *uint {
	if actorID == 0 {
		return nil
	}
	return &actorID
}
