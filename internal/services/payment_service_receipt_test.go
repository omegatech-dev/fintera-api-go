package services

import (
	"context"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestPaymentService_ValidateReceiptUpload(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects an already paid payment", func(t *testing.T) {
		ledgerRepo := new(mockLedgerRepo)
		service := &PaymentService{ledgerRepo: ledgerRepo}
		payment := &models.Payment{
			ID:         1,
			ContractID: 7,
			Status:     models.PaymentStatusPaid,
		}

		err := service.ValidateReceiptUpload(ctx, payment)

		assert.ErrorContains(t, err, "ya no admite comprobante")
		ledgerRepo.AssertNotCalled(t, "CalculateBalance", ctx, uint(7))
	})

	t.Run("rejects a closed contract", func(t *testing.T) {
		ledgerRepo := new(mockLedgerRepo)
		service := &PaymentService{ledgerRepo: ledgerRepo}
		payment := &models.Payment{
			ID:         1,
			ContractID: 7,
			Status:     models.PaymentStatusPending,
			Contract:   models.Contract{ID: 7, Status: models.ContractStatusClosed},
		}

		err := service.ValidateReceiptUpload(ctx, payment)

		assert.ErrorContains(t, err, "liquidado")
		ledgerRepo.AssertNotCalled(t, "CalculateBalance", ctx, uint(7))
	})

	t.Run("rejects an approved contract without outstanding balance", func(t *testing.T) {
		ledgerRepo := new(mockLedgerRepo)
		service := &PaymentService{ledgerRepo: ledgerRepo}
		payment := &models.Payment{
			ID:         1,
			ContractID: 7,
			Status:     models.PaymentStatusPending,
			Contract:   models.Contract{ID: 7, Status: models.ContractStatusApproved},
		}

		ledgerRepo.On("CalculateBalance", ctx, uint(7)).Return(0.0, nil)

		err := service.ValidateReceiptUpload(ctx, payment)

		assert.ErrorContains(t, err, "no tiene saldo pendiente")
		ledgerRepo.AssertExpectations(t)
	})

	t.Run("allows an approved contract with debt", func(t *testing.T) {
		ledgerRepo := new(mockLedgerRepo)
		service := &PaymentService{ledgerRepo: ledgerRepo}
		payment := &models.Payment{
			ID:         1,
			ContractID: 7,
			Status:     models.PaymentStatusPending,
			Contract:   models.Contract{ID: 7, Status: models.ContractStatusApproved},
		}

		ledgerRepo.On("CalculateBalance", ctx, uint(7)).Return(-65000.0, nil)

		err := service.ValidateReceiptUpload(ctx, payment)

		assert.NoError(t, err)
		ledgerRepo.AssertExpectations(t)
	})

	t.Run("allows the reservation of a pending contract", func(t *testing.T) {
		ledgerRepo := new(mockLedgerRepo)
		service := &PaymentService{ledgerRepo: ledgerRepo}
		payment := &models.Payment{
			ID:          1,
			ContractID:  7,
			Status:      models.PaymentStatusPending,
			PaymentType: models.PaymentTypeReservation,
			Contract:    models.Contract{ID: 7, Status: models.ContractStatusPending},
		}

		err := service.ValidateReceiptUpload(ctx, payment)

		assert.NoError(t, err)
		ledgerRepo.AssertNotCalled(t, "CalculateBalance", ctx, uint(7))
	})
}
