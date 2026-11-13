package services

import (
	"context"
	"errors"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPaymentService_ensureReservationContractApproved(t *testing.T) {
	ctx := context.Background()

	t.Run("non-reservation payments skip the contract check", func(t *testing.T) {
		contractRepo := new(mockContractRepo)
		service := &PaymentService{contractRepo: contractRepo}

		err := service.ensureReservationContractApproved(ctx, &models.Payment{
			ID:          1,
			ContractID:  7,
			PaymentType: models.PaymentTypeInstallment,
		})

		assert.NoError(t, err)
		contractRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
	})

	t.Run("reservation payments require an approved contract", func(t *testing.T) {
		contractRepo := new(mockContractRepo)
		contractRepo.On("FindByID", ctx, uint(7)).Return(&models.Contract{
			ID:     7,
			Status: models.ContractStatusSubmitted,
		}, nil)
		service := &PaymentService{contractRepo: contractRepo}

		err := service.ensureReservationContractApproved(ctx, &models.Payment{
			ID:          2,
			ContractID:  7,
			PaymentType: models.PaymentTypeReservation,
		})

		assert.ErrorIs(t, err, ErrContractNotApproved)
	})

	t.Run("reservation payments pass once the contract is approved", func(t *testing.T) {
		contractRepo := new(mockContractRepo)
		contractRepo.On("FindByID", ctx, uint(7)).Return(&models.Contract{
			ID:     7,
			Status: models.ContractStatusApproved,
		}, nil)
		service := &PaymentService{contractRepo: contractRepo}

		err := service.ensureReservationContractApproved(ctx, &models.Payment{
			ID:          3,
			ContractID:  7,
			PaymentType: models.PaymentTypeReservation,
		})

		assert.NoError(t, err)
	})

	t.Run("contract lookup failures are returned", func(t *testing.T) {
		contractRepo := new(mockContractRepo)
		contractRepo.On("FindByID", ctx, uint(7)).Return(nil, errors.New("db down"))
		service := &PaymentService{contractRepo: contractRepo}

		err := service.ensureReservationContractApproved(ctx, &models.Payment{
			ID:          4,
			ContractID:  7,
			PaymentType: models.PaymentTypeReservation,
		})

		assert.Error(t, err)
		assert.NotErrorIs(t, err, ErrContractNotApproved)
	})
}

func TestPaymentService_ApproveBlocksReservationWithoutApprovedContract(t *testing.T) {
	ctx := context.Background()

	paymentRepo := new(mockPaymentRepo)
	contractRepo := new(mockContractRepo)
	service := &PaymentService{repo: paymentRepo, contractRepo: contractRepo}

	paymentRepo.On("FindByID", ctx, uint(11)).Return(&models.Payment{
		ID:          11,
		ContractID:  7,
		Amount:      500,
		Status:      models.PaymentStatusSubmitted,
		PaymentType: models.PaymentTypeReservation,
	}, nil)
	contractRepo.On("FindByID", ctx, uint(7)).Return(&models.Contract{
		ID:     7,
		Status: models.ContractStatusPending,
	}, nil)

	_, err := service.Approve(ctx, 11, 0, 0, 0, 1, "", "")

	assert.ErrorIs(t, err, ErrContractNotApproved)
	paymentRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestPaymentService_ApprovePaymentWithLedgerBlocksReservationWithoutApprovedContract(t *testing.T) {
	ctx := context.Background()

	paymentRepo := new(mockPaymentRepo)
	contractRepo := new(mockContractRepo)
	ledgerRepo := new(mockLedgerRepo)
	service := &PaymentService{repo: paymentRepo, contractRepo: contractRepo, ledgerRepo: ledgerRepo}

	paymentRepo.On("FindByID", ctx, uint(12)).Return(&models.Payment{
		ID:          12,
		ContractID:  7,
		Amount:      500,
		Status:      models.PaymentStatusPending,
		PaymentType: models.PaymentTypeReservation,
	}, nil)
	contractRepo.On("FindByID", ctx, uint(7)).Return(&models.Contract{
		ID:     7,
		Status: models.ContractStatusSubmitted,
	}, nil)

	err := service.ApprovePaymentWithLedger(ctx, 12, 500)

	assert.ErrorIs(t, err, ErrContractNotApproved)
	paymentRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	ledgerRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPaymentService_UpdateReceiptClearsRejectionReason(t *testing.T) {
	ctx := context.Background()

	paymentRepo := new(mockPaymentRepo)
	service := &PaymentService{repo: paymentRepo}
	reason := "comprobante ilegible"

	paymentRepo.On("FindByID", ctx, uint(5)).Return(&models.Payment{
		ID:              5,
		ContractID:      7,
		Amount:          500,
		Status:          models.PaymentStatusRejected,
		PaymentType:     models.PaymentTypeInstallment,
		RejectionReason: &reason,
	}, nil)
	paymentRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Payment) bool {
		return p.ID == 5 &&
			p.Status == models.PaymentStatusSubmitted &&
			p.RejectionReason == nil &&
			p.DocumentPath != nil &&
			*p.DocumentPath == "receipts/2026/11/new.pdf"
	})).Return(nil)

	err := service.UpdateReceipt(ctx, 5, "receipts/2026/11/new.pdf", "", "", 0)

	assert.NoError(t, err)
	paymentRepo.AssertExpectations(t)
}
