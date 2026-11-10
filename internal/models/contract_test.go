package models

import (
	"testing"
	"time"
)

func TestContractResponse_HoldExpiresAt(t *testing.T) {
	created := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	contract := Contract{
		ID:        1,
		GUID:      "contract-guid",
		Status:    ContractStatusPending,
		CreatedAt: created,
	}

	resp := contract.ToResponse()
	if resp.HoldExpiresAt == nil {
		t.Fatal("expected hold_expires_at for pending contract")
	}
	want := created.Add(48 * time.Hour)
	if !resp.HoldExpiresAt.Equal(want) {
		t.Fatalf("hold_expires_at = %v, want %v", resp.HoldExpiresAt, want)
	}

	for _, status := range []string{
		ContractStatusSubmitted,
		ContractStatusApproved,
		ContractStatusRejected,
		ContractStatusCancelled,
	} {
		contract.Status = status
		if got := contract.ToResponse().HoldExpiresAt; got != nil {
			t.Fatalf("hold_expires_at for %s = %v, want nil", status, got)
		}
	}
}

func TestContractResponse_ReservationPayment(t *testing.T) {
	paidAt := time.Date(2026, 11, 10, 8, 30, 0, 0, time.UTC)
	document := "receipts/2026/11/voucher.pdf"
	amount := 500.0
	contract := Contract{
		ID:        7,
		Status:    ContractStatusSubmitted,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		Payments: []Payment{
			{
				ID:           11,
				ContractID:   7,
				Amount:       amount,
				PaymentType:  PaymentTypeReservation,
				Status:       PaymentStatusSubmitted,
				PaymentDate:  &paidAt,
				DocumentPath: &document,
			},
		},
	}

	resp := contract.ToResponse()
	if resp.ReservationPayment == nil {
		t.Fatal("expected reservation_payment in response")
	}
	if resp.ReservationPayment.PaymentType != PaymentTypeReservation {
		t.Fatalf("payment_type = %q, want reservation", resp.ReservationPayment.PaymentType)
	}
	if resp.ReservationPayment.PaidAt == nil || !resp.ReservationPayment.PaidAt.Equal(paidAt) {
		t.Fatalf("paid_at = %v, want %v", resp.ReservationPayment.PaidAt, paidAt)
	}
	if resp.ReservationPayment.ReceiptURL == nil || *resp.ReservationPayment.ReceiptURL != "/uploads/receipts/2026/11/voucher.pdf" {
		t.Fatalf("receipt_url = %v", resp.ReservationPayment.ReceiptURL)
	}
	if len(resp.PaymentSchedule) != 1 {
		t.Fatalf("payment_schedule len = %d, want 1", len(resp.PaymentSchedule))
	}
}
