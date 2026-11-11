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
)

func TestPaymentService_applyToPendingInstallments(t *testing.T) {
	ctx := context.Background()

	t.Run("reduces the last installment when the amount is partial", func(t *testing.T) {
		repo := new(mockPaymentRepo)
		service := &PaymentService{repo: repo}

		current := &models.Payment{ID: 1, ContractID: 7, Status: models.PaymentStatusSubmitted}
		pending := []models.Payment{
			{ID: 2, ContractID: 7, Amount: 100, Status: models.PaymentStatusPending},
			{ID: 3, ContractID: 7, Amount: 200, Status: models.PaymentStatusPending},
			{ID: 9, ContractID: 7, Amount: 500, Status: models.PaymentStatusPaid},
		}

		repo.On("FindByContract", ctx, uint(7)).Return(pending, nil)
		repo.On("Delete", ctx, uint(3)).Return(nil)
		repo.On("Update", ctx, mock.MatchedBy(func(payment *models.Payment) bool {
			return payment.ID == 2 && payment.Amount == 50
		})).Return(nil)

		err := service.applyToPendingInstallments(ctx, current, 250)

		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("excludes the payment being approved", func(t *testing.T) {
		repo := new(mockPaymentRepo)
		service := &PaymentService{repo: repo}

		current := &models.Payment{ID: 3, ContractID: 7, Status: models.PaymentStatusSubmitted}
		pending := []models.Payment{
			{ID: 2, ContractID: 7, Amount: 100, Status: models.PaymentStatusPending},
			{ID: 3, ContractID: 7, Amount: 200, Status: models.PaymentStatusSubmitted},
		}

		repo.On("FindByContract", ctx, uint(7)).Return(pending, nil)
		repo.On("Delete", ctx, uint(2)).Return(nil)

		err := service.applyToPendingInstallments(ctx, current, 100)

		assert.NoError(t, err)
		repo.AssertNotCalled(t, "Delete", ctx, uint(3))
		repo.AssertExpectations(t)
	})

	t.Run("ignores non-positive amounts", func(t *testing.T) {
		repo := new(mockPaymentRepo)
		service := &PaymentService{repo: repo}

		err := service.applyToPendingInstallments(ctx, &models.Payment{ID: 1, ContractID: 7}, 0)

		assert.NoError(t, err)
		repo.AssertNotCalled(t, "FindByContract", mock.Anything, mock.Anything)
	})
}

func TestPaymentService_ApproveCapitalRepayment(t *testing.T) {
	logger.Setup("test")
	ctx := context.Background()

	paymentRepo := new(mockPaymentRepo)
	contractRepo := new(mockContractRepo)
	ledgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)

	service := &PaymentService{
		repo:         paymentRepo,
		contractRepo: contractRepo,
		ledgerRepo:   ledgerRepo,
		auditSvc:     auditSvc,
		worker:       worker,
	}

	prepayment := &models.Payment{
		ID:          11,
		ContractID:  7,
		Amount:      1000,
		Status:      models.PaymentStatusSubmitted,
		PaymentType: models.PaymentTypeCapitalRepayment,
	}
	installments := []models.Payment{
		*prepayment,
		{ID: 12, ContractID: 7, Amount: 500, Status: models.PaymentStatusPending},
		{ID: 13, ContractID: 7, Amount: 800, Status: models.PaymentStatusPending},
	}

	paymentRepo.On("FindByID", ctx, uint(11)).Return(prepayment, nil)
	paymentRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Payment) bool {
		return p.ID == 11 && p.Status == models.PaymentStatusPaid && p.PaidAmount != nil && *p.PaidAmount == 1000
	})).Return(nil)
	paymentRepo.On("FindByContract", ctx, uint(7)).Return(installments, nil)
	paymentRepo.On("Delete", ctx, uint(13)).Return(nil)
	paymentRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Payment) bool {
		return p.ID == 12 && p.Amount == 300
	})).Return(nil)
	ledgerRepo.On("Create", ctx, mock.MatchedBy(func(entry *models.ContractLedgerEntry) bool {
		return entry.EntryType == models.EntryTypePrepayment && entry.Amount == 1000
	})).Return(nil)

	// Background jobs (balance refresh and notification) are best-effort.
	ledgerRepo.On("CalculateBalance", mock.Anything, uint(7)).Return(-500.0, nil).Maybe()
	contractRepo.On("FindByID", mock.Anything, uint(7)).
		Return(&models.Contract{ID: 7, Status: models.ContractStatusApproved}, nil).Maybe()
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	contractRepo.On("FindByIDWithDetails", mock.Anything, uint(7)).
		Return(nil, errors.New("skip notification")).Maybe()

	payment, err := service.Approve(ctx, 11, 0, 0, 0, 1, "", "")

	assert.NoError(t, err)
	assert.Equal(t, models.PaymentStatusPaid, payment.Status)
	paymentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	paymentRepo.AssertExpectations(t)
	ledgerRepo.AssertExpectations(t)
}
