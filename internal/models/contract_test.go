package models

import (
	"testing"
	"time"
)

func TestContractResponse_HoldExpiresAt(t *testing.T) {
	previous := ReservationHoldHours()
	t.Cleanup(func() { SetReservationHoldHours(previous) })
	SetReservationHoldHours(48)

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

	contract.Status = ContractStatusSubmitted
	if got := contract.ToResponse().HoldExpiresAt; got == nil {
		t.Fatal("expected hold_expires_at while the reservation is submitted")
	}

	for _, status := range []string{
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

func TestContractResponse_LotMeasurementUnit(t *testing.T) {
	ft2 := "ft2"
	contract := Contract{
		ID:     1,
		Status: ContractStatusApproved,
		Lot: Lot{
			MeasurementUnit: &ft2,
			Project:         Project{MeasurementUnit: MeasurementUnitM2},
		},
	}
	resp := contract.ToResponse()
	if resp.LotMeasurementUnit == nil || *resp.LotMeasurementUnit != MeasurementUnitFT2 {
		t.Fatalf("lot_measurement_unit = %v, want ft2", resp.LotMeasurementUnit)
	}

	contract.Lot.MeasurementUnit = nil
	resp = contract.ToResponse()
	if resp.LotMeasurementUnit == nil || *resp.LotMeasurementUnit != MeasurementUnitM2 {
		t.Fatalf("lot_measurement_unit fallback = %v, want m2", resp.LotMeasurementUnit)
	}
}

func TestNormalizeBankCreditStatus(t *testing.T) {
	cases := map[string]bool{
		"":                true,
		"not_started":     true,
		" Pre_Qualified ": true,
		"pre_qualified":   true,
		"pre_approved":    true,
		"approved":        true,
		"APPROVED":        true,
		"maybe":           false,
		"rejected":        false,
	}
	for input, wantOK := range cases {
		got, ok := NormalizeBankCreditStatus(input)
		if ok != wantOK {
			t.Errorf("NormalizeBankCreditStatus(%q) ok = %v, want %v", input, ok, wantOK)
			continue
		}
		if ok && input == "APPROVED" && got != BankCreditStatusApproved {
			t.Errorf("normalized = %q, want %q", got, BankCreditStatusApproved)
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
