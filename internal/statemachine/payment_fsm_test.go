package statemachine

import (
	"context"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
)

var paymentStatuses = []string{
	models.PaymentStatusPending,
	models.PaymentStatusSubmitted,
	models.PaymentStatusPaid,
	models.PaymentStatusRejected,
	models.PaymentStatusReadjustment,
}

func TestPaymentFSM_Submit(t *testing.T) {
	ctx := context.Background()
	receipt := "receipts/2026/11/voucher.pdf"

	tests := []struct {
		name    string
		status  string
		receipt *string
		wantErr bool
	}{
		{name: "pending with receipt", status: models.PaymentStatusPending, receipt: &receipt},
		{name: "pending without receipt", status: models.PaymentStatusPending, receipt: nil, wantErr: true},
		{name: "already submitted", status: models.PaymentStatusSubmitted, receipt: &receipt, wantErr: true},
		{name: "paid", status: models.PaymentStatusPaid, receipt: &receipt, wantErr: true},
		{name: "rejected", status: models.PaymentStatusRejected, receipt: &receipt, wantErr: true},
		{name: "readjustment", status: models.PaymentStatusReadjustment, receipt: &receipt, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payment := &models.Payment{Status: tc.status, DocumentPath: tc.receipt}

			err := NewPaymentFSM(payment).Submit(ctx)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.status, payment.Status, "status must not change on a rejected transition")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, models.PaymentStatusSubmitted, payment.Status)
		})
	}
}

func TestPaymentFSM_Approve(t *testing.T) {
	ctx := context.Background()
	allowed := map[string]bool{
		models.PaymentStatusPending:   true,
		models.PaymentStatusSubmitted: true,
	}

	for _, status := range paymentStatuses {
		t.Run("from "+status, func(t *testing.T) {
			payment := &models.Payment{Status: status}

			err := NewPaymentFSM(payment).Approve(ctx)

			if allowed[status] {
				assert.NoError(t, err)
				assert.Equal(t, models.PaymentStatusPaid, payment.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, payment.Status, "status must not change on a rejected transition")
		})
	}
}

func TestPaymentFSM_Reject(t *testing.T) {
	ctx := context.Background()

	for _, status := range paymentStatuses {
		t.Run("from "+status, func(t *testing.T) {
			payment := &models.Payment{Status: status}

			err := NewPaymentFSM(payment).Reject(ctx)

			if status == models.PaymentStatusSubmitted {
				assert.NoError(t, err)
				assert.Equal(t, models.PaymentStatusRejected, payment.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, payment.Status, "status must not change on a rejected transition")
		})
	}
}

func TestPaymentFSM_Undo(t *testing.T) {
	ctx := context.Background()

	for _, status := range paymentStatuses {
		t.Run("from "+status, func(t *testing.T) {
			payment := &models.Payment{Status: status}

			err := NewPaymentFSM(payment).Undo(ctx)

			if status == models.PaymentStatusPaid {
				assert.NoError(t, err)
				assert.Equal(t, models.PaymentStatusSubmitted, payment.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, payment.Status, "status must not change on a rejected transition")
		})
	}
}

func TestPaymentFSM_Readjustment(t *testing.T) {
	ctx := context.Background()

	for _, status := range paymentStatuses {
		t.Run("from "+status, func(t *testing.T) {
			payment := &models.Payment{Status: status}

			err := NewPaymentFSM(payment).Readjustment(ctx)

			if status == models.PaymentStatusPending {
				assert.NoError(t, err)
				assert.Equal(t, models.PaymentStatusReadjustment, payment.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, payment.Status, "status must not change on a rejected transition")
		})
	}
}

func TestPaymentFSM_Can(t *testing.T) {
	payment := &models.Payment{Status: models.PaymentStatusSubmitted}
	fsm := NewPaymentFSM(payment)

	assert.True(t, fsm.Can("approve"))
	assert.True(t, fsm.Can("reject"))
	assert.False(t, fsm.Can("undo"))
	assert.Equal(t, models.PaymentStatusSubmitted, fsm.Current())
}
