package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"gorm.io/gorm"
)

// CommissionService resolves commission rates, freezes them on contracts and
// manages the global rules and per-seller exceptions.
type CommissionService struct {
	repo     repository.CommissionRepository
	userRepo repository.UserRepository
	auditSvc *AuditService
}

// NewCommissionService creates the commission service.
func NewCommissionService(repo repository.CommissionRepository, userRepo repository.UserRepository, auditSvc *AuditService) *CommissionService {
	return &CommissionService{repo: repo, userRepo: userRepo, auditSvc: auditSvc}
}

// CommissionRulesInput is the writable part of the rules form. Pointer fields
// let an update omit a rate without resetting it to zero.
type CommissionRulesInput struct {
	DefaultRate           *float64 `json:"default_rate"`
	UnassignedRate        *float64 `json:"unassigned_rate"`
	PaymentRule           string   `json:"payment_rule"`
	AcceptanceWindowHours *int     `json:"acceptance_window_hours"`
}

// Rules returns the global commission configuration.
func (s *CommissionService) Rules(ctx context.Context) (*models.CommissionRulesResponse, error) {
	settings, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	return rulesResponse(settings), nil
}

// UpdateRules validates and persists the two global rates and the acceptance
// window. The payment rule is part of the rules payload for UI compatibility;
// the payout trigger is fixed at contract approval.
func (s *CommissionService) UpdateRules(ctx context.Context, actorID uint, input CommissionRulesInput) (*models.CommissionRulesResponse, error) {
	settings, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	if input.DefaultRate != nil {
		if !validRate(*input.DefaultRate) {
			return nil, ErrInvalidCommissionRate
		}
		settings.CommissionDefaultRate = *input.DefaultRate
	}
	if input.UnassignedRate != nil {
		if !validRate(*input.UnassignedRate) {
			return nil, ErrInvalidCommissionRate
		}
		settings.CommissionUnassignedRate = *input.UnassignedRate
	}
	if input.PaymentRule != "" && input.PaymentRule != models.CommissionPaymentRule {
		return nil, fmt.Errorf("%w: regla de pago no soportada: %s", ErrInvalidCommissionRules, input.PaymentRule)
	}
	if input.AcceptanceWindowHours != nil {
		if *input.AcceptanceWindowHours <= 0 || *input.AcceptanceWindowHours > 168 {
			return nil, fmt.Errorf("%w: la ventana de aceptación debe estar entre 1 y 168 horas", ErrInvalidCommissionRules)
		}
		settings.CommissionAcceptanceWindowHours = *input.AcceptanceWindowHours
	}

	if err := s.repo.UpdateSettings(ctx, settings); err != nil {
		return nil, err
	}
	if s.auditSvc != nil {
		logIgnoredError("audit commission rules update", s.auditSvc.Log(ctx, actorID, "COMMISSION_RULES_UPDATE", "CommissionSettings", settings.ID,
			fmt.Sprintf("Reglas de comisión actualizadas: tasa por defecto %.2f%%, tasa sin vendedor %.2f%%",
				settings.CommissionDefaultRate, settings.CommissionUnassignedRate), "", ""))
	}
	return rulesResponse(settings), nil
}

// ListExceptions returns every per-seller rate exception with the seller name.
func (s *CommissionService) ListExceptions(ctx context.Context) ([]models.CommissionExceptionResponse, error) {
	rates, err := s.repo.ListSellerRates(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]models.CommissionExceptionResponse, 0, len(rates))
	for i := range rates {
		responses = append(responses, exceptionResponse(&rates[i]))
	}
	return responses, nil
}

// UpsertException creates or replaces the exception for one seller.
func (s *CommissionService) UpsertException(ctx context.Context, actorID, sellerID uint, rate float64) (*models.CommissionExceptionResponse, error) {
	if sellerID == 0 || !validRate(rate) {
		return nil, ErrInvalidCommissionRate
	}
	seller, err := s.userRepo.FindByID(ctx, sellerID)
	if err != nil || seller == nil || !seller.IsSeller() {
		return nil, ErrAssignmentSellerInactive
	}

	exception := &models.SellerCommissionRate{
		SellerID: sellerID,
		Rate:     rate,
	}
	if err := s.repo.UpsertSellerRate(ctx, exception); err != nil {
		return nil, err
	}
	// OnConflict does not return the existing row ID, so reload the exception
	// (also covers a concurrent upsert of the same seller).
	if reloaded, findErr := s.repo.FindSellerRate(ctx, sellerID); findErr == nil {
		exception = reloaded
	}
	exception.Seller = seller
	if s.auditSvc != nil {
		logIgnoredError("audit commission exception upsert", s.auditSvc.Log(ctx, actorID, "COMMISSION_EXCEPTION_UPSERT", "SellerCommissionRate", exception.ID,
			fmt.Sprintf("Excepción de comisión para %s: %.2f%%", seller.FullName, rate), "", ""))
	}
	response := exceptionResponse(exception)
	return &response, nil
}

// DeleteException removes a seller exception. The seller returns to the
// project/global rate.
func (s *CommissionService) DeleteException(ctx context.Context, actorID, id uint) error {
	exception, err := s.repo.FindSellerRateByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCommissionExceptionMissing
		}
		return err
	}
	if err := s.repo.DeleteSellerRate(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCommissionExceptionMissing
		}
		return err
	}
	if s.auditSvc != nil {
		name := fmt.Sprintf("vendedor #%d", exception.SellerID)
		if exception.Seller != nil && exception.Seller.FullName != "" {
			name = exception.Seller.FullName
		}
		logIgnoredError("audit commission exception delete", s.auditSvc.Log(ctx, actorID, "COMMISSION_EXCEPTION_DELETE", "SellerCommissionRate", id,
			fmt.Sprintf("Excepción de comisión eliminada para %s", name), "", ""))
	}
	return nil
}

// ResolveRateForContract applies seller exception -> project rate -> global
// default, or the unassigned rate when the reservation has no seller.
func (s *CommissionService) ResolveRateForContract(ctx context.Context, contract *models.Contract) (float64, error) {
	if contract == nil {
		return 0, ErrNotFound
	}
	settings, err := s.settings(ctx)
	if err != nil {
		return 0, err
	}
	if contract.SellerID == nil {
		return models.UnassignedCommissionRate(*settings), nil
	}
	exception, err := s.sellerException(ctx, *contract.SellerID)
	if err != nil {
		return 0, err
	}
	return models.ResolveCommissionRate(exception, &contract.Lot.Project, contract.FinancingType, *settings), nil
}

// ResolveHistoricalRate resolves the rate for a migrated sale, which has no
// project: seller exception -> global default.
func (s *CommissionService) ResolveHistoricalRate(ctx context.Context, sellerID uint) (float64, error) {
	settings, err := s.settings(ctx)
	if err != nil {
		return 0, err
	}
	if sellerID == 0 {
		return models.UnassignedCommissionRate(*settings), nil
	}
	exception, err := s.sellerException(ctx, sellerID)
	if err != nil {
		return 0, err
	}
	return models.ResolveCommissionRate(exception, nil, "", *settings), nil
}

// ApplyToContract freezes the resolved amount and the commission provenance on
// the contract. Reservations pass the approval time so the accrual date is the
// approval date; the amount is not recalculated for existing approvals unless
// the caller explicitly invokes this method.
func (s *CommissionService) ApplyToContract(ctx context.Context, contract *models.Contract, approvedAt time.Time) error {
	if contract == nil {
		return ErrNotFound
	}
	rate, err := s.ResolveRateForContract(ctx, contract)
	if err != nil {
		return err
	}
	contract.CommissionAmount = contract.CalculateCommissionWithRate(rate)
	contract.CommissionOrigin = models.CommissionOriginForAssignment(contract.AssignmentOrigin)
	if contract.CommissionOrigin == "" {
		contract.CommissionOrigin = models.CommissionOriginDirect
	}
	if !approvedAt.IsZero() {
		date := dateOnly(approvedAt)
		contract.CommissionAccrualDate = &date
	}
	return nil
}

// EnsureContractCommission fills a missing amount for legacy contracts without
// changing an already frozen approval.
func (s *CommissionService) EnsureContractCommission(ctx context.Context, contract *models.Contract) error {
	if contract == nil || contract.CommissionAmount > 0 {
		return nil
	}
	rate, err := s.ResolveRateForContract(ctx, contract)
	if err != nil {
		return err
	}
	contract.CommissionAmount = contract.CalculateCommissionWithRate(rate)
	if contract.CommissionOrigin == "" {
		contract.CommissionOrigin = models.CommissionOriginForAssignment(contract.AssignmentOrigin)
	}
	return nil
}

func (s *CommissionService) settings(ctx context.Context) (*models.CommissionSettings, error) {
	if s == nil || s.repo == nil {
		defaults := models.DefaultCommissionSettings()
		return &defaults, nil
	}
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			defaults := models.DefaultCommissionSettings()
			return &defaults, nil
		}
		return nil, err
	}
	if settings == nil {
		defaults := models.DefaultCommissionSettings()
		return &defaults, nil
	}
	return settings, nil
}

func (s *CommissionService) sellerException(ctx context.Context, sellerID uint) (*float64, error) {
	rate, err := s.repo.FindSellerRate(ctx, sellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if rate == nil {
		return nil, nil
	}
	value := rate.Rate
	return &value, nil
}

func rulesResponse(settings *models.CommissionSettings) *models.CommissionRulesResponse {
	if settings == nil {
		defaults := models.DefaultCommissionSettings()
		settings = &defaults
	}
	return &models.CommissionRulesResponse{
		DefaultRate:           settings.CommissionDefaultRate,
		UnassignedRate:        settings.CommissionUnassignedRate,
		PaymentRule:           models.CommissionPaymentRule,
		AcceptanceWindowHours: settings.CommissionAcceptanceWindowHours,
	}
}

// AcceptanceWindow returns the configured window a seller has to answer an
// assignment, falling back to the documented two hours.
func (s *CommissionService) AcceptanceWindow(ctx context.Context) time.Duration {
	settings, err := s.settings(ctx)
	if err != nil || settings == nil || settings.CommissionAcceptanceWindowHours <= 0 {
		return models.AssignmentAcceptWindow()
	}
	return time.Duration(settings.CommissionAcceptanceWindowHours) * time.Hour
}

// EffectiveRate returns the seller exception when present, otherwise the global
// default. It is used to explain the rate on the referral screen.
func (s *CommissionService) EffectiveRate(ctx context.Context, sellerID uint) (float64, error) {
	if sellerID > 0 {
		if rate, err := s.repo.FindSellerRate(ctx, sellerID); err == nil && rate != nil {
			return rate.Rate, nil
		}
	}
	settings, err := s.settings(ctx)
	if err != nil || settings == nil {
		return models.DefaultCommissionRate, nil
	}
	if settings.CommissionDefaultRate <= 0 {
		return models.DefaultCommissionRate, nil
	}
	return settings.CommissionDefaultRate, nil
}

func exceptionResponse(rate *models.SellerCommissionRate) models.CommissionExceptionResponse {
	name := ""
	if rate.Seller != nil {
		name = rate.Seller.FullName
	}
	return models.CommissionExceptionResponse{
		ID:         rate.ID,
		SellerID:   rate.SellerID,
		SellerName: name,
		Rate:       rate.Rate,
	}
}

func validRate(rate float64) bool {
	return rate >= 0 && rate <= 100
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
