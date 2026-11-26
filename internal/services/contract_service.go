package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/statemachine"
	"gorm.io/gorm"
)

type ContractService struct {
	repo            repository.ContractRepository
	lotRepo         repository.LotRepository
	userRepo        repository.UserRepository
	paymentRepo     repository.PaymentRepository
	ledgerRepo      repository.LedgerRepository
	notificationSvc *NotificationService
	emailSvc        *EmailService
	auditSvc        *AuditService
	worker          *jobs.Worker
	paymentSchedule *PaymentScheduleService
	referralSvc     *ReferralService
	assignmentSvc   *AssignmentService
	commissionSvc   *CommissionService
	identityDocSvc  *IdentityDocumentService
}

func NewContractService(
	repo repository.ContractRepository,
	lotRepo repository.LotRepository,
	userRepo repository.UserRepository,
	paymentRepo repository.PaymentRepository,
	ledgerRepo repository.LedgerRepository,
	notificationSvc *NotificationService,
	emailSvc *EmailService,
	auditSvc *AuditService,
	worker *jobs.Worker,
) *ContractService {
	return &ContractService{
		repo:            repo,
		lotRepo:         lotRepo,
		userRepo:        userRepo,
		paymentRepo:     paymentRepo,
		ledgerRepo:      ledgerRepo,
		notificationSvc: notificationSvc,
		emailSvc:        emailSvc,
		auditSvc:        auditSvc,
		worker:          worker,
		paymentSchedule: NewPaymentScheduleService(),
	}
}

// SetAssignmentServices wires the referral and assignment services after
// construction. Keeping it as a setter preserves the existing constructor
// used by focused unit tests.
func (s *ContractService) SetAssignmentServices(referral *ReferralService, assignment *AssignmentService) {
	s.referralSvc = referral
	s.assignmentSvc = assignment
}

// SetCommissionService wires the commission-rate resolver. Keeping it as a
// setter preserves the constructor used by focused unit tests.
func (s *ContractService) SetCommissionService(commission *CommissionService) {
	s.commissionSvc = commission
}

// SetIdentityDocumentService wires the identity document service. Keeping it
// as a setter preserves the constructor used by focused unit tests.
func (s *ContractService) SetIdentityDocumentService(identity *IdentityDocumentService) {
	s.identityDocSvc = identity
}

// ContractAssignmentInput carries the assignment-related reservation fields
// parsed from the create request.
type ContractAssignmentInput struct {
	Origin            string
	SellerID          *uint
	ReferralGUID      string
	AttributionCookie string
	// AutoAccept assigns the requested seller immediately. It is used when a
	// seller creates the reservation in the back office, so they do not have to
	// accept their own assignment, and attribution cannot reassign it.
	AutoAccept bool
}

// CreateWithAssignment resolves the referral attribution and the requested
// seller, persists the contract and then materializes the assignment.
func (s *ContractService) CreateWithAssignment(ctx context.Context, contract *models.Contract, input ContractAssignmentInput, links ...models.ContractIdentityDocument) error {
	now := time.Now()
	origin := normalizeAssignmentOrigin(input.Origin)

	var attributedSeller *models.User
	var referralGUID string
	if !input.AutoAccept && s.referralSvc != nil {
		attributedSeller, referralGUID, _ = s.referralSvc.ResolveAttribution(ctx, input.ReferralGUID, input.AttributionCookie, now)
	}

	switch {
	case attributedSeller != nil:
		contract.SellerID = &attributedSeller.ID
		contract.ReferralGUID = &referralGUID
		contract.AssignmentOrigin = models.AssignmentOriginReferral
		contract.AssignmentStatus = models.AssignmentStatusAccepted
		contract.AssignedAt = &now
		contract.RespondedAt = &now
	case origin == models.AssignmentOriginReferral:
		if input.SellerID != nil {
			seller, err := s.ValidateAssignmentSeller(ctx, *input.SellerID)
			if err != nil {
				return err
			}
			contract.SellerID = &seller.ID
			contract.AssignmentOrigin = models.AssignmentOriginReferral
			contract.AssignmentStatus = models.AssignmentStatusAccepted
			contract.AssignedAt = &now
			contract.RespondedAt = &now
		} else {
			contract.AssignmentOrigin = models.AssignmentOriginAutomatic
			contract.AssignmentStatus = models.AssignmentStatusNone
		}
	case (origin == models.AssignmentOriginManual || origin == "") && input.SellerID != nil:
		seller, err := s.ValidateAssignmentSeller(ctx, *input.SellerID)
		if err != nil {
			return err
		}
		contract.SellerID = &seller.ID
		contract.AssignmentOrigin = models.AssignmentOriginManual
		contract.AssignedAt = &now
		if input.AutoAccept {
			contract.AssignmentStatus = models.AssignmentStatusAccepted
			contract.RespondedAt = &now
			contract.AcceptDeadlineAt = nil
		} else {
			deadline := s.assignmentDeadline(ctx, now)
			contract.AssignmentStatus = models.AssignmentStatusPending
			contract.AcceptDeadlineAt = &deadline
		}
	default:
		contract.AssignmentOrigin = models.AssignmentOriginAutomatic
		contract.AssignmentStatus = models.AssignmentStatusNone
	}

	if err := s.Create(ctx, contract, links...); err != nil {
		return err
	}
	if s.assignmentSvc != nil {
		if err := s.assignmentSvc.ApplyOnCreate(ctx, contract, now); err != nil {
			return err
		}
		// The automatic path picks the seller after the initial commission
		// preview. Recalculate and persist so the stored amount uses the
		// assigned seller's rate instead of the unassigned fallback.
		if contract.SellerID != nil && s.commissionSvc != nil {
			if err := s.commissionSvc.ApplyToContract(ctx, contract, time.Time{}); err != nil {
				logIgnoredError("recalculate commission after assignment", err)
			} else if err := s.repo.Update(ctx, contract); err != nil {
				logIgnoredError("persist commission after assignment", err)
			}
		}
	}
	return nil
}

// assignmentDeadline returns the acceptance deadline using the admin-configured
// window, falling back to the documented default.
func (s *ContractService) assignmentDeadline(ctx context.Context, now time.Time) time.Time {
	if s.commissionSvc != nil {
		return now.Add(s.commissionSvc.AcceptanceWindow(ctx))
	}
	return models.AssignmentDeadlineFrom(now)
}

// ValidateAssignmentSeller checks that a manually chosen seller is active.
func (s *ContractService) ValidateAssignmentSeller(ctx context.Context, sellerID uint) (*models.User, error) {
	if s.assignmentSvc == nil {
		return nil, ErrAssignmentSellerInactive
	}
	return s.assignmentSvc.ValidateSeller(ctx, sellerID)
}

// Reassign modes accepted by the admin reassignment flow.
const (
	ReassignModeSeller    = "seller"
	ReassignModeAutomatic = "automatic"
)

// ContractReassignInput carries the admin reassignment request. When Accept is
// true the seller is assigned immediately; otherwise the seller receives a
// normal offer with the configured acceptance window. Approved, closed and
// cancelled contracts are always assigned immediately so the commission stays
// stable.
type ContractReassignInput struct {
	Mode           string
	SellerID       *uint
	Accept         bool
	ExcludeCurrent bool
	ActorID        uint
}

// Reassign changes the seller of a contract in any lifecycle status. A paid
// commission is recovered from the previous seller as a pending credit before
// the new seller's commission is calculated.
func (s *ContractService) Reassign(ctx context.Context, guid string, input ContractReassignInput) (*models.Contract, *models.Assignment, error) {
	if s.assignmentSvc == nil {
		return nil, nil, ErrAssignmentSellerInactive
	}
	contract, err := s.repo.FindByGUIDWithDetails(ctx, guid)
	if err != nil {
		return nil, nil, ErrNotFound
	}

	now := time.Now()
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	origin := models.AssignmentOriginManual
	var seller *models.User

	switch mode {
	case "", ReassignModeAutomatic:
		origin = models.AssignmentOriginAutomatic
		var exclude []uint
		if input.ExcludeCurrent && contract.SellerID != nil {
			exclude = append(exclude, *contract.SellerID)
		}
		seller, _, err = s.assignmentSvc.chooseAutomaticSeller(ctx, contract.ID, exclude)
		if err != nil {
			return nil, nil, err
		}
		if seller == nil && len(exclude) > 0 {
			seller, _, err = s.assignmentSvc.chooseAutomaticSeller(ctx, contract.ID, nil)
			if err != nil {
				return nil, nil, err
			}
		}
		if seller == nil {
			return nil, nil, ErrNoAssignableSeller
		}
	case ReassignModeSeller:
		if input.SellerID == nil || *input.SellerID == 0 {
			return nil, nil, ErrAssignmentSellerInactive
		}
		seller, err = s.assignmentSvc.ValidateSeller(ctx, *input.SellerID)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, ErrAssignmentSellerInactive
	}

	previousSellerID := contract.SellerID
	previousOrigin := contract.AssignmentOrigin
	previousStatus := contract.AssignmentStatus
	previousAssignedAt := contract.AssignedAt
	previousDeadline := contract.AcceptDeadlineAt
	previousRespondedAt := contract.RespondedAt
	previousPaidAt := contract.CommissionPaidAt
	previousPaidBy := contract.CommissionPaidBy

	paidAmount := contract.CommissionAmount
	if paidAmount <= 0 {
		paidAmount = contract.CalculateCommission()
	}
	sameSeller := previousSellerID != nil && *previousSellerID == seller.ID
	// Cancelled and rejected contracts already settled their commission (a
	// cancellation may have created a recoverable credit). Reassigning them
	// must not duplicate the credit or resurrect a commission.
	skipCommission := contract.Status == models.ContractStatusCancelled ||
		contract.Status == models.ContractStatusRejected

	contract.SellerID = &seller.ID
	contract.AssignmentOrigin = origin
	if s.commissionSvc != nil && !skipCommission && (contract.CommissionPaidAt == nil || !sameSeller) {
		if err := s.commissionSvc.ApplyToContract(ctx, contract, time.Time{}); err != nil {
			contract.SellerID = previousSellerID
			contract.AssignmentOrigin = previousOrigin
			return nil, nil, err
		}
	}

	creditNote := ""
	if !skipCommission && contract.CommissionPaidAt != nil && previousSellerID != nil && !sameSeller && s.commissionSvc != nil {
		credit, creditErr := s.commissionSvc.CreateCommissionCredit(ctx, contract, *previousSellerID, paidAmount)
		if creditErr != nil {
			contract.SellerID = previousSellerID
			contract.AssignmentOrigin = previousOrigin
			return nil, nil, creditErr
		}
		if credit != nil {
			creditNote = fmt.Sprintf(". Crédito de comisión al vendedor anterior: L%.2f", credit.Amount)
			contract.CommissionPaidAt = nil
			contract.CommissionPaidBy = nil
		}
	}

	if err := s.assignmentSvc.SupersedePending(ctx, contract.ID, input.ActorID, now); err != nil {
		contract.SellerID = previousSellerID
		contract.AssignmentOrigin = previousOrigin
		contract.AssignmentStatus = previousStatus
		contract.AssignedAt = previousAssignedAt
		contract.AcceptDeadlineAt = previousDeadline
		contract.RespondedAt = previousRespondedAt
		contract.CommissionPaidAt = previousPaidAt
		contract.CommissionPaidBy = previousPaidBy
		return nil, nil, err
	}

	accept := input.Accept || (contract.Status != models.ContractStatusPending && contract.Status != models.ContractStatusSubmitted)
	if accept {
		if _, err := s.assignmentSvc.AssignNow(ctx, contract, seller.ID, origin, now); err != nil {
			contract.CommissionPaidAt = previousPaidAt
			contract.CommissionPaidBy = previousPaidBy
			return nil, nil, err
		}
	} else if err := s.assignmentSvc.Offer(ctx, contract, seller, origin, now, nil); err != nil {
		contract.CommissionPaidAt = previousPaidAt
		contract.CommissionPaidBy = previousPaidBy
		return nil, nil, err
	}

	contract.Seller = seller

	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, nil, err
	}

	if s.auditSvc != nil {
		previous := "sin vendedor"
		if previousSellerID != nil {
			previous = fmt.Sprintf("vendedor #%d", *previousSellerID)
		}
		detail := fmt.Sprintf("Reasignación manual: %s → %s (%s)%s", previous, seller.FullName, origin, creditNote)
		logIgnoredError("write reassignment audit log", s.auditSvc.Log(ctx, input.ActorID, "REASSIGN", "Contract", contract.ID, detail, "", ""))
	}

	history, err := s.assignmentSvc.History(ctx, contract.ID)
	if err != nil || len(history) == 0 {
		return contract, nil, err
	}
	assignment := history[0]
	return contract, &assignment, nil
}

// AssignmentHistory returns every seller offer attempted for a contract.
func (s *ContractService) AssignmentHistory(ctx context.Context, guid string) ([]models.Assignment, error) {
	contract, err := s.repo.FindByGUIDWithDetails(ctx, guid)
	if err != nil {
		return nil, ErrNotFound
	}
	if s.assignmentSvc == nil {
		return []models.Assignment{}, nil
	}
	return s.assignmentSvc.History(ctx, contract.ID)
}

func normalizeAssignmentOrigin(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.AssignmentOriginReferral:
		return models.AssignmentOriginReferral
	case models.AssignmentOriginManual:
		return models.AssignmentOriginManual
	case models.AssignmentOriginAutomatic:
		return models.AssignmentOriginAutomatic
	default:
		return ""
	}
}

// ClearReferralAttribution expires the attribution cookie after a reservation
// consumes it. It is a no-op when referrals are not wired.
func (s *ContractService) ClearReferralAttribution(w http.ResponseWriter) {
	if s.referralSvc == nil || w == nil {
		return
	}
	http.SetCookie(w, s.referralSvc.ClearedAttributionCookie())
}

// FindByID gets a contract by ID
func (s *ContractService) FindByID(ctx context.Context, id uint) (*models.Contract, error) {
	return s.repo.FindByID(ctx, id)
}

// GetLedgerEntries returns ledger entries for a contract (lightweight; used by Ledger endpoint).
func (s *ContractService) GetLedgerEntries(ctx context.Context, contractID uint) ([]models.ContractLedgerEntry, error) {
	return s.ledgerRepo.FindByContractID(ctx, contractID)
}

// FindByUser returns the contracts where the user is the applicant.
func (s *ContractService) FindByUser(ctx context.Context, userID uint) ([]models.Contract, error) {
	return s.repo.FindByUser(ctx, userID)
}

// FindByIDWithDetails gets a contract by ID with all nested associations preloaded
func (s *ContractService) FindByIDWithDetails(ctx context.Context, id uint) (*models.Contract, error) {
	return s.repo.FindByIDWithDetails(ctx, id)
}

// FindByGUIDWithDetails gets a contract by its public GUID.
func (s *ContractService) FindByGUIDWithDetails(ctx context.Context, guid string) (*models.Contract, error) {
	return s.repo.FindByGUIDWithDetails(ctx, guid)
}

// FindLotByProjectAndGUID resolves a public lot inside a published project.
func (s *ContractService) FindLotByProjectAndGUID(ctx context.Context, projectGUID, lotGUID string) (*models.Lot, error) {
	lot, err := s.lotRepo.FindByGUID(ctx, lotGUID)
	if err != nil {
		return nil, err
	}
	if lot.Project.GUID != projectGUID || !lot.Project.IsPublished || !lot.Project.IsActive {
		return nil, fmt.Errorf("lote no encontrado")
	}
	return lot, nil
}

func (s *ContractService) List(ctx context.Context, query *repository.ContractQuery) ([]models.Contract, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *ContractService) GetStats(ctx context.Context) (*repository.ContractStats, error) {
	return s.repo.GetStats(ctx)
}

func (s *ContractService) Create(ctx context.Context, contract *models.Contract, links ...models.ContractIdentityDocument) error {
	// Verify lot is available
	lot, err := s.lotRepo.FindByID(ctx, contract.LotID)
	if err != nil {
		return err
	}

	// Auto-generate GUID if not provided
	if contract.GUID == "" {
		contract.GUID = uuid.New().String()
	}

	if !lot.IsAvailable() {
		return errors.New("el lote no está disponible")
	}

	// Create contract
	// Calculate balance: Amount - Reserve - DownPayment
	// Assuming contract.Amount is already set or derived from lot price
	// If not set, use Lot EffectivePrice
	if contract.Amount == nil || *contract.Amount == 0 {
		price := lot.EffectivePrice()
		contract.Amount = &price
	}

	// Freeze the commission preview using the seller exception -> project
	// rate -> global default resolution. The definitive amount is recomputed
	// when the contract is approved.
	if contract.Lot.ID == 0 {
		contract.Lot = *lot
	}
	if s.commissionSvc != nil {
		if err := s.commissionSvc.ApplyToContract(ctx, contract, time.Time{}); err != nil {
			logIgnoredError("resolve commission on create", err)
			contract.CommissionAmount = contract.CalculateCommission()
		}
	} else if contract.CommissionAmount == 0 {
		contract.CommissionAmount = contract.CalculateCommission()
	}

	// Initial balance is the full amount (as debt)
	// It will be reduced as payments (including reserve and down payment) are approved
	balance := -(*contract.Amount)
	contract.Balance = &balance

	// Atomically claim the lot. This is the real guard against double-booking:
	// the earlier IsAvailable check is only a fast path.
	reserved, err := s.lotRepo.ReserveIfAvailable(ctx, lot.ID)
	if err != nil {
		return err
	}
	if !reserved {
		return errors.New("el lote no está disponible")
	}

	if len(links) > 0 {
		err = s.repo.CreateWithIdentityDocuments(ctx, contract, links)
	} else {
		err = s.repo.Create(ctx, contract)
	}
	if err != nil {
		lot.Status = models.LotStatusAvailable
		logIgnoredError("release lot after create failure", s.lotRepo.Update(ctx, lot))
		return err
	}

	// Create the reservation payment immediately so the portal can upload the
	// voucher while the contract is pending. It is due at the 48h hold expiry.
	if contract.ReserveAmount != nil && *contract.ReserveAmount > 0 {
		due := models.HoldExpiresAt(contract.CreatedAt)
		if contract.CreatedAt.IsZero() {
			due = models.HoldExpiresAt(time.Now())
		}
		description := "Pago de Reserva"
		reservation := &models.Payment{
			ContractID:  contract.ID,
			Amount:      *contract.ReserveAmount,
			DueDate:     due,
			Status:      models.PaymentStatusPending,
			PaymentType: models.PaymentTypeReservation,
			Description: &description,
		}
		if err := s.paymentRepo.Create(ctx, reservation); err != nil {
			return fmt.Errorf("failed to create reservation payment: %w", err)
		}
	}

	// Retrieve applicant for notification
	applicant, err := s.userRepo.FindByID(ctx, contract.ApplicantUserID)
	if err != nil {
		return fmt.Errorf("failed to retrieve applicant: %w", err)
	}
	contract.ApplicantUser = *applicant

	// Notify admins asynchronously
	notificationMsg := fmt.Sprintf("Se ha recibido una nueva solicitud de contrato\nProyecto: %s\nLote: %s\nSolicitante: %s",
		lot.Project.Name, lot.Name, applicant.FullName)

	s.worker.EnqueueAsync(func(ctx context.Context) error {
		return s.notificationSvc.NotifyAdmins(ctx,
			"Nueva solicitud de contrato",
			notificationMsg,
			models.NotificationTypeContractApproved)
	})

	// Notify applicant via email
	contractForEmail := contract
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		// Log error inside service if it fails
		if err := s.emailSvc.SendContractSubmitted(ctx, contractForEmail); err != nil {
			return err
		}
		return nil
	})

	// Audit log
	logIgnoredError("write contract create audit log", s.auditSvc.Log(ctx, contract.ApplicantUserID, "CREATE", "Contract", contract.ID,
		fmt.Sprintf("Solicitud de contrato creada para el lote %s del proyecto %s. Precio: %.2f", lot.Name, lot.Project.Name, *contract.Amount), "", ""))

	return nil
}

// ContractIdentityResult reports what happened with the identity documents of
// a contract creation.
type ContractIdentityResult struct {
	// Filled are the kinds that were stored by this request.
	Filled []string
	// AlreadyPresent are the kinds the applicant already had on file. The
	// uploaded file of those kinds was not written.
	AlreadyPresent []string
}

// CheckIdentityRequirement returns an error when the applicant is missing a
// document and the requirement is enforced. The caller uses it to answer
// before it writes anything.
func (s *ContractService) CheckIdentityRequirement(ctx context.Context, applicantUserID uint, uploads []IdentityDocumentUpload) error {
	if s.identityDocSvc == nil {
		return nil
	}
	return s.identityDocSvc.CheckRequirement(ctx, applicantUserID, uploads)
}

// CreateWithIdentityDocuments persists a contract together with the identity
// documents of its applicant.
//
// The requirement is checked before anything is written, so a rejected request
// leaves no half-uploaded document behind. The typed uploads only fill the
// empty slots: a document that is already on file is kept and reported as
// already present, because replacing it is a deliberate act of the staff.
func (s *ContractService) CreateWithIdentityDocuments(ctx context.Context, contract *models.Contract, input ContractAssignmentInput, uploads []IdentityDocumentUpload, attachmentPaths []string) (ContractIdentityResult, error) {
	var result ContractIdentityResult

	if s.identityDocSvc == nil {
		if err := s.CreateWithAssignment(ctx, contract, input); err != nil {
			return result, err
		}
		return result, nil
	}

	applicantID := contract.ApplicantUserID

	if err := s.identityDocSvc.CheckRequirement(ctx, applicantID, uploads); err != nil {
		return result, err
	}

	actorID := uint(0)
	if contract.CreatorID != nil {
		actorID = *contract.CreatorID
	}

	filled, alreadyPresent, err := s.identityDocSvc.FillMissing(ctx, applicantID, uploads, actorID)
	if err != nil {
		return result, err
	}
	result.Filled = filled
	result.AlreadyPresent = alreadyPresent

	links, err := s.identityLinks(ctx, applicantID)
	if err != nil {
		return result, err
	}

	if err := s.CreateWithAssignment(ctx, contract, input, links...); err != nil {
		return result, err
	}

	// The free-form attachments of the request become read-only entries of the
	// contract, so the staff sees what the customer attached without waiting
	// for the one-off migration task. A failure is not fatal: the paths stay in
	// document_paths and the migration task picks them up later.
	logIgnoredError("attach contract legacy files", s.identityDocSvc.AttachLegacyFiles(ctx, contract.ID, attachmentPaths))

	return result, nil
}

// identityLinks builds the links of a contract for the documents the
// applicant has on file after the fill.
func (s *ContractService) identityLinks(ctx context.Context, applicantID uint) ([]models.ContractIdentityDocument, error) {
	docs, err := s.identityDocSvc.DocumentsByUser(ctx, applicantID)
	if err != nil {
		return nil, err
	}
	links := make([]models.ContractIdentityDocument, 0, len(docs))
	for i := range docs {
		doc := docs[i]
		kind := doc.Kind
		docID := doc.ID
		links = append(links, models.ContractIdentityDocument{
			IdentityDocumentID: &docID,
			Kind:               &kind,
			Source:             models.IdentityDocumentSourceUpload,
		})
	}
	return links, nil
}

func (s *ContractService) Update(ctx context.Context, contract *models.Contract) error {
	return s.repo.Update(ctx, contract)
}

func (s *ContractService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.userRepo.FindByEmail(ctx, email)
}

func (s *ContractService) GetUserByID(ctx context.Context, id uint) (*models.User, error) {
	return s.userRepo.FindByID(ctx, id)
}

func (s *ContractService) GetUserByIdentity(ctx context.Context, identity string) (*models.User, error) {
	return s.userRepo.FindByIdentity(ctx, identity)
}

func (s *ContractService) CreateUser(ctx context.Context, user *models.User, password string) error {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	user.EncryptedPassword = hashedPassword
	user.MustChangePassword = true
	if err := s.userRepo.Create(ctx, user); err != nil {
		return err
	}
	// Send welcome email with temp password (best-effort)
	if err := s.emailSvc.SendAccountCreated(ctx, user, password); err != nil {
		_ = err // Logged inside SendAccountCreated
	}
	return nil
}

// ErrAccountAlreadyConfigured is returned when a customer already replaced the
// temporary password, so resending the setup email would reset it.
var ErrAccountAlreadyConfigured = errors.New("la cuenta ya está configurada")

// ResendAccountSetup regenerates the temporary password and resends the account
// setup email for a customer who has not completed onboarding yet.
func (s *ContractService) ResendAccountSetup(ctx context.Context, contractGUID string) error {
	contract, err := s.repo.FindByGUIDWithDetails(ctx, contractGUID)
	if err != nil {
		return ErrNotFound
	}
	user := contract.ApplicantUser
	if !user.MustChangePassword {
		return ErrAccountAlreadyConfigured
	}

	tempPassword, err := GenerateTempPassword()
	if err != nil {
		return err
	}
	hashed, err := HashPassword(tempPassword)
	if err != nil {
		return err
	}
	// Send first so a disabled/unconfigured mailer cannot silently reset the
	// stored password without the customer receiving the new one.
	if err := s.emailSvc.SendAccountCreated(ctx, &user, tempPassword); err != nil {
		return err
	}
	user.EncryptedPassword = hashed
	user.MustChangePassword = true
	if err := s.userRepo.Update(ctx, &user); err != nil {
		return err
	}
	return nil
}

// Approve approves a contract
func (s *ContractService) Approve(ctx context.Context, id uint) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use FSM to validate and transition state
	fsm := statemachine.NewContractFSM(contract)
	if err := fsm.Approve(ctx); err != nil {
		return nil, fmt.Errorf("cannot approve contract: %w", err)
	}

	// Set approval timestamp
	now := time.Now()
	contract.ApprovedAt = &now
	contract.Active = true

	// Freeze the commission at approval: seller exception -> project rate ->
	// global default (or the unassigned rate). The accrual date is the approval
	// date for reservations.
	if s.commissionSvc != nil {
		if err := s.commissionSvc.ApplyToContract(ctx, contract, now); err != nil {
			return nil, fmt.Errorf("failed to resolve commission: %w", err)
		}
	} else if contract.CommissionAmount == 0 {
		contract.CommissionAmount = contract.CalculateCommission()
		contract.CommissionOrigin = models.CommissionOriginForAssignment(contract.AssignmentOrigin)
		date := dateOnly(now)
		contract.CommissionAccrualDate = &date
	}

	// Update contract
	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, err
	}

	// Create initial ledger entry (contract amount as debit)
	if contract.Amount != nil {
		initialEntry := &models.ContractLedgerEntry{
			ContractID:  contract.ID,
			Amount:      -(*contract.Amount), // Negative representing debt
			Description: "Monto Inicial del Contrato",
			EntryType:   models.EntryTypeInitial,
			EntryDate:   now,
		}
		if err := s.ledgerRepo.Create(ctx, initialEntry); err != nil {
			return nil, fmt.Errorf("failed to create initial ledger entry: %w", err)
		}
	}

	// Generate payment schedule
	payments, err := s.paymentSchedule.GenerateSchedule(ctx, contract)
	if err != nil {
		return nil, fmt.Errorf("failed to generate payment schedule: %w", err)
	}

	// Keep the reservation payment created at contract creation instead of
	// duplicating it when the schedule is generated.
	existingPayments, _ := s.paymentRepo.FindByContract(ctx, contract.ID)
	hasReservation := false
	for i := range existingPayments {
		if existingPayments[i].PaymentType == models.PaymentTypeReservation {
			hasReservation = true
			break
		}
	}

	// Create payments
	for _, payment := range payments {
		if payment.PaymentType == models.PaymentTypeReservation && hasReservation {
			continue
		}
		if err := s.paymentRepo.Create(ctx, &payment); err != nil {
			return nil, fmt.Errorf("failed to create payment: %w", err)
		}
	}

	// Update lot status to reserved (it moves to financed when first payment is made)
	lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
	if lot != nil {
		lot.Status = models.LotStatusReserved
		logIgnoredError("update lot status on approval", s.lotRepo.Update(ctx, lot))
	}

	// Find the "main" payment for the email highlight (Installment or Full Balance)
	highlightAmount := 0.0
	firstPaymentDate := ""

	// Determine relevant payment generic logic
	var targetPayment *models.Payment
	for i := range payments {
		p := &payments[i]
		if contract.FinancingType == models.FinancingTypeDirect {
			if p.PaymentType == models.PaymentTypeInstallment {
				targetPayment = p
				break
			}
		} else {
			// Bank or Cash -> Look for the full balance payment
			if p.PaymentType == models.PaymentTypeFull {
				targetPayment = p
				break
			}
		}
	}

	// Fallback to first payment if specific type not found (e.g. only reserve exists?)
	if targetPayment == nil && len(payments) > 0 {
		targetPayment = &payments[0]
	}

	if targetPayment != nil {
		highlightAmount = targetPayment.Amount
		firstPaymentDate = targetPayment.DueDate.Format("02/01/2006")
	}

	contractForEmail := contract
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		if err := s.notificationSvc.NotifyUser(ctx, contractForEmail.ApplicantUserID,
			"Contrato aprobado",
			"Tu solicitud de contrato ha sido aprobada",
			models.NotificationTypeContractApproved); err != nil {
			logIgnoredError("notify approved contract", err)
		}
		return s.emailSvc.SendContractApproved(ctx, contractForEmail, highlightAmount, firstPaymentDate)
	})

	// Audit log
	// Assuming creator/approver ID is available in context or contract.CreatorID.
	// For now, using ApplicantUserID as a fallback if CreatorID is nil, but ideally it should be the admin performing the action.
	// Since Approve takes context, we might extract user from context if available in middleware, but for now we log the action on the contract.
	userID := contract.ApplicantUserID
	if contract.CreatorID != nil {
		userID = *contract.CreatorID
	}
	contractAmount := 0.0
	if contract.Amount != nil {
		contractAmount = *contract.Amount
	}
	logIgnoredError("write contract approve audit log", s.auditSvc.Log(ctx, userID, "APPROVE", "Contract", contract.ID,
		fmt.Sprintf("Contrato aprobado. Lote: %s, Precio: %.2f", contract.Lot.Name, contractAmount), "", ""))

	return contract, nil
}

func (s *ContractService) Reject(ctx context.Context, id uint, reason string) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use FSM to validate and transition state
	fsm := statemachine.NewContractFSM(contract)
	if err := fsm.Reject(ctx); err != nil {
		return nil, fmt.Errorf("cannot reject contract: %w", err)
	}

	contract.RejectionReason = &reason
	now := time.Now()
	contract.RejectedAt = &now

	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, err
	}

	// Release lot
	lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
	if lot != nil {
		lot.Status = models.LotStatusAvailable
		logIgnoredError("release lot on rejection", s.lotRepo.Update(ctx, lot))
	}

	// Notify contract owner (in-app + email) with rejection reason
	notifyTitle := "Contrato rechazado"
	notifyMsg := "Tu solicitud de contrato ha sido rechazada."
	if reason != "" {
		notifyMsg = "Tu solicitud de contrato ha sido rechazada. Razón: " + reason
	}
	contractCopy := contract // capture for async
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		if err := s.notificationSvc.NotifyUser(ctx, contractCopy.ApplicantUserID, notifyTitle, notifyMsg, models.NotificationTypeContractRejected); err != nil {
			return err
		}
		_ = s.emailSvc.SendContractRejected(ctx, contractCopy, reason) // best-effort; owner already got in-app message
		return nil
	})

	// Audit log
	logIgnoredError("write contract reject audit log", s.auditSvc.Log(ctx, contract.ApplicantUserID, "REJECT", "Contract", contract.ID,
		fmt.Sprintf("Contrato rechazado. Razón: %s", reason), "", ""))

	return contract, nil
}

// DeleteRejected deletes a rejected contract and releases the lot so it can be reserved again. Only allowed when status is rejected.
func (s *ContractService) DeleteRejected(ctx context.Context, id uint) error {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return err
	}
	if contract.Status != models.ContractStatusRejected {
		return fmt.Errorf("solo se pueden eliminar contratos en estado rechazado")
	}

	// Delete all payments for this contract (rejected contracts typically have none)
	_ = s.paymentRepo.DeleteByContract(ctx, contract.ID)
	// Delete ledger entries (rejected contracts typically have none)
	_ = s.ledgerRepo.DeleteByContractID(ctx, contract.ID)
	// Delete the contract
	if err := s.repo.Delete(ctx, contract.ID); err != nil {
		return fmt.Errorf("failed to delete contract: %w", err)
	}

	// Release lot so it can be reserved again
	lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
	if lot != nil {
		lot.Status = models.LotStatusAvailable
		logIgnoredError("release lot after deleting rejected contract", s.lotRepo.Update(ctx, lot))
	}

	logIgnoredError("write contract delete audit log", s.auditSvc.Log(ctx, contract.ApplicantUserID, "DELETE", "Contract", contract.ID,
		fmt.Sprintf("Contrato rechazado eliminado. Lote %d liberado para nueva reserva", contract.LotID), "", ""))
	return nil
}

// SystemActorID identifies cancellations triggered by background jobs rather
// than a signed-in admin.
const SystemActorID uint = 0

// Cancel cancels a contract, preserves its financial history and settles the
// ledger to zero. Pending/submitted payments are marked cancelled so collection
// jobs stop picking them up; paid payments stay untouched and are refunded in
// the ledger.
func (s *ContractService) Cancel(ctx context.Context, id uint, note string, actorID uint) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use FSM to validate and transition state
	fsm := statemachine.NewContractFSM(contract)
	if err := fsm.Cancel(ctx); err != nil {
		return nil, fmt.Errorf("cannot cancel contract: %w", err)
	}

	contract.Note = &note
	contract.Active = false
	// The cancellation settles the ledger, so the cached balance is zero.
	zero := 0.0
	contract.Balance = &zero
	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, err
	}

	// Keep the schedule rows for the history, but move the unpaid ones to
	// cancelled. Paid payments must survive so the refund can be calculated.
	if err := s.paymentRepo.UpdateStatusByContract(ctx, contract.ID,
		[]string{models.PaymentStatusPending, models.PaymentStatusSubmitted},
		models.PaymentStatusCancelled); err != nil {
		return nil, fmt.Errorf("failed to cancel pending payments: %w", err)
	}
	// Mirror the batch update in the loaded contract so the response already
	// reflects the cancelled schedule.
	for i := range contract.Payments {
		payment := &contract.Payments[i]
		if payment.Status == models.PaymentStatusPending || payment.Status == models.PaymentStatusSubmitted {
			payment.Status = models.PaymentStatusCancelled
		}
	}

	paidTotal := 0.0
	for i := range contract.Payments {
		payment := &contract.Payments[i]
		if payment.Status == models.PaymentStatusPaid && payment.PaidAmount != nil {
			paidTotal += *payment.PaidAmount
		}
	}

	now := time.Now()
	if paidTotal > 0 {
		refund := &models.ContractLedgerEntry{
			ContractID:  contract.ID,
			Amount:      -paidTotal,
			Description: fmt.Sprintf("Reembolso por cancelación: L%.2f", paidTotal),
			EntryType:   models.EntryTypeRefund,
			EntryDate:   now,
		}
		if err := s.ledgerRepo.Create(ctx, refund); err != nil {
			return nil, fmt.Errorf("failed to create refund ledger entry: %w", err)
		}
	}

	// Write off the contract amount so the balance ends at zero: the initial
	// entry is -amount and the refund reversed every payment credit.
	if contract.Amount != nil && *contract.Amount > 0 {
		writeOff := &models.ContractLedgerEntry{
			ContractID:  contract.ID,
			Amount:      *contract.Amount,
			Description: fmt.Sprintf("Castigo por cancelación: L%.2f", *contract.Amount),
			EntryType:   models.EntryTypeAdjustment,
			EntryDate:   now,
		}
		if err := s.ledgerRepo.Create(ctx, writeOff); err != nil {
			return nil, fmt.Errorf("failed to create write-off ledger entry: %w", err)
		}
	}

	// Release the lot only when no other active contract is using it.
	other, err := s.repo.FindActiveByLot(ctx, contract.LotID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logIgnoredError("find active contract on cancellation", err)
	} else if other == nil || other.ID == contract.ID {
		lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
		if lot != nil {
			lot.Status = models.LotStatusAvailable
			logIgnoredError("release lot on cancellation", s.lotRepo.Update(ctx, lot))
		}
	}

	// Commission credit: a commission already paid to the seller becomes a
	// recoverable pending credit. The hook is optional so focused tests can
	// build the service without a commission dependency.
	var commissionCredit *models.SellerCommissionCredit
	if s.commissionSvc != nil && contract.CommissionPaidAt != nil {
		commissionCredit, err = s.commissionSvc.CreateCancellationCredit(ctx, contract)
		if err != nil {
			return nil, fmt.Errorf("failed to create commission credit: %w", err)
		}
	}

	// Background jobs have no human actor; attribute their audit entry to the
	// contract owner so the audit_logs foreign key stays valid.
	auditActorID := actorID
	if auditActorID == SystemActorID {
		auditActorID = contract.ApplicantUserID
	}
	creditNote := ""
	if commissionCredit != nil {
		creditNote = fmt.Sprintf(". Crédito de comisión: L%.2f", commissionCredit.Amount)
	}
	logIgnoredError("write contract cancel audit log", s.auditSvc.Log(ctx, auditActorID, "CANCEL", "Contract", contract.ID,
		fmt.Sprintf("Contrato cancelado. Nota: %s. Reembolso: L%.2f%s", note, paidTotal, creditNote), "", ""))

	return contract, nil
}

// Close closes a contract when balance is paid off
func (s *ContractService) Close(ctx context.Context, id uint) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	// Calculate current balance
	balance, err := s.ledgerRepo.CalculateBalance(ctx, contract.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate balance: %w", err)
	}

	if balance < 0 {
		return nil, fmt.Errorf("cannot close contract with outstanding balance: %.2f", balance)
	}

	// Use FSM to validate and transition
	fsm := statemachine.NewContractFSM(contract)
	if err := fsm.Close(ctx); err != nil {
		return nil, fmt.Errorf("cannot close contract: %w", err)
	}

	// Set closed timestamp
	now := time.Now()
	contract.ClosedAt = &now
	contract.Active = false

	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, err
	}

	// Update lot status to fully paid
	lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
	if lot != nil {
		lot.Status = models.LotStatusFullyPaid
		logIgnoredError("mark lot fully paid", s.lotRepo.Update(ctx, lot))
	}

	// Notify user
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
			"Contrato completado",
			"¡Felicidades! Tu contrato ha sido completado",
			models.NotificationTypeContractApproved)
	})

	// Audit log
	logIgnoredError("write contract close audit log", s.auditSvc.Log(ctx, contract.ApplicantUserID, "CLOSE", "Contract", contract.ID,
		"Contrato completado y cerrado exitosamente", "", ""))

	return contract, nil
}

// MarkCommissionPaid records that the seller commission for an approved or
// closed contract has been paid out. The amount itself stays frozen.
func (s *ContractService) MarkCommissionPaid(ctx context.Context, id uint, actorID uint) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	if contract.Status != models.ContractStatusApproved && contract.Status != models.ContractStatusClosed {
		return nil, fmt.Errorf("solo se puede pagar la comisión de contratos aprobados o cerrados")
	}
	// Older contracts may not have the commission persisted; freeze the
	// resolved amount at payout time so the report and the payout agree.
	commission := contract.CommissionAmount
	if commission <= 0 {
		if s.commissionSvc != nil {
			if err := s.commissionSvc.EnsureContractCommission(ctx, contract); err != nil {
				return nil, err
			}
			commission = contract.CommissionAmount
		}
	}
	if commission <= 0 {
		commission = contract.CalculateCommission()
	}
	if commission <= 0 {
		return nil, fmt.Errorf("el contrato no tiene comisión calculada")
	}
	if contract.CommissionPaidAt != nil {
		return nil, fmt.Errorf("la comisión ya fue marcada como pagada")
	}

	if err := s.repo.MarkCommissionPaid(ctx, contract.ID, actorID, commission); err != nil {
		return nil, err
	}

	now := time.Now()
	contract.CommissionAmount = commission
	contract.CommissionPaidAt = &now
	contract.CommissionPaidBy = &actorID

	logIgnoredError("write commission paid audit log", s.auditSvc.Log(ctx, actorID, "COMMISSION_PAID", "Contract", contract.ID,
		fmt.Sprintf("Comisión pagada: %.2f", contract.CommissionAmount), "", ""))

	return contract, nil
}

// CreatePrepaymentRequest records a customer request to pay part (or all) of
// the outstanding balance ahead of schedule. The payment stays submitted until
// staff validates the uploaded receipt; approving it applies the amount as a
// capital repayment and shortens the schedule.
func (s *ContractService) CreatePrepaymentRequest(ctx context.Context, contractGUID string, amount float64) (*models.Payment, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("el monto debe ser mayor a cero")
	}

	contract, err := s.repo.FindByGUIDWithDetails(ctx, contractGUID)
	if err != nil {
		return nil, fmt.Errorf("contrato no encontrado")
	}
	if contract.Status != models.ContractStatusApproved {
		return nil, fmt.Errorf("solo puedes solicitar un pago anticipado de un contrato aprobado")
	}

	balance, err := s.ledgerRepo.CalculateBalance(ctx, contract.ID)
	if err != nil {
		return nil, fmt.Errorf("no se pudo calcular el saldo del contrato")
	}
	outstanding := -balance
	if outstanding <= 0.005 {
		return nil, fmt.Errorf("el contrato no tiene saldo pendiente")
	}
	if amount > outstanding+0.005 {
		return nil, fmt.Errorf("el monto no puede superar el saldo pendiente de L%.2f", outstanding)
	}

	payments, err := s.paymentRepo.FindByContract(ctx, contract.ID)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron revisar los pagos del contrato")
	}
	for _, payment := range payments {
		if payment.PaymentType == models.PaymentTypeCapitalRepayment && payment.Status == models.PaymentStatusSubmitted {
			return nil, fmt.Errorf("ya tienes una solicitud de pago anticipado en revisión")
		}
	}

	now := time.Now()
	description := fmt.Sprintf("Pago anticipado: L%.2f", amount)
	prepayment := &models.Payment{
		ContractID:  contract.ID,
		Amount:      amount,
		DueDate:     now,
		Status:      models.PaymentStatusSubmitted,
		PaymentType: models.PaymentTypeCapitalRepayment,
		Description: &description,
	}
	if err := s.paymentRepo.Create(ctx, prepayment); err != nil {
		return nil, fmt.Errorf("no se pudo registrar la solicitud de pago anticipado")
	}

	return prepayment, nil
}

// CapitalRepayment applies a capital repayment to the contract, reducing the
// pending obligations from the end of the schedule and settling the terminal
// balance when the repayment covers it.
func (s *ContractService) CapitalRepayment(ctx context.Context, id uint, amount float64, actorID uint, ip, userAgent string) error {
	if amount <= 0 {
		return fmt.Errorf("repayment amount must be greater than 0")
	}

	// 1. Fetch contract with details (including payments)
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load contract: %w", err)
	}

	// Check contract is approved
	if contract.Status != models.ContractStatusApproved {
		return fmt.Errorf("can only apply capital repayment to approved contracts")
	}

	// The ledger is authoritative: never credit more than the outstanding debt
	// or the schedule and the balance would disagree.
	balance, err := s.ledgerRepo.CalculateBalance(ctx, contract.ID)
	if err != nil {
		return fmt.Errorf("failed to calculate balance: %w", err)
	}
	outstanding := -balance
	if outstanding <= capitalRepaymentEpsilon {
		return fmt.Errorf("el contrato no tiene saldo pendiente")
	}
	if amount > outstanding+capitalRepaymentEpsilon {
		return fmt.Errorf("el abono no puede superar el saldo pendiente de L%.2f", outstanding)
	}

	// 2. Create Payment record for the repayment (to show in schedule/history)
	now := time.Now()
	repaymentDescription := fmt.Sprintf("Abono a Capital: L%.2f", amount)
	repaymentPayment := &models.Payment{
		ContractID:  contract.ID,
		Amount:      amount,
		PaidAmount:  &amount,
		PaymentDate: &now,
		Status:      models.PaymentStatusPaid,
		PaymentType: models.PaymentTypeCapitalRepayment,
		Description: &repaymentDescription,
		ApprovedAt:  &now,
		DueDate:     now, // Using now as due date for recording purposes
	}
	if err := s.paymentRepo.Create(ctx, repaymentPayment); err != nil {
		return fmt.Errorf("failed to create repayment payment record: %w", err)
	}

	// 3. Create prepayment ledger entry (credit) and link to payment
	prepaymentEntry := &models.ContractLedgerEntry{
		ContractID:  contract.ID,
		PaymentID:   &repaymentPayment.ID,
		Amount:      amount, // Positive (payment/credit)
		Description: repaymentDescription,
		EntryType:   models.EntryTypePrepayment,
		EntryDate:   now,
	}
	if err := s.ledgerRepo.Create(ctx, prepaymentEntry); err != nil {
		return fmt.Errorf("failed to create ledger entry: %w", err)
	}

	// 4. Distribute the repayment over the pending obligations. Future
	// installments are removed or reduced, while the full balance (cash/bank
	// contracts) is marked as paid once covered.
	if _, err := applyCapitalToSchedule(ctx, s.paymentRepo, contract.ID, repaymentPayment.ID, amount); err != nil {
		return err
	}

	// 5. Update contract balance and lot status if needed
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		// Calculate current balance
		balance, err := s.ledgerRepo.CalculateBalance(ctx, contract.ID)
		if err == nil {
			contract.Balance = &balance

			// Clear associations to prevent GORM from re-saving/resurrecting deleted payments
			// The contract object still has the stale 'Payments' slice from the original fetch,
			// which includes the payments we just deleted. Saving the contract with these
			// associations would re-insert them.
			contract.Payments = nil
			contract.LedgerEntries = nil

			logIgnoredError("update contract balance after capital repayment", s.repo.Update(ctx, contract))

			// Auto-close if balance >= 0
			if balance >= 0 {
				_, closeErr := s.Close(ctx, contract.ID)
				logIgnoredError("auto-close contract after capital repayment", closeErr)
			}
		}
		return nil
	})

	// 6. Audit log
	logIgnoredError("write capital repayment audit log", s.auditSvc.Log(ctx, actorID, "CAPITAL_REPAYMENT", "Contract", contract.ID,
		fmt.Sprintf("Abono a capital de L%.2f aplicado al contrato #%d", amount, contract.ID), ip, userAgent))

	return nil
}

// SendReminder sends a manual payment/reservation reminder for one contract.
func (s *ContractService) SendReminder(ctx context.Context, guid string, actorID uint) error {
	contract, err := s.repo.FindByGUIDWithDetails(ctx, guid)
	if err != nil {
		return ErrNotFound
	}
	if err := s.emailSvc.SendReservationVoucherReminder(ctx, contract); err != nil {
		return err
	}
	logIgnoredError("audit contract reminder", s.auditSvc.Log(ctx, actorID, "REMINDER", "Contract", contract.ID,
		"Recordatorio de pago enviado al cliente", "", ""))
	return nil
}

// SendReservationVoucherReminders emails customers whose reservation is about
// to expire without an uploaded voucher. It only contacts each reservation once.
func (s *ContractService) SendReservationVoucherReminders(ctx context.Context) error {
	contracts, err := s.repo.FindReservationsAwaitingVoucher(ctx, 40)
	if err != nil {
		return err
	}

	for i := range contracts {
		contract := &contracts[i]
		if err := s.emailSvc.SendReservationVoucherReminder(ctx, contract); err != nil {
			logIgnoredError("send reservation voucher reminder", err)
			continue
		}
		if err := s.repo.MarkReservationReminderSent(ctx, contract.ID); err != nil {
			logIgnoredError("mark reservation reminder sent", err)
		}
	}

	return nil
}

func (s *ContractService) ReleaseUnpaidReservations(ctx context.Context) error {
	// Find reservations older than 48 hours without payment
	contracts, err := s.repo.FindPendingReservations(ctx, 48)
	if err != nil {
		return err
	}

	for _, contract := range contracts {
		_, cancelErr := s.Cancel(ctx, contract.ID, "Reservación no pagada a tiempo", SystemActorID)
		logIgnoredError("cancel unpaid reservation", cancelErr)
	}

	return nil
}
