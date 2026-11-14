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

func TestContractResponse_NextPayment(t *testing.T) {
	document := "receipts/2026/11/voucher.pdf"
	earlier := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	contract := Contract{
		ID:     7,
		Status: ContractStatusApproved,
		Payments: []Payment{
			{
				ID:           10,
				ContractID:   7,
				Amount:       500,
				PaymentType:  PaymentTypeReservation,
				Status:       PaymentStatusPaid,
				DueDate:      earlier,
				DocumentPath: &document,
			},
			{
				ID:          11,
				ContractID:  7,
				Amount:      1000,
				PaymentType: PaymentTypeDownPayment,
				Status:      PaymentStatusPending,
				DueDate:     later,
				Description: stringPtrForModelTest("Pago Inicial"),
			},
			{
				ID:          12,
				ContractID:  7,
				Amount:      900,
				PaymentType: PaymentTypeInstallment,
				Status:      PaymentStatusPending,
				DueDate:     earlier,
				Description: stringPtrForModelTest("Cuota 1 de 12"),
			},
			{
				ID:           13,
				ContractID:   7,
				Amount:       900,
				PaymentType:  PaymentTypeInstallment,
				Status:       PaymentStatusSubmitted,
				DueDate:      earlier.Add(-time.Hour),
				Description:  stringPtrForModelTest("Cuota 1 de 12"),
				DocumentPath: &document,
			},
		},
	}

	resp := contract.ToResponse()
	if resp.NextPayment == nil {
		t.Fatal("expected next_payment in response")
	}
	if resp.NextPayment.ID != 12 {
		t.Fatalf("next_payment id = %d, want 12 (earliest without receipt)", resp.NextPayment.ID)
	}
	if resp.NextPayment.Label != "Cuota 1" {
		t.Fatalf("next_payment label = %q, want Cuota 1", resp.NextPayment.Label)
	}
	if resp.NextPaymentID == nil || *resp.NextPaymentID != 12 {
		t.Fatalf("next_payment_id = %v, want 12", resp.NextPaymentID)
	}
	if resp.NextPaymentAmount == nil || *resp.NextPaymentAmount != 900 {
		t.Fatalf("next_payment_amount = %v, want 900", resp.NextPaymentAmount)
	}
	if resp.NextPaymentDate == nil || !resp.NextPaymentDate.Equal(earlier) {
		t.Fatalf("next_payment_date = %v, want %v", resp.NextPaymentDate, earlier)
	}
	if resp.NextPaymentLabel != "Cuota 1" || resp.NextPaymentType != PaymentTypeInstallment {
		t.Fatalf("flat next payment fields = %q/%q", resp.NextPaymentType, resp.NextPaymentLabel)
	}

	contract.Status = ContractStatusCancelled
	if got := contract.ToResponse().NextPayment; got != nil {
		t.Fatalf("cancelled contract next_payment = %+v, want nil", got)
	}
}

func TestPaymentCollectionLabel(t *testing.T) {
	description := "Cuota 3 de 12"
	cases := []struct {
		payment Payment
		want    string
	}{
		{payment: Payment{PaymentType: PaymentTypeReservation}, want: "Reserva"},
		{payment: Payment{PaymentType: PaymentTypeDownPayment}, want: "Prima"},
		{payment: Payment{PaymentType: PaymentTypeFull}, want: "Total"},
		{payment: Payment{PaymentType: PaymentTypeCapitalRepayment}, want: "Abono a capital"},
		{payment: Payment{PaymentType: PaymentTypeInstallment, Description: &description}, want: "Cuota 3"},
		{payment: Payment{PaymentType: PaymentTypeInstallment}, want: "Cuota"},
	}
	for _, testCase := range cases {
		if got := testCase.payment.CollectionLabel(); got != testCase.want {
			t.Errorf("CollectionLabel(%q) = %q, want %q", testCase.payment.PaymentType, got, testCase.want)
		}
	}
}

func stringPtrForModelTest(value string) *string {
	return &value
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
	if resp.ReservationPayment.ReceiptURL == nil || *resp.ReservationPayment.ReceiptURL != "/receipts/2026/11/voucher.pdf" {
		t.Fatalf("receipt_url = %v", resp.ReservationPayment.ReceiptURL)
	}
	if len(resp.PaymentSchedule) != 1 {
		t.Fatalf("payment_schedule len = %d, want 1", len(resp.PaymentSchedule))
	}
}

func TestContractResponseRedactForCustomer(t *testing.T) {
	sellerID := uint(7)
	now := time.Now()
	guid := "8f14e45f-ceea-4a67-9c0f-1f2d3e4a5b6c"
	resp := ContractResponse{
		CommissionAmount:      1000,
		CommissionOrigin:      CommissionOriginDirect,
		CommissionPaidAt:      &now,
		CommissionPaidBy:      &sellerID,
		CommissionAccrualDate: &now,
		Seller:                "Cristofer",
		SellerID:              &sellerID,
		AssignmentOrigin:      AssignmentOriginManual,
		AssignmentStatus:      AssignmentStatusAccepted,
		AssignedAt:            &now,
		AcceptDeadlineAt:      &now,
		RespondedAt:           &now,
		ReferralGUID:          &guid,
	}

	redacted := resp.RedactForCustomer()
	if redacted.CommissionAmount != 0 || redacted.CommissionOrigin != "" ||
		redacted.SellerID != nil || redacted.AssignmentOrigin != "" || redacted.AssignmentStatus != "" ||
		redacted.AssignedAt != nil || redacted.AcceptDeadlineAt != nil || redacted.RespondedAt != nil ||
		redacted.ReferralGUID != nil || redacted.CommissionPaidAt != nil || redacted.CommissionPaidBy != nil ||
		redacted.CommissionAccrualDate != nil {
		t.Fatalf("customer payload not fully redacted: %+v", redacted)
	}
}

func TestContractResponseSellerContact(t *testing.T) {
	seller := &User{FullName: "Cristofer Hernández", Phone: "+504 3184-9999", Email: "cristofer@example.com"}
	contract := &Contract{Seller: seller}

	resp := contract.ToResponse()
	if resp.SellerContact == nil {
		t.Fatal("expected seller_contact")
	}
	if resp.SellerContact.Name != seller.FullName || resp.SellerContact.Phone != seller.Phone || resp.SellerContact.Email != seller.Email {
		t.Fatalf("seller_contact = %+v", resp.SellerContact)
	}

	redacted := resp.RedactForCustomer()
	if redacted.SellerContact == nil || redacted.SellerContact.Name != seller.FullName {
		t.Fatalf("customer lost seller_contact: %+v", redacted.SellerContact)
	}
	if redacted.SellerID != nil || redacted.CommissionAmount != 0 || redacted.ReferralGUID != nil || redacted.AssignmentStatus != "" {
		t.Fatalf("internal fields leaked: %+v", redacted)
	}
}
