package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Repositories

type mockContractRepo struct {
	mock.Mock
}

func (m *mockContractRepo) FindByID(ctx context.Context, id uint) (*models.Contract, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindByIDWithDetails(ctx context.Context, id uint) (*models.Contract, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindByGUIDWithDetails(ctx context.Context, guid string) (*models.Contract, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindByLot(ctx context.Context, lotID uint) ([]models.Contract, error) {
	args := m.Called(ctx, lotID)
	return args.Get(0).([]models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindByUser(ctx context.Context, userID uint) ([]models.Contract, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Contract), args.Error(1)
}

func (m *mockContractRepo) Create(ctx context.Context, contract *models.Contract) error {
	args := m.Called(ctx, contract)
	return args.Error(0)
}

func (m *mockContractRepo) Update(ctx context.Context, contract *models.Contract) error {
	args := m.Called(ctx, contract)
	return args.Error(0)
}

func (m *mockContractRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockContractRepo) List(ctx context.Context, query *repository.ContractQuery) ([]models.Contract, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]models.Contract), args.Get(1).(int64), args.Error(2)
}

func (m *mockContractRepo) FindActiveByLot(ctx context.Context, lotID uint) (*models.Contract, error) {
	args := m.Called(ctx, lotID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindPendingReservations(ctx context.Context, olderThan int) ([]models.Contract, error) {
	args := m.Called(ctx, olderThan)
	return args.Get(0).([]models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindReservationsAwaitingVoucher(ctx context.Context, olderThanHours int) ([]models.Contract, error) {
	args := m.Called(ctx, olderThanHours)
	return args.Get(0).([]models.Contract), args.Error(1)
}

func (m *mockContractRepo) MarkReservationReminderSent(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockContractRepo) MarkCommissionPaid(ctx context.Context, id uint, paidBy uint, amount float64) error {
	args := m.Called(ctx, id, paidBy, amount)
	return args.Error(0)
}

func (m *mockContractRepo) GetStats(ctx context.Context) (*repository.ContractStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ContractStats), args.Error(1)
}

func (m *mockContractRepo) HasActiveContracts(ctx context.Context, userID uint) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

type mockUserRepoForContract struct {
	mock.Mock
}

func (m *mockUserRepoForContract) FindByID(ctx context.Context, id uint) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepoForContract) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepoForContract) FindByIdentity(ctx context.Context, identity string) (*models.User, error) {
	args := m.Called(ctx, identity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepoForContract) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockUserRepoForContract) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockUserRepoForContract) SetRecoveryCode(ctx context.Context, userID uint, code string, sentAt time.Time) error {
	args := m.Called(ctx, userID, code, sentAt)
	return args.Error(0)
}

func (m *mockUserRepoForContract) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepoForContract) SoftDelete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepoForContract) Restore(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepoForContract) List(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]models.User), args.Get(1).(int64), args.Error(2)
}

func (m *mockUserRepoForContract) FindAdmins(ctx context.Context) ([]models.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.User), args.Error(1)
}

func (m *mockUserRepoForContract) CountByRole(ctx context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func (m *mockUserRepoForContract) CountActiveClientsByCreator(ctx context.Context, creatorIDs []uint) (map[uint]int64, error) {
	return map[uint]int64{}, nil
}

func (m *mockUserRepoForContract) FindAll(ctx context.Context) ([]models.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.User), args.Error(1)
}

type mockPaymentRepo struct {
	mock.Mock
}

func (m *mockPaymentRepo) FindByID(ctx context.Context, id uint) (*models.Payment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) FindByContract(ctx context.Context, contractID uint) ([]models.Payment, error) {
	args := m.Called(ctx, contractID)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) Create(ctx context.Context, payment *models.Payment) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

func (m *mockPaymentRepo) Update(ctx context.Context, payment *models.Payment) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

func (m *mockPaymentRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockPaymentRepo) DeleteByContract(ctx context.Context, contractID uint) error {
	args := m.Called(ctx, contractID)
	return args.Error(0)
}

func (m *mockPaymentRepo) List(ctx context.Context, query *repository.ListQuery) ([]models.Payment, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]models.Payment), args.Get(1).(int64), args.Error(2)
}

func (m *mockPaymentRepo) FindOverdue(ctx context.Context) ([]models.Payment, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) FindOverdueForActiveContracts(ctx context.Context) ([]models.Payment, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) FindPaymentsDueTomorrowForActiveContracts(ctx context.Context) ([]models.Payment, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) MarkOverdueReminderSent(ctx context.Context, paymentIDs []uint) error {
	args := m.Called(ctx, paymentIDs)
	return args.Error(0)
}

func (m *mockPaymentRepo) MarkUpcomingReminderSent(ctx context.Context, paymentIDs []uint) error {
	args := m.Called(ctx, paymentIDs)
	return args.Error(0)
}

func (m *mockPaymentRepo) FindPendingByUser(ctx context.Context, userID uint) ([]models.Payment, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) FindPaidByMonth(ctx context.Context, month, year int) ([]models.Payment, error) {
	args := m.Called(ctx, month, year)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetMonthlyStats(ctx context.Context) (*repository.PaymentStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaymentStats), args.Error(1)
}

func (m *mockPaymentRepo) FindByUserID(ctx context.Context, userID uint) ([]models.Payment, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) BatchUpdateInterest(ctx context.Context, updates map[uint]float64) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

type mockLedgerRepo struct {
	mock.Mock
}

func (m *mockLedgerRepo) Create(ctx context.Context, entry *models.ContractLedgerEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *mockLedgerRepo) FindByContractID(ctx context.Context, contractID uint) ([]models.ContractLedgerEntry, error) {
	args := m.Called(ctx, contractID)
	return args.Get(0).([]models.ContractLedgerEntry), args.Error(1)
}

func (m *mockLedgerRepo) FindByPaymentID(ctx context.Context, paymentID uint) ([]models.ContractLedgerEntry, error) {
	args := m.Called(ctx, paymentID)
	return args.Get(0).([]models.ContractLedgerEntry), args.Error(1)
}

func (m *mockLedgerRepo) CalculateBalance(ctx context.Context, contractID uint) (float64, error) {
	args := m.Called(ctx, contractID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockLedgerRepo) FindOrCreateByPaymentAndType(ctx context.Context, entry *models.ContractLedgerEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *mockLedgerRepo) BatchUpsertInterest(ctx context.Context, entries []models.ContractLedgerEntry) error {
	args := m.Called(ctx, entries)
	return args.Error(0)
}

func (m *mockLedgerRepo) DeleteByContractID(ctx context.Context, contractID uint) error {
	args := m.Called(ctx, contractID)
	return args.Error(0)
}

func TestContractService_Create(t *testing.T) {
	// Initialize logger to avoid nil pointer dereference
	logger.Setup("test")

	ctx := context.Background()
	mockContractRepo := new(mockContractRepo)
	mockLotRepo := new(mockLotRepo)
	mockUserRepo := new(mockUserRepoForContract)
	mockPaymentRepo := new(mockPaymentRepo)
	mockLedgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)

	// Create notification and email services with their respective stubs
	// For simplicity, we can pass nil for services that aren't strictly accessed if we mock well,
	// but emailSvc and notificationSvc methods are called in background jobs.
	// Since we are mocking worker.EnqueueAsync or letting the background job run and fail,
	// we should probably just test the synchronous part and ignore background job errors
	// or provide proper stubs. Let's pass nil and intercept worker if needed.

	notificationSvc := NewNotificationService(nil, nil)

	// EmailSvc requires auth config
	cfg := &config.Config{ResendAPIKey: "mock-key"}
	emailSvc := NewEmailService(cfg)

	service := NewContractService(mockContractRepo, mockLotRepo, mockUserRepo, mockPaymentRepo, mockLedgerRepo, notificationSvc, emailSvc, auditSvc, worker)

	lot := &models.Lot{
		ID:     1,
		Status: models.LotStatusAvailable,
		Price:  10000,
		Project: models.Project{
			Name: "Test Project",
		},
	}

	contract := &models.Contract{
		LotID:           1,
		ApplicantUserID: 2,
	}

	applicant := &models.User{
		ID:       2,
		FullName: "Test User",
	}

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	mockLotRepo.On("ReserveIfAvailable", ctx, uint(1)).Return(true, nil)

	mockContractRepo.On("Create", ctx, mock.AnythingOfType("*models.Contract")).Return(nil)
	mockUserRepo.On("FindByID", ctx, uint(2)).Return(applicant, nil)

	err := service.Create(ctx, contract)

	assert.NoError(t, err)
	assert.NotEmpty(t, contract.GUID)
	assert.Equal(t, 10000.0, *contract.Amount)
	assert.Equal(t, -10000.0, *contract.Balance)

	mockLotRepo.AssertExpectations(t)
	mockContractRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
}

func TestContractService_Reject(t *testing.T) {
	// Initialize logger to avoid nil pointer dereference
	logger.Setup("test")

	ctx := context.Background()
	mockContractRepo := new(mockContractRepo)
	mockLotRepo := new(mockLotRepo)
	mockUserRepo := new(mockUserRepoForContract)
	mockPaymentRepo := new(mockPaymentRepo)
	mockLedgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)

	cfg := &config.Config{ResendAPIKey: "mock-key"}
	emailSvc := NewEmailService(cfg)
	service := NewContractService(mockContractRepo, mockLotRepo, mockUserRepo, mockPaymentRepo, mockLedgerRepo, nil, emailSvc, auditSvc, worker)

	contract := &models.Contract{
		ID:              1,
		LotID:           1,
		Status:          models.ContractStatusPending,
		ApplicantUserID: 2,
	}

	lot := &models.Lot{
		ID:     1,
		Status: models.LotStatusReserved,
	}

	mockContractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	mockContractRepo.On("Update", ctx, mock.MatchedBy(func(c *models.Contract) bool {
		return c.Status == models.ContractStatusRejected && *c.RejectionReason == "Bad credit"
	})).Return(nil)

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.Status == models.LotStatusAvailable
	})).Return(nil)

	result, err := service.Reject(ctx, 1, "Bad credit")

	assert.NoError(t, err)
	assert.Equal(t, models.ContractStatusRejected, result.Status)

	mockContractRepo.AssertExpectations(t)
	mockLotRepo.AssertExpectations(t)
}

func TestContractService_Cancel(t *testing.T) {
	// Initialize logger to avoid nil pointer dereference
	logger.Setup("test")

	ctx := context.Background()
	mockContractRepo := new(mockContractRepo)
	mockLotRepo := new(mockLotRepo)
	mockUserRepo := new(mockUserRepoForContract)
	mockPaymentRepo := new(mockPaymentRepo)
	mockLedgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)

	cfg := &config.Config{ResendAPIKey: "mock-key"}
	emailSvc := NewEmailService(cfg)
	service := NewContractService(mockContractRepo, mockLotRepo, mockUserRepo, mockPaymentRepo, mockLedgerRepo, nil, emailSvc, auditSvc, worker)

	contract := &models.Contract{
		ID:              1,
		LotID:           1,
		Status:          models.ContractStatusSubmitted,
		ApplicantUserID: 2,
		Payments: []models.Payment{
			{ID: 1, Status: models.PaymentStatusPending},
			{ID: 2, Status: models.PaymentStatusPaid},
		},
	}

	lot := &models.Lot{
		ID:     1,
		Status: models.LotStatusFinanced,
	}

	mockContractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	mockContractRepo.On("Update", ctx, mock.MatchedBy(func(c *models.Contract) bool {
		return c.Status == models.ContractStatusCancelled && *c.Note == "User requested"
	})).Return(nil)

	mockPaymentRepo.On("Delete", ctx, uint(1)).Return(nil)
	mockLedgerRepo.On("DeleteByContractID", ctx, uint(1)).Return(nil)

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.Status == models.LotStatusAvailable
	})).Return(nil)

	result, err := service.Cancel(ctx, 1, "User requested")

	assert.NoError(t, err)
	assert.Equal(t, models.ContractStatusCancelled, result.Status)

	mockContractRepo.AssertExpectations(t)
	mockLotRepo.AssertExpectations(t)
	mockPaymentRepo.AssertExpectations(t)
	mockLedgerRepo.AssertExpectations(t)
}

func newPrepaymentService() (*ContractService, *mockContractRepo, *mockPaymentRepo, *mockLedgerRepo) {
	contractRepo := new(mockContractRepo)
	paymentRepo := new(mockPaymentRepo)
	ledgerRepo := new(mockLedgerRepo)
	service := NewContractService(
		contractRepo,
		new(mockLotRepo),
		new(mockUserRepoForContract),
		paymentRepo,
		ledgerRepo,
		nil,
		nil,
		nil,
		nil,
	)
	return service, contractRepo, paymentRepo, ledgerRepo
}

func TestContractService_CapitalRepayment_RequiresApprovedContract(t *testing.T) {
	logger.Setup("test")
	ctx := context.Background()
	service, contractRepo, paymentRepo, ledgerRepo := newPrepaymentService()

	contractRepo.On("FindByIDWithDetails", ctx, uint(7)).Return(&models.Contract{
		ID:     7,
		Status: models.ContractStatusSubmitted,
	}, nil)

	err := service.CapitalRepayment(ctx, 7, 1000, 1, "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approved")
	paymentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	ledgerRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestContractService_CapitalRepayment_RejectsAmountAboveBalance(t *testing.T) {
	logger.Setup("test")
	ctx := context.Background()
	service, contractRepo, paymentRepo, ledgerRepo := newPrepaymentService()

	contractRepo.On("FindByIDWithDetails", ctx, uint(7)).Return(&models.Contract{
		ID:     7,
		Status: models.ContractStatusApproved,
	}, nil)
	ledgerRepo.On("CalculateBalance", ctx, uint(7)).Return(-1000.0, nil)

	err := service.CapitalRepayment(ctx, 7, 1500, 1, "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "superar el saldo")
	paymentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	ledgerRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestContractService_CapitalRepayment_RejectsContractWithoutBalance(t *testing.T) {
	logger.Setup("test")
	ctx := context.Background()
	service, contractRepo, paymentRepo, ledgerRepo := newPrepaymentService()

	contractRepo.On("FindByIDWithDetails", ctx, uint(7)).Return(&models.Contract{
		ID:     7,
		Status: models.ContractStatusApproved,
	}, nil)
	ledgerRepo.On("CalculateBalance", ctx, uint(7)).Return(0.0, nil)

	err := service.CapitalRepayment(ctx, 7, 1000, 1, "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tiene saldo pendiente")
	paymentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestContractService_CreatePrepaymentRequest(t *testing.T) {
	logger.Setup("test")
	ctx := context.Background()
	service, contractRepo, paymentRepo, ledgerRepo := newPrepaymentService()

	contract := &models.Contract{
		ID:              7,
		GUID:            "contract-guid",
		Status:          models.ContractStatusApproved,
		ApplicantUserID: 2,
	}

	contractRepo.On("FindByGUIDWithDetails", ctx, "contract-guid").Return(contract, nil)
	ledgerRepo.On("CalculateBalance", ctx, uint(7)).Return(-10000.0, nil)
	paymentRepo.On("FindByContract", ctx, uint(7)).Return([]models.Payment{}, nil)
	paymentRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Payment) bool {
		return p.ContractID == 7 &&
			p.Amount == 2500 &&
			p.Status == models.PaymentStatusSubmitted &&
			p.PaymentType == models.PaymentTypeCapitalRepayment
	})).Return(nil)

	payment, err := service.CreatePrepaymentRequest(ctx, "contract-guid", 2500)

	assert.NoError(t, err)
	assert.Equal(t, 2500.0, payment.Amount)
	assert.Equal(t, models.PaymentStatusSubmitted, payment.Status)
	assert.Equal(t, models.PaymentTypeCapitalRepayment, payment.PaymentType)

	contractRepo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
	ledgerRepo.AssertExpectations(t)
}

func TestContractService_CreatePrepaymentRequest_Validation(t *testing.T) {
	logger.Setup("test")
	ctx := context.Background()

	tests := []struct {
		name       string
		amount     float64
		status     string
		balance    float64
		payments   []models.Payment
		errorMatch string
	}{
		{
			name:       "non-positive amount",
			amount:     0,
			status:     models.ContractStatusApproved,
			balance:    -10000,
			errorMatch: "mayor a cero",
		},
		{
			name:       "contract not approved",
			amount:     1000,
			status:     models.ContractStatusSubmitted,
			balance:    -10000,
			errorMatch: "contrato aprobado",
		},
		{
			name:       "no outstanding balance",
			amount:     1000,
			status:     models.ContractStatusApproved,
			balance:    0,
			errorMatch: "no tiene saldo pendiente",
		},
		{
			name:       "amount exceeds balance",
			amount:     12000,
			status:     models.ContractStatusApproved,
			balance:    -10000,
			errorMatch: "no puede superar el saldo",
		},
		{
			name:    "duplicate request in review",
			amount:  1000,
			status:  models.ContractStatusApproved,
			balance: -10000,
			payments: []models.Payment{
				{ID: 9, Status: models.PaymentStatusSubmitted, PaymentType: models.PaymentTypeCapitalRepayment},
			},
			errorMatch: "en revisión",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, contractRepo, paymentRepo, ledgerRepo := newPrepaymentService()
			contract := &models.Contract{
				ID:     7,
				GUID:   "contract-guid",
				Status: tc.status,
			}

			contractRepo.On("FindByGUIDWithDetails", ctx, "contract-guid").Return(contract, nil)
			if tc.amount > 0 {
				ledgerRepo.On("CalculateBalance", ctx, uint(7)).Return(tc.balance, nil).Maybe()
			}
			if tc.status == models.ContractStatusApproved && tc.balance < 0 {
				paymentRepo.On("FindByContract", ctx, uint(7)).Return(tc.payments, nil).Maybe()
			}

			_, err := service.CreatePrepaymentRequest(ctx, "contract-guid", tc.amount)

			assert.ErrorContains(t, err, tc.errorMatch)
			paymentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

// newContractServiceForActions wires a ContractService with testify repository
// mocks and a real AuditService backed by sqlmock. Email delivery is disabled so
// background jobs never make network calls.
func newContractServiceForActions(t *testing.T) (*ContractService, *mockContractRepo, *mockLotRepo, *mockUserRepoForContract, *mockPaymentRepo, *mockLedgerRepo, sqlmock.Sqlmock) {
	t.Helper()
	logger.Setup("test")

	contractRepo := new(mockContractRepo)
	lotRepo := new(mockLotRepo)
	userRepo := new(mockUserRepoForContract)
	paymentRepo := new(mockPaymentRepo)
	ledgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)
	t.Cleanup(worker.Shutdown)

	emailSvc := NewEmailService(&config.Config{EnableEmailNotifications: false})
	notificationSvc := NewNotificationService(nil, nil)

	service := NewContractService(contractRepo, lotRepo, userRepo, paymentRepo, ledgerRepo, notificationSvc, emailSvc, auditSvc, worker)
	return service, contractRepo, lotRepo, userRepo, paymentRepo, ledgerRepo, mockSQL
}

func TestContractService_Approve(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, lotRepo, _, paymentRepo, ledgerRepo, mockSQL := newContractServiceForActions(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	amount, down, reserve := 10000.0, 1000.0, 500.0
	contract := &models.Contract{
		ID:              1,
		LotID:           1,
		Status:          models.ContractStatusPending,
		Amount:          &amount,
		DownPayment:     &down,
		ReserveAmount:   &reserve,
		PaymentTerm:     2,
		FinancingType:   models.FinancingTypeDirect,
		ApplicantUserID: 2,
		Lot:             models.Lot{ID: 1, Name: "Lote 1", Project: models.Project{Name: "Proyecto"}},
		ApplicantUser:   models.User{ID: 2, FullName: "Cliente"},
	}

	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	contractRepo.On("Update", ctx, mock.AnythingOfType("*models.Contract")).Return(nil)
	ledgerRepo.On("Create", ctx, mock.MatchedBy(func(entry *models.ContractLedgerEntry) bool {
		return entry.EntryType == models.EntryTypeInitial && entry.Amount == -amount
	})).Return(nil)
	paymentRepo.On("FindByContract", ctx, uint(1)).Return([]models.Payment{}, nil)
	created := 0
	paymentRepo.On("Create", ctx, mock.AnythingOfType("*models.Payment")).
		Run(func(mock.Arguments) { created++ }).
		Return(nil)
	lotRepo.On("FindByID", ctx, uint(1)).Return(&models.Lot{ID: 1}, nil)
	lotRepo.On("Update", ctx, mock.AnythingOfType("*models.Lot")).Return(nil)

	approved, err := service.Approve(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, models.ContractStatusApproved, approved.Status)
	assert.NotNil(t, approved.ApprovedAt)
	assert.True(t, approved.Active)
	assert.Positive(t, created, "the generated payment schedule must be persisted")

	contractRepo.AssertExpectations(t)
	ledgerRepo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
	lotRepo.AssertExpectations(t)
}

func TestContractService_Approve_KeepsExistingReservation(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, lotRepo, _, paymentRepo, ledgerRepo, mockSQL := newContractServiceForActions(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	amount, down, reserve := 10000.0, 1000.0, 500.0
	contract := &models.Contract{
		ID:              1,
		LotID:           1,
		Status:          models.ContractStatusPending,
		Amount:          &amount,
		DownPayment:     &down,
		ReserveAmount:   &reserve,
		PaymentTerm:     2,
		FinancingType:   models.FinancingTypeDirect,
		ApplicantUserID: 2,
		Lot:             models.Lot{ID: 1, Name: "Lote 1", Project: models.Project{Name: "Proyecto"}},
	}

	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	contractRepo.On("Update", ctx, mock.AnythingOfType("*models.Contract")).Return(nil)
	ledgerRepo.On("Create", ctx, mock.AnythingOfType("*models.ContractLedgerEntry")).Return(nil)
	paymentRepo.On("FindByContract", ctx, uint(1)).Return([]models.Payment{
		{ID: 9, ContractID: 1, PaymentType: models.PaymentTypeReservation, Status: models.PaymentStatusPending},
	}, nil)

	var createdTypes []string
	paymentRepo.On("Create", ctx, mock.AnythingOfType("*models.Payment")).
		Run(func(args mock.Arguments) {
			createdTypes = append(createdTypes, args.Get(1).(*models.Payment).PaymentType)
		}).
		Return(nil)
	lotRepo.On("FindByID", ctx, uint(1)).Return(&models.Lot{ID: 1}, nil)
	lotRepo.On("Update", ctx, mock.AnythingOfType("*models.Lot")).Return(nil)

	_, err := service.Approve(ctx, 1)

	assert.NoError(t, err)
	assert.NotContains(t, createdTypes, models.PaymentTypeReservation,
		"the reservation payment created at contract creation must not be duplicated")
	assert.Positive(t, len(createdTypes))
}

func TestContractService_Approve_RejectsInvalidState(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, _, _, paymentRepo, _, _ := newContractServiceForActions(t)

	amount := 10000.0
	contract := &models.Contract{ID: 1, Status: models.ContractStatusApproved, Amount: &amount}
	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)

	_, err := service.Approve(ctx, 1)

	assert.ErrorContains(t, err, "cannot approve contract")
	paymentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	contractRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestContractService_Approve_PropagatesLedgerError(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, _, _, paymentRepo, ledgerRepo, _ := newContractServiceForActions(t)

	amount := 10000.0
	contract := &models.Contract{
		ID:              1,
		LotID:           1,
		Status:          models.ContractStatusPending,
		Amount:          &amount,
		PaymentTerm:     2,
		ApplicantUserID: 2,
	}
	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	contractRepo.On("Update", ctx, mock.AnythingOfType("*models.Contract")).Return(nil)
	ledgerRepo.On("Create", ctx, mock.MatchedBy(func(entry *models.ContractLedgerEntry) bool {
		return entry.EntryType == models.EntryTypeInitial && entry.Amount == -amount
	})).Return(errors.New("ledger unavailable"))

	_, err := service.Approve(ctx, 1)

	assert.ErrorContains(t, err, "failed to create initial ledger entry")
	paymentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestContractService_Close_RejectsOutstandingBalance(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, _, _, _, ledgerRepo, _ := newContractServiceForActions(t)

	balance := 0.0
	contract := &models.Contract{ID: 1, Status: models.ContractStatusApproved, Balance: &balance}
	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	ledgerRepo.On("CalculateBalance", ctx, uint(1)).Return(-250.0, nil)

	_, err := service.Close(ctx, 1)

	assert.ErrorContains(t, err, "outstanding balance")
	contractRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestContractService_Close_RejectsInvalidState(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, _, _, _, ledgerRepo, _ := newContractServiceForActions(t)

	balance := 0.0
	contract := &models.Contract{ID: 1, Status: models.ContractStatusSubmitted, Balance: &balance}
	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	ledgerRepo.On("CalculateBalance", ctx, uint(1)).Return(0.0, nil)

	_, err := service.Close(ctx, 1)

	assert.ErrorContains(t, err, "cannot close contract")
	contractRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestContractService_Close_Success(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, lotRepo, _, _, ledgerRepo, mockSQL := newContractServiceForActions(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	balance := 0.0
	contract := &models.Contract{ID: 1, LotID: 1, Status: models.ContractStatusApproved, Balance: &balance, ApplicantUserID: 2}
	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	ledgerRepo.On("CalculateBalance", ctx, uint(1)).Return(0.0, nil)
	contractRepo.On("Update", ctx, mock.MatchedBy(func(c *models.Contract) bool {
		return c.Status == models.ContractStatusClosed && !c.Active && c.ClosedAt != nil
	})).Return(nil)
	lotRepo.On("FindByID", ctx, uint(1)).Return(&models.Lot{ID: 1, Status: models.LotStatusFinanced}, nil)
	lotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.Status == models.LotStatusFullyPaid
	})).Return(nil)

	closed, err := service.Close(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, models.ContractStatusClosed, closed.Status)
	assert.NotNil(t, closed.ClosedAt)
	assert.False(t, closed.Active)
	contractRepo.AssertExpectations(t)
	lotRepo.AssertExpectations(t)
}

func TestContractService_DeleteRejected(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, lotRepo, _, paymentRepo, ledgerRepo, mockSQL := newContractServiceForActions(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	contract := &models.Contract{ID: 1, LotID: 1, Status: models.ContractStatusRejected, ApplicantUserID: 2}
	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	paymentRepo.On("DeleteByContract", ctx, uint(1)).Return(nil)
	ledgerRepo.On("DeleteByContractID", ctx, uint(1)).Return(nil)
	contractRepo.On("Delete", ctx, uint(1)).Return(nil)
	lotRepo.On("FindByID", ctx, uint(1)).Return(&models.Lot{ID: 1, Status: models.LotStatusReserved}, nil)
	lotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.Status == models.LotStatusAvailable
	})).Return(nil)

	err := service.DeleteRejected(ctx, 1)

	assert.NoError(t, err)
	contractRepo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
	ledgerRepo.AssertExpectations(t)
	lotRepo.AssertExpectations(t)
}

func TestContractService_DeleteRejected_RejectsOtherStates(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, _, _, paymentRepo, ledgerRepo, _ := newContractServiceForActions(t)

	contract := &models.Contract{ID: 1, Status: models.ContractStatusPending}
	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)

	err := service.DeleteRejected(ctx, 1)

	assert.ErrorContains(t, err, "rechazado")
	paymentRepo.AssertNotCalled(t, "DeleteByContract", mock.Anything, mock.Anything)
	ledgerRepo.AssertNotCalled(t, "DeleteByContractID", mock.Anything, mock.Anything)
	contractRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestContractService_ResendAccountSetup(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		service, contractRepo, _, userRepo, _, _, _ := newContractServiceForActions(t)
		contractRepo.On("FindByGUIDWithDetails", ctx, "guid").
			Return(&models.Contract{GUID: "guid", ApplicantUser: models.User{ID: 5, MustChangePassword: true}}, nil)
		userRepo.On("Update", ctx, mock.AnythingOfType("*models.User")).Return(nil)

		err := service.ResendAccountSetup(ctx, "guid")

		assert.NoError(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("account already configured", func(t *testing.T) {
		service, contractRepo, _, userRepo, _, _, _ := newContractServiceForActions(t)
		contractRepo.On("FindByGUIDWithDetails", ctx, "guid").
			Return(&models.Contract{GUID: "guid", ApplicantUser: models.User{ID: 5, MustChangePassword: false}}, nil)

		err := service.ResendAccountSetup(ctx, "guid")

		assert.ErrorIs(t, err, ErrAccountAlreadyConfigured)
		userRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("contract not found", func(t *testing.T) {
		service, contractRepo, _, userRepo, _, _, _ := newContractServiceForActions(t)
		contractRepo.On("FindByGUIDWithDetails", ctx, "missing").Return(nil, errors.New("not found"))

		err := service.ResendAccountSetup(ctx, "missing")

		assert.ErrorIs(t, err, ErrNotFound)
		userRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})
}

func TestContractService_CapitalRepayment_Success(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, _, _, paymentRepo, ledgerRepo, mockSQL := newContractServiceForActions(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	contract := &models.Contract{ID: 7, Status: models.ContractStatusApproved, ApplicantUserID: 2}
	// The balance refresh runs on the worker context, so match any context.
	contractRepo.On("FindByIDWithDetails", mock.Anything, uint(7)).Return(contract, nil)
	ledgerRepo.On("CalculateBalance", mock.Anything, uint(7)).Return(-1000.0, nil)

	paymentRepo.On("Create", ctx, mock.MatchedBy(func(p *models.Payment) bool {
		return p.ContractID == 7 &&
			p.PaymentType == models.PaymentTypeCapitalRepayment &&
			p.Status == models.PaymentStatusPaid &&
			p.Amount == 1000
	})).Return(nil)
	ledgerRepo.On("Create", ctx, mock.MatchedBy(func(e *models.ContractLedgerEntry) bool {
		return e.ContractID == 7 &&
			e.EntryType == models.EntryTypePrepayment &&
			e.Amount == 1000
	})).Return(nil)
	paymentRepo.On("FindByContract", ctx, uint(7)).Return([]models.Payment{
		{ID: 20, ContractID: 7, Amount: 1000, Status: models.PaymentStatusPending, PaymentType: models.PaymentTypeInstallment},
		{ID: 21, ContractID: 7, Amount: 1000, Status: models.PaymentStatusPending, PaymentType: models.PaymentTypeInstallment},
	}, nil)
	paymentRepo.On("Delete", ctx, uint(21)).Return(nil)
	// The balance refresh happens asynchronously; make it optional for the
	// deterministic assertions below.
	contractRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Contract")).Return(nil).Maybe()

	err := service.CapitalRepayment(ctx, 7, 1000, 1, "", "")

	assert.NoError(t, err)
	paymentRepo.AssertExpectations(t)
	ledgerRepo.AssertExpectations(t)
}

func TestContractService_CapitalRepayment_AutoClosesSettledContract(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, _, _, paymentRepo, ledgerRepo, _ := newContractServiceForActions(t)

	contract := &models.Contract{ID: 7, Status: models.ContractStatusApproved, ApplicantUserID: 2}
	// The auto-close path runs on the worker context, so match any context and
	// sequence the balance: outstanding first, settled afterwards.
	contractRepo.On("FindByIDWithDetails", mock.Anything, uint(7)).Return(contract, nil)
	ledgerRepo.On("CalculateBalance", mock.Anything, uint(7)).Return(-1000.0, nil).Once()
	ledgerRepo.On("CalculateBalance", mock.Anything, uint(7)).Return(0.0, nil).Maybe()
	paymentRepo.On("Create", ctx, mock.AnythingOfType("*models.Payment")).Return(nil)
	ledgerRepo.On("Create", ctx, mock.AnythingOfType("*models.ContractLedgerEntry")).Return(nil)
	paymentRepo.On("FindByContract", ctx, uint(7)).Return([]models.Payment{}, nil)

	closed := make(chan struct{}, 1)
	contractRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Contract")).
		Run(func(args mock.Arguments) {
			c := args.Get(1).(*models.Contract)
			if c.Status == models.ContractStatusClosed {
				select {
				case closed <- struct{}{}:
				default:
				}
			}
		}).
		Return(nil).
		Maybe()

	err := service.CapitalRepayment(ctx, 7, 1000, 1, "", "")
	assert.NoError(t, err)

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("expected the settled contract to be auto-closed")
	}
}

func TestContractService_Update(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, _, _, _, _, _ := newContractServiceForActions(t)

	contract := &models.Contract{ID: 1, Status: models.ContractStatusPending, PaymentTerm: 12}
	contractRepo.On("Update", ctx, contract).Return(nil)

	err := service.Update(ctx, contract)

	assert.NoError(t, err)
	contractRepo.AssertExpectations(t)
}

func TestContractService_ReleaseUnpaidReservations(t *testing.T) {
	ctx := context.Background()
	service, contractRepo, lotRepo, _, _, ledgerRepo, mockSQL := newContractServiceForActions(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	stale := []models.Contract{
		{ID: 1, LotID: 1, Status: models.ContractStatusPending, ApplicantUserID: 2},
	}
	contractRepo.On("FindPendingReservations", ctx, 48).Return(stale, nil)
	contractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(&stale[0], nil)
	contractRepo.On("Update", ctx, mock.MatchedBy(func(c *models.Contract) bool {
		return c.Status == models.ContractStatusCancelled
	})).Return(nil)
	ledgerRepo.On("DeleteByContractID", ctx, uint(1)).Return(nil)
	lotRepo.On("FindByID", ctx, uint(1)).Return(&models.Lot{ID: 1, Status: models.LotStatusReserved}, nil)
	lotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.Status == models.LotStatusAvailable
	})).Return(nil)

	err := service.ReleaseUnpaidReservations(ctx)

	assert.NoError(t, err)
	contractRepo.AssertExpectations(t)
	lotRepo.AssertExpectations(t)
}

func TestContractService_MarkCommissionPaid(t *testing.T) {
	logger.Setup("test")

	ctx := context.Background()
	contractRepo := new(mockContractRepo)
	lotRepo := new(mockLotRepo)
	userRepo := new(mockUserRepoForContract)
	paymentRepo := new(mockPaymentRepo)
	ledgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)
	cfg := &config.Config{ResendAPIKey: "mock-key"}
	emailSvc := NewEmailService(cfg)
	service := NewContractService(contractRepo, lotRepo, userRepo, paymentRepo, ledgerRepo, nil, emailSvc, auditSvc, worker)

	amount := 400000.0
	contract := &models.Contract{
		ID:               7,
		Status:           models.ContractStatusApproved,
		CommissionAmount: 0,
		Amount:           &amount,
		FinancingType:    models.FinancingTypeDirect,
		Lot: models.Lot{
			Project: models.Project{ID: 3, CommissionRateDirect: 4},
		},
	}

	contractRepo.On("FindByIDWithDetails", ctx, uint(7)).Return(contract, nil)
	contractRepo.On("MarkCommissionPaid", ctx, uint(7), uint(9), 16000.0).Return(nil)

	updated, err := service.MarkCommissionPaid(ctx, 7, 9)

	assert.NoError(t, err)
	assert.Equal(t, 16000.0, updated.CommissionAmount)
	assert.NotNil(t, updated.CommissionPaidAt)
	assert.Equal(t, uint(9), *updated.CommissionPaidBy)
	contractRepo.AssertExpectations(t)
}

func TestContractService_MarkCommissionPaid_RejectsAlreadyPaid(t *testing.T) {
	logger.Setup("test")

	ctx := context.Background()
	contractRepo := new(mockContractRepo)
	service := NewContractService(contractRepo, new(mockLotRepo), new(mockUserRepoForContract), new(mockPaymentRepo), new(mockLedgerRepo), nil, nil, nil, jobs.NewWorker(1))

	paidAt := time.Now()
	contract := &models.Contract{
		ID:               8,
		Status:           models.ContractStatusApproved,
		CommissionAmount: 1000,
		CommissionPaidAt: &paidAt,
	}
	contractRepo.On("FindByIDWithDetails", ctx, uint(8)).Return(contract, nil)

	_, err := service.MarkCommissionPaid(ctx, 8, 9)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ya fue marcada")
	contractRepo.AssertExpectations(t)
}
