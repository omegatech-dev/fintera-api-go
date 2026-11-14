package services

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// AssignmentService implements the reservation assignment lifecycle: initial
// offer, acceptance window, acceptance/rejection and background reassignment.
type AssignmentService struct {
	repo            repository.AssignmentRepository
	contractRepo    repository.ContractRepository
	userRepo        repository.UserRepository
	commissionRepo  repository.CommissionRepository
	notificationSvc *NotificationService
	auditSvc        *AuditService
	now             func() time.Time
	randIntn        func(int) int
}

// NewAssignmentService creates the assignment service.
func NewAssignmentService(
	repo repository.AssignmentRepository,
	contractRepo repository.ContractRepository,
	userRepo repository.UserRepository,
	notificationSvc *NotificationService,
	auditSvc *AuditService,
) *AssignmentService {
	return &AssignmentService{
		repo:            repo,
		contractRepo:    contractRepo,
		userRepo:        userRepo,
		notificationSvc: notificationSvc,
		auditSvc:        auditSvc,
		now:             time.Now,
		randIntn:        rand.Intn,
	}
}

// SetClock is used by tests to make deadlines deterministic.
func (s *AssignmentService) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// SetCommissionRepo wires the settings source for the configurable acceptance
// window. Keeping it as a setter preserves the constructor used by tests.
func (s *AssignmentService) SetCommissionRepo(repo repository.CommissionRepository) {
	s.commissionRepo = repo
}

// acceptanceWindow returns the configured answer window, falling back to the
// documented two hours.
func (s *AssignmentService) acceptanceWindow(ctx context.Context) time.Duration {
	if s.commissionRepo != nil {
		settings, err := s.commissionRepo.GetSettings(ctx)
		if err == nil && settings != nil && settings.CommissionAcceptanceWindowHours > 0 {
			return time.Duration(settings.CommissionAcceptanceWindowHours) * time.Hour
		}
	}
	return models.AssignmentAcceptWindow()
}

// ValidateSeller ensures an explicitly chosen seller is an active seller.
func (s *AssignmentService) ValidateSeller(ctx context.Context, sellerID uint) (*models.User, error) {
	if s.userRepo == nil {
		return nil, ErrAssignmentSellerInactive
	}
	seller, err := s.userRepo.FindByID(ctx, sellerID)
	if err != nil || seller == nil {
		return nil, ErrAssignmentSellerInactive
	}
	if !seller.IsSeller() || !seller.IsActive() {
		return nil, ErrAssignmentSellerInactive
	}
	return seller, nil
}

// ApplyOnCreate finishes the assignment configured on a freshly created
// contract. Referral and manual assignments already carry their seller; the
// automatic path runs the selection algorithm here.
func (s *AssignmentService) ApplyOnCreate(ctx context.Context, contract *models.Contract, now time.Time) error {
	if s == nil || s.repo == nil || contract == nil || contract.ID == 0 {
		return nil
	}

	switch contract.AssignmentStatus {
	case models.AssignmentStatusAccepted:
		if contract.SellerID == nil {
			return nil
		}
		responded := now
		if contract.RespondedAt != nil {
			responded = *contract.RespondedAt
		}
		return s.recordAccepted(ctx, contract, *contract.SellerID, contract.AssignmentOrigin, now, responded)
	case models.AssignmentStatusPending:
		if contract.SellerID != nil {
			deadline := contract.AcceptDeadlineAt
			if deadline == nil {
				value := now.Add(s.acceptanceWindow(ctx))
				deadline = &value
			}
			return s.recordPending(ctx, contract, *contract.SellerID, contract.AssignmentOrigin, now, deadline)
		}
	}

	origin := contract.AssignmentOrigin
	if origin == "" {
		origin = models.AssignmentOriginAutomatic
	}
	if origin != models.AssignmentOriginAutomatic {
		return nil
	}

	seller, err := s.SelectSeller(ctx, contract.ID, nil)
	if err != nil {
		return err
	}
	if seller == nil {
		return s.repo.UpdateContractAssignment(ctx, contract.ID, repository.ContractAssignmentUpdate{
			Origin:     models.AssignmentOriginAutomatic,
			Status:     models.AssignmentStatusNone,
			AssignedAt: contract.AssignedAt,
		})
	}
	return s.Offer(ctx, contract, seller, models.AssignmentOriginAutomatic, now, nil)
}

// SelectSeller applies the fairness rules and returns nil when the pool is
// exhausted: high before medium, lowest pending load, least recently assigned,
// then a random tiebreak.
func (s *AssignmentService) SelectSeller(ctx context.Context, contractID uint, extraExclude []uint) (*models.User, error) {
	expired, err := s.repo.ExpiredSellerIDsForContract(ctx, contractID)
	if err != nil {
		return nil, err
	}
	excluded := append([]uint{}, extraExclude...)
	excluded = append(excluded, expired...)

	sellers, err := s.repo.ListAssignableSellers(ctx, excluded)
	if err != nil {
		return nil, err
	}

	candidates := make([]models.User, 0, len(sellers))
	for _, seller := range sellers {
		if seller.IsAssignable() {
			candidates = append(candidates, seller)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	ids := make([]uint, 0, len(candidates))
	for _, seller := range candidates {
		ids = append(ids, seller.ID)
	}
	loads, err := s.repo.PendingLoadBySellerIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	lastAssigned, err := s.repo.LastAssignedAtBySellerIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	levelRank := func(level string) int {
		switch level {
		case models.ScoreLevelHigh:
			return 0
		case models.ScoreLevelMedium:
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		leftRank, rightRank := levelRank(left.ScoreLevel), levelRank(right.ScoreLevel)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftLoad, rightLoad := loads[left.ID], loads[right.ID]
		if leftLoad != rightLoad {
			return leftLoad < rightLoad
		}
		return lessAssigned(lastAssigned[left.ID], lastAssigned[right.ID])
	})

	// Same level, load and rotation timestamp: pick randomly. Once the seller
	// receives the offer their timestamp moves to the end of the queue.
	first := candidates[0]
	tieEnd := 1
	for tieEnd < len(candidates) && sameSelectionKey(first, candidates[tieEnd], loads, lastAssigned) {
		tieEnd++
	}
	if tieEnd > 1 && s.randIntn != nil {
		return &candidates[s.randIntn(tieEnd)], nil
	}
	return &candidates[0], nil
}

func lessAssigned(left, right *time.Time) bool {
	if left == nil {
		return right != nil
	}
	if right == nil {
		return false
	}
	return left.Before(*right)
}

func sameSelectionKey(left, right models.User, loads map[uint]int64, last map[uint]*time.Time) bool {
	if left.ScoreLevel != right.ScoreLevel {
		return false
	}
	if loads[left.ID] != loads[right.ID] {
		return false
	}
	leftLast, rightLast := last[left.ID], last[right.ID]
	if leftLast == nil || rightLast == nil {
		return leftLast == nil && rightLast == nil
	}
	return leftLast.Equal(*rightLast)
}

// Offer creates a pending assignment and mirrors it on the contract.
func (s *AssignmentService) Offer(ctx context.Context, contract *models.Contract, seller *models.User, origin string, now time.Time, deadline *time.Time) error {
	if deadline == nil {
		value := now.Add(s.acceptanceWindow(ctx))
		deadline = &value
	}
	return s.recordPending(ctx, contract, seller.ID, origin, now, deadline)
}

func (s *AssignmentService) recordPending(ctx context.Context, contract *models.Contract, sellerID uint, origin string, now time.Time, deadline *time.Time) error {
	if origin == "" {
		origin = models.AssignmentOriginAutomatic
	}
	assignment := &models.Assignment{
		ContractID:       contract.ID,
		SellerID:         sellerID,
		Origin:           origin,
		Status:           models.AssignmentStatusPending,
		AssignedAt:       now.UTC(),
		AcceptDeadlineAt: deadline,
	}
	if err := s.repo.Create(ctx, assignment); err != nil {
		return err
	}
	if err := s.repo.UpdateContractAssignment(ctx, contract.ID, repository.ContractAssignmentUpdate{
		SellerID:         &sellerID,
		Origin:           origin,
		Status:           models.AssignmentStatusPending,
		AssignedAt:       &assignment.AssignedAt,
		AcceptDeadlineAt: deadline,
	}); err != nil {
		return err
	}

	contract.SellerID = &sellerID
	contract.AssignmentOrigin = origin
	contract.AssignmentStatus = models.AssignmentStatusPending
	contract.AssignedAt = &assignment.AssignedAt
	contract.AcceptDeadlineAt = deadline
	contract.RespondedAt = nil

	s.audit(ctx, sellerID, "ASSIGN", fmt.Sprintf("Asignación creada para el contrato %d", assignment.ID), assignment.ID)
	s.notifyAssignmentCreated(ctx, contract, assignment)
	return nil
}

func (s *AssignmentService) recordAccepted(ctx context.Context, contract *models.Contract, sellerID uint, origin string, assignedAt, respondedAt time.Time) error {
	if origin == "" {
		origin = models.AssignmentOriginReferral
	}
	assignment := &models.Assignment{
		ContractID:  contract.ID,
		SellerID:    sellerID,
		Origin:      origin,
		Status:      models.AssignmentStatusAccepted,
		AssignedAt:  assignedAt.UTC(),
		RespondedAt: &respondedAt,
	}
	if err := s.repo.Create(ctx, assignment); err != nil {
		return err
	}
	contract.SellerID = &sellerID
	contract.AssignmentOrigin = origin
	contract.AssignmentStatus = models.AssignmentStatusAccepted
	contract.AssignedAt = &assignment.AssignedAt
	contract.RespondedAt = &respondedAt
	contract.AcceptDeadlineAt = nil

	s.audit(ctx, sellerID, "ASSIGN", fmt.Sprintf("Asignación aceptada automáticamente para el contrato %d", assignment.ID), assignment.ID)
	s.notifyAssignmentAccepted(ctx, contract, assignment)
	return nil
}

// PendingForSeller lists the active offers of one seller.
func (s *AssignmentService) PendingForSeller(ctx context.Context, sellerID uint) ([]models.Assignment, error) {
	return s.repo.FindPendingBySeller(ctx, sellerID)
}

// PendingAll lists every active offer; used by admins.
func (s *AssignmentService) PendingAll(ctx context.Context) ([]models.Assignment, error) {
	return s.repo.FindPendingAll(ctx)
}

// Accept commits a pending assignment to its seller. Repeating the same call
// is a no-op; acting on an expired offer is rejected.
func (s *AssignmentService) Accept(ctx context.Context, assignmentID, actorID uint, isAdmin bool) (*models.Assignment, error) {
	assignment, err := s.repo.FindByIDWithDetails(ctx, assignmentID)
	if err != nil {
		return nil, ErrAssignmentNotFound
	}
	if !isAdmin && assignment.SellerID != actorID {
		return nil, ErrAssignmentNotOwned
	}
	if assignment.Contract.ID != 0 &&
		assignment.Contract.Status != models.ContractStatusPending &&
		assignment.Contract.Status != models.ContractStatusSubmitted {
		return assignment, ErrAssignmentNotPending
	}

	now := s.now()
	switch assignment.Status {
	case models.AssignmentStatusAccepted:
		return assignment, nil
	case models.AssignmentStatusExpired:
		return assignment, ErrAssignmentExpired
	}
	if assignment.IsOverdue(now) {
		if err := s.expire(ctx, assignment, now); err != nil {
			return assignment, err
		}
		return assignment, ErrAssignmentExpired
	}

	updated, err := s.repo.UpdateStatusIfPending(ctx, assignment.ID, models.AssignmentStatusAccepted, now)
	if err != nil {
		return nil, err
	}
	if !updated {
		current, reloadErr := s.repo.FindByIDWithDetails(ctx, assignment.ID)
		if reloadErr != nil {
			return nil, ErrAssignmentNotPending
		}
		if current.Status == models.AssignmentStatusAccepted {
			return current, nil
		}
		return current, ErrAssignmentNotPending
	}

	if err := s.repo.UpdateContractAssignment(ctx, assignment.ContractID, repository.ContractAssignmentUpdate{
		SellerID:         &assignment.SellerID,
		Origin:           assignment.Origin,
		Status:           models.AssignmentStatusAccepted,
		AssignedAt:       &assignment.AssignedAt,
		AcceptDeadlineAt: nil,
		RespondedAt:      &now,
	}); err != nil {
		return nil, err
	}

	assignment.Status = models.AssignmentStatusAccepted
	assignment.RespondedAt = &now
	assignment.AcceptDeadlineAt = nil
	s.audit(ctx, actorID, "ACCEPT", fmt.Sprintf("Asignación aceptada para el contrato %d", assignment.ID), assignment.ID)
	s.notifyAdmins(ctx, "Asignación aceptada", fmt.Sprintf(
		"%s aceptó la reserva del proyecto %s, lote %s.",
		s.sellerName(assignment), assignment.Contract.Lot.Project.Name, assignment.Contract.Lot.Name,
	), models.NotificationTypeAssignmentAccepted)
	return assignment, nil
}

// Reject expires a pending offer and immediately reassigns it. Repeating the
// rejection is a no-op.
func (s *AssignmentService) Reject(ctx context.Context, assignmentID, actorID uint, isAdmin bool) (*models.Assignment, error) {
	assignment, err := s.repo.FindByIDWithDetails(ctx, assignmentID)
	if err != nil {
		return nil, ErrAssignmentNotFound
	}
	if !isAdmin && assignment.SellerID != actorID {
		return nil, ErrAssignmentNotOwned
	}
	if assignment.Status == models.AssignmentStatusExpired {
		return assignment, nil
	}
	if assignment.Status != models.AssignmentStatusPending {
		return assignment, ErrAssignmentNotPending
	}

	now := s.now()
	updated, err := s.repo.UpdateStatusIfPending(ctx, assignment.ID, models.AssignmentStatusExpired, now)
	if err != nil {
		return nil, err
	}
	if !updated {
		current, reloadErr := s.repo.FindByIDWithDetails(ctx, assignment.ID)
		if reloadErr != nil {
			return nil, ErrAssignmentNotPending
		}
		if current.Status == models.AssignmentStatusExpired {
			return current, nil
		}
		return current, ErrAssignmentNotPending
	}

	assignment.Status = models.AssignmentStatusExpired
	assignment.RespondedAt = &now
	s.audit(ctx, actorID, "REJECT", fmt.Sprintf("Asignación rechazada para el contrato %d", assignment.ID), assignment.ID)
	s.notifyAssignmentExpired(ctx, assignment)
	if err := s.reassign(ctx, assignment, now); err != nil {
		return assignment, err
	}
	return assignment, nil
}

// ExpireOverdue is the recurring job entry point. It expires every overdue
// pending offer and reassigns the reservation while the pool has candidates.
func (s *AssignmentService) ExpireOverdue(ctx context.Context) error {
	now := s.now()
	overdue, err := s.repo.FindOverduePending(ctx, now)
	if err != nil {
		return err
	}
	var firstErr error
	for index := range overdue {
		assignment := &overdue[index]
		if err := s.expire(ctx, assignment, now); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *AssignmentService) expire(ctx context.Context, assignment *models.Assignment, now time.Time) error {
	updated, err := s.repo.UpdateStatusIfPending(ctx, assignment.ID, models.AssignmentStatusExpired, now)
	if err != nil || !updated {
		return err
	}
	assignment.Status = models.AssignmentStatusExpired
	assignment.RespondedAt = &now
	s.audit(ctx, assignment.SellerID, "EXPIRE", fmt.Sprintf("Asignación expirada para el contrato %d", assignment.ID), assignment.ID)
	s.notifyAssignmentExpired(ctx, assignment)
	return s.reassign(ctx, assignment, now)
}

// reassign clears the current seller and offers the reservation to the next
// eligible seller. When nobody is eligible the reservation stays without a
// seller and is never cancelled.
func (s *AssignmentService) reassign(ctx context.Context, assignment *models.Assignment, now time.Time) error {
	contract := assignment.Contract
	if contract.ID == 0 {
		if s.contractRepo == nil {
			return ErrAssignmentNotFound
		}
		loaded, err := s.contractRepo.FindByIDWithDetails(ctx, assignment.ContractID)
		if err != nil {
			return err
		}
		contract = *loaded
	}

	if contract.Status != models.ContractStatusPending && contract.Status != models.ContractStatusSubmitted {
		return s.repo.UpdateContractAssignment(ctx, contract.ID, repository.ContractAssignmentUpdate{
			Origin:           originOrAutomatic(contract.AssignmentOrigin),
			Status:           models.AssignmentStatusNone,
			AssignedAt:       contract.AssignedAt,
			AcceptDeadlineAt: nil,
			RespondedAt:      &now,
		})
	}

	if err := s.repo.UpdateContractAssignment(ctx, contract.ID, repository.ContractAssignmentUpdate{
		Origin:           originOrAutomatic(contract.AssignmentOrigin),
		Status:           models.AssignmentStatusNone,
		AssignedAt:       contract.AssignedAt,
		AcceptDeadlineAt: nil,
		RespondedAt:      &now,
	}); err != nil {
		return err
	}

	seller, err := s.SelectSeller(ctx, contract.ID, nil)
	if err != nil {
		return err
	}
	if seller == nil {
		return nil
	}
	return s.Offer(ctx, &contract, seller, models.AssignmentOriginAutomatic, now, nil)
}

func originOrAutomatic(origin string) string {
	if origin == "" || origin == models.AssignmentOriginNone {
		return models.AssignmentOriginAutomatic
	}
	return origin
}

func (s *AssignmentService) sellerName(assignment *models.Assignment) string {
	if assignment.Seller != nil && assignment.Seller.FullName != "" {
		return assignment.Seller.FullName
	}
	return "El vendedor"
}

func (s *AssignmentService) audit(ctx context.Context, actorID uint, action, details string, assignmentID uint) {
	if s.auditSvc == nil {
		return
	}
	logIgnoredError("write assignment audit log", s.auditSvc.Log(ctx, actorID, action, "Assignment", assignmentID, details, "", ""))
}

func (s *AssignmentService) notifyAssignmentCreated(ctx context.Context, contract *models.Contract, assignment *models.Assignment) {
	if s.notificationSvc == nil {
		return
	}
	message := fmt.Sprintf(
		"Se te asignó una reserva en %s, lote %s. Tienes hasta %s para aceptarla.",
		contract.Lot.Project.Name, contract.Lot.Name,
		formatDeadline(assignment.AcceptDeadlineAt),
	)
	logIgnoredError("notify assignment created", s.notificationSvc.NotifyUser(
		ctx, assignment.SellerID, "Nueva reserva asignada", message, models.NotificationTypeAssignmentCreated))
}

func (s *AssignmentService) notifyAssignmentAccepted(ctx context.Context, contract *models.Contract, assignment *models.Assignment) {
	if s.notificationSvc == nil {
		return
	}
	message := fmt.Sprintf(
		"Se te asignó una reserva por referido en %s, lote %s.",
		contract.Lot.Project.Name, contract.Lot.Name,
	)
	logIgnoredError("notify assignment accepted", s.notificationSvc.NotifyUser(
		ctx, assignment.SellerID, "Reserva asignada", message, models.NotificationTypeAssignmentAccepted))
}

func (s *AssignmentService) notifyAssignmentExpired(ctx context.Context, assignment *models.Assignment) {
	if s.notificationSvc == nil {
		return
	}
	message := fmt.Sprintf(
		"La asignación de la reserva del proyecto %s, lote %s expiró.",
		assignment.Contract.Lot.Project.Name, assignment.Contract.Lot.Name,
	)
	logIgnoredError("notify assignment expired", s.notificationSvc.NotifyUser(
		ctx, assignment.SellerID, "Asignación expirada", message, models.NotificationTypeAssignmentExpired))
}

func (s *AssignmentService) notifyAdmins(ctx context.Context, title, message, notifType string) {
	if s.notificationSvc == nil {
		return
	}
	logIgnoredError("notify admins", s.notificationSvc.NotifyAdmins(ctx, title, message, notifType))
}

func formatDeadline(deadline *time.Time) string {
	if deadline == nil {
		return "sin fecha límite"
	}
	return deadline.Format("02/01/2006 15:04")
}
