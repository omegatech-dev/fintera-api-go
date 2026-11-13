package services

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newPaymentLedgerTestService builds a PaymentService wired to mocks. The
// background balance refresh is expected to fail on purpose so the async path
// exits before it needs a ContractService; tests can still override the
// CalculateBalance expectation when they need it to succeed.
func newPaymentLedgerTestService(t *testing.T) (*PaymentService, *mockPaymentRepo, *mockContractRepo, *mockLedgerRepo, sqlmock.Sqlmock) {
	t.Helper()
	logger.Setup("test")

	paymentRepo := new(mockPaymentRepo)
	contractRepo := new(mockContractRepo)
	ledgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	worker := jobs.NewWorker(1)
	t.Cleanup(worker.Shutdown)

	service := &PaymentService{
		repo:            paymentRepo,
		contractRepo:    contractRepo,
		ledgerRepo:      ledgerRepo,
		notificationSvc: NewNotificationService(nil, nil),
		auditSvc:        NewAuditService(gdb),
		worker:          worker,
	}

	return service, paymentRepo, contractRepo, ledgerRepo, mockSQL
}

func TestPaymentService_Approve_CreatesPaymentLedgerEntry(t *testing.T) {
	ctx := context.Background()
	service, paymentRepo, contractRepo, ledgerRepo, mockSQL := newPaymentLedgerTestService(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	payment := &models.Payment{
		ID:          11,
		ContractID:  7,
		Amount:      1000,
		Status:      models.PaymentStatusSubmitted,
		PaymentType: models.PaymentTypeInstallment,
	}

	paymentRepo.On("FindByID", ctx, uint(11)).Return(payment, nil)
	paymentRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Payment) bool {
		return p.Status == models.PaymentStatusPaid &&
			p.PaidAmount != nil && *p.PaidAmount == 1000 &&
			p.PaymentDate != nil && p.ApprovedAt != nil
	})).Return(nil)
	ledgerRepo.On("Create", ctx, mock.MatchedBy(func(e *models.ContractLedgerEntry) bool {
		return e.ContractID == 7 &&
			e.PaymentID != nil && *e.PaymentID == 11 &&
			e.EntryType == models.EntryTypePayment &&
			e.Amount == 1000
	})).Return(nil)

	// Async balance refresh and notification are best effort.
	ledgerRepo.On("CalculateBalance", mock.Anything, uint(7)).
		Return(0.0, errors.New("skip async balance refresh")).Maybe()
	contractRepo.On("FindByIDWithDetails", mock.Anything, uint(7)).
		Return(nil, errors.New("skip async notification")).Maybe()

	approved, err := service.Approve(ctx, 11, 0, 0, 1000, 1, "", "")

	require.NoError(t, err)
	assert.Equal(t, models.PaymentStatusPaid, approved.Status)
	paymentRepo.AssertExpectations(t)
	ledgerRepo.AssertExpectations(t)
	require.NoError(t, mockSQL.ExpectationsWereMet())
}

func TestPaymentService_Approve_SplitsPaymentAndPrepaymentEntries(t *testing.T) {
	ctx := context.Background()
	service, paymentRepo, contractRepo, ledgerRepo, mockSQL := newPaymentLedgerTestService(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	payment := &models.Payment{
		ID:          11,
		ContractID:  7,
		Amount:      1000,
		Status:      models.PaymentStatusSubmitted,
		PaymentType: models.PaymentTypeInstallment,
	}

	paymentRepo.On("FindByID", ctx, uint(11)).Return(payment, nil)
	paymentRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Payment) bool {
		return p.Status == models.PaymentStatusPaid && p.PaidAmount != nil && *p.PaidAmount == 1500
	})).Return(nil)
	// The 500 overpayment is applied to the pending installment.
	paymentRepo.On("FindByContract", ctx, uint(7)).Return([]models.Payment{
		{ID: 12, ContractID: 7, Amount: 500, Status: models.PaymentStatusPending, PaymentType: models.PaymentTypeInstallment},
	}, nil)
	paymentRepo.On("Delete", ctx, uint(12)).Return(nil)

	ledgerRepo.On("Create", ctx, mock.MatchedBy(func(e *models.ContractLedgerEntry) bool {
		return e.EntryType == models.EntryTypePayment && e.Amount == 1000
	})).Return(nil)
	ledgerRepo.On("Create", ctx, mock.MatchedBy(func(e *models.ContractLedgerEntry) bool {
		return e.EntryType == models.EntryTypePrepayment && e.Amount == 500
	})).Return(nil)

	ledgerRepo.On("CalculateBalance", mock.Anything, uint(7)).
		Return(0.0, errors.New("skip async balance refresh")).Maybe()
	contractRepo.On("FindByIDWithDetails", mock.Anything, uint(7)).
		Return(nil, errors.New("skip async notification")).Maybe()

	_, err := service.Approve(ctx, 11, 0, 0, 1500, 1, "", "")

	require.NoError(t, err)
	paymentRepo.AssertExpectations(t)
	ledgerRepo.AssertExpectations(t)
	require.NoError(t, mockSQL.ExpectationsWereMet())
}

func TestPaymentService_UndoPayment_CreatesReversalEntry(t *testing.T) {
	ctx := context.Background()
	service, paymentRepo, contractRepo, ledgerRepo, _ := newPaymentLedgerTestService(t)

	paidAmount := 1000.0
	payment := &models.Payment{
		ID:          11,
		ContractID:  7,
		Amount:      1000,
		Status:      models.PaymentStatusPaid,
		PaymentType: models.PaymentTypeInstallment,
		PaidAmount:  &paidAmount,
	}

	paymentRepo.On("FindByID", ctx, uint(11)).Return(payment, nil)
	ledgerRepo.On("FindByPaymentID", ctx, uint(11)).Return([]models.ContractLedgerEntry{{
		ID:          3,
		ContractID:  7,
		PaymentID:   &payment.ID,
		Amount:      1000,
		Description: "Pago Recibido",
		EntryType:   models.EntryTypePayment,
	}}, nil)
	ledgerRepo.On("Create", ctx, mock.MatchedBy(func(e *models.ContractLedgerEntry) bool {
		return e.ContractID == 7 &&
			e.PaymentID != nil && *e.PaymentID == 11 &&
			e.EntryType == models.EntryTypeAdjustment &&
			e.Amount == -1000
	})).Return(nil)
	paymentRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Payment) bool {
		return p.Status == models.PaymentStatusSubmitted &&
			p.PaidAmount == nil && p.PaymentDate == nil && p.ApprovedAt == nil
	})).Return(nil)

	contractRepo.On("FindByIDWithDetails", mock.Anything, uint(7)).
		Return(nil, errors.New("skip async notification")).Maybe()

	require.NoError(t, service.UndoPayment(ctx, 11))
	assert.Nil(t, payment.PaidAmount)
	paymentRepo.AssertExpectations(t)
	ledgerRepo.AssertExpectations(t)
}

func TestPaymentService_UndoPayment_RejectsInstallmentSettledByCapitalRepayment(t *testing.T) {
	ctx := context.Background()
	service, paymentRepo, _, ledgerRepo, _ := newPaymentLedgerTestService(t)

	zero := 0.0
	paymentRepo.On("FindByID", ctx, uint(11)).Return(&models.Payment{
		ID:          11,
		ContractID:  7,
		Amount:      1000,
		Status:      models.PaymentStatusPaid,
		PaymentType: models.PaymentTypeInstallment,
		PaidAmount:  &zero,
	}, nil)

	err := service.UndoPayment(ctx, 11)

	require.ErrorContains(t, err, "abono a capital")
	ledgerRepo.AssertNotCalled(t, "FindByPaymentID", mock.Anything, mock.Anything)
	ledgerRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	paymentRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestPaymentService_UndoPayment_PropagatesLedgerLookupError(t *testing.T) {
	ctx := context.Background()
	service, paymentRepo, _, ledgerRepo, _ := newPaymentLedgerTestService(t)

	paidAmount := 1000.0
	paymentRepo.On("FindByID", ctx, uint(11)).Return(&models.Payment{
		ID:          11,
		ContractID:  7,
		Amount:      1000,
		Status:      models.PaymentStatusPaid,
		PaymentType: models.PaymentTypeInstallment,
		PaidAmount:  &paidAmount,
	}, nil)
	ledgerRepo.On("FindByPaymentID", ctx, uint(11)).Return([]models.ContractLedgerEntry{}, errors.New("db down"))

	err := service.UndoPayment(ctx, 11)

	require.ErrorContains(t, err, "failed to find ledger entries")
	ledgerRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	paymentRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}
