package handlers

import (
	"testing"
)

func TestParseMaxPaymentDate(t *testing.T) {
	t.Run("cash requires a date", func(t *testing.T) {
		if _, err := parseMaxPaymentDate("cash", ""); err == nil {
			t.Fatal("expected an error for cash without a date")
		}
	})

	t.Run("bank accepts no date", func(t *testing.T) {
		got, err := parseMaxPaymentDate("bank", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("date = %v, want nil", got)
		}
	})

	t.Run("bank parses a provided date", func(t *testing.T) {
		got, err := parseMaxPaymentDate("bank", "2099-01-15")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.Year() != 2099 || got.Month() != 1 || got.Day() != 15 {
			t.Fatalf("date = %v, want 2099-01-15", got)
		}
	})

	t.Run("direct ignores the date", func(t *testing.T) {
		got, err := parseMaxPaymentDate("direct", "2099-01-15")
		if err != nil || got != nil {
			t.Fatalf("got %v, err %v; want nil, nil", got, err)
		}
	})

	t.Run("past dates are rejected", func(t *testing.T) {
		if _, err := parseMaxPaymentDate("cash", "2000-01-01"); err == nil {
			t.Fatal("expected an error for a past date")
		}
	})

	t.Run("malformed dates are rejected", func(t *testing.T) {
		if _, err := parseMaxPaymentDate("cash", "15/01/2099"); err == nil {
			t.Fatal("expected an error for a malformed date")
		}
	})
}

func TestParseBankCreditDetails(t *testing.T) {
	t.Run("bank stores both values", func(t *testing.T) {
		name, status, err := parseBankCreditDetails("bank", "  Banco Atlántida ", " Pre_Qualified ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name == nil || *name != "Banco Atlántida" {
			t.Fatalf("name = %v", name)
		}
		if status == nil || *status != "pre_qualified" {
			t.Fatalf("status = %v", status)
		}
	})

	t.Run("bank allows empty values", func(t *testing.T) {
		name, status, err := parseBankCreditDetails("bank", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != nil || status != nil {
			t.Fatalf("name = %v, status = %v; want nil, nil", name, status)
		}
	})

	t.Run("invalid status is rejected", func(t *testing.T) {
		if _, _, err := parseBankCreditDetails("bank", "BAC", "maybe"); err == nil {
			t.Fatal("expected an error for an invalid credit status")
		}
	})

	t.Run("long bank names are rejected", func(t *testing.T) {
		long := make([]rune, maxBankNameLength+1)
		for i := range long {
			long[i] = 'a'
		}
		if _, _, err := parseBankCreditDetails("bank", string(long), ""); err == nil {
			t.Fatal("expected an error for a long bank name")
		}
	})

	t.Run("non-bank modalities ignore the fields", func(t *testing.T) {
		for _, financingType := range []string{"direct", "cash"} {
			name, status, err := parseBankCreditDetails(financingType, "BAC", "approved")
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", financingType, err)
			}
			if name != nil || status != nil {
				t.Fatalf("%s: name = %v, status = %v; want nil, nil", financingType, name, status)
			}
		}
	})
}
