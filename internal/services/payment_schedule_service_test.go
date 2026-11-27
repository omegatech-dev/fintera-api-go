package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
)

func scheduleTestContract(financing string) *models.Contract {
	amount := 240000.0
	down := 20000.0
	reserve := 5000.0
	term := 12
	max := time.Now().AddDate(0, 0, 30)
	return &models.Contract{
		ID:             1,
		Amount:         &amount,
		DownPayment:    &down,
		ReserveAmount:  &reserve,
		PaymentTerm:    term,
		FinancingType:  financing,
		MaxPaymentDate: &max,
	}
}

// withSchedulePolicy updates the process-wide policy for a test and restores it.
func withSchedulePolicy(t *testing.T, policy models.SettingsPolicy) {
	t.Helper()
	previous := models.SettingsPolicySnapshot()
	t.Cleanup(func() { models.SetSettingsPolicy(previous) })
	models.SetSettingsPolicy(policy)
}

func TestScheduleUsesConfiguredReservationWindow(t *testing.T) {
	withSchedulePolicy(t, models.SettingsPolicy{
		HoldHours:                         72,
		MinAmount:                         5000,
		ReservationDueDays:                3,
		DownPaymentDueDays:                14,
		DownPaymentDueDaysWithReservation: 21,
		EmailNotificationsEnabled:         true,
		OverdueRemindersEnabled:           true,
		UpcomingReminderDays:              1,
		DefaultCurrency:                   "HNL",
		DefaultLocale:                     "es",
		Timezone:                          "America/Tegucigalpa",
	})

	service := NewPaymentScheduleService()
	payments, err := service.GenerateSchedule(context.Background(), scheduleTestContract(models.FinancingTypeDirect))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(payments) == 0 {
		t.Fatal("expected payments")
	}
	var reservation *models.Payment
	for i := range payments {
		if payments[i].PaymentType == models.PaymentTypeReservation {
			reservation = &payments[i]
		}
	}
	if reservation == nil {
		t.Fatal("expected a reservation payment")
	}
	days := int(time.Until(reservation.DueDate).Hours() / 24)
	if days < 2 || days > 3 {
		t.Errorf("reservation due in %d days, want 3", days)
	}

	var down *models.Payment
	for i := range payments {
		if payments[i].PaymentType == models.PaymentTypeDownPayment {
			down = &payments[i]
		}
	}
	if down == nil {
		t.Fatal("expected a down payment")
	}
	// The contract carries a reservation, so the wider window applies.
	days = int(time.Until(down.DueDate).Hours() / 24)
	if days < 20 || days > 21 {
		t.Errorf("down payment due in %d days, want 21", days)
	}
}

func TestScheduleDownPaymentWithoutReservation(t *testing.T) {
	withSchedulePolicy(t, models.SettingsPolicy{
		HoldHours:                         72,
		MinAmount:                         5000,
		ReservationDueDays:                3,
		DownPaymentDueDays:                14,
		DownPaymentDueDaysWithReservation: 21,
		EmailNotificationsEnabled:         true,
		OverdueRemindersEnabled:           true,
		UpcomingReminderDays:              1,
		DefaultCurrency:                   "HNL",
		DefaultLocale:                     "es",
		Timezone:                          "America/Tegucigalpa",
	})

	contract := scheduleTestContract(models.FinancingTypeDirect)
	zero := 0.0
	contract.ReserveAmount = &zero

	payments, err := NewPaymentScheduleService().GenerateSchedule(context.Background(), contract)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := range payments {
		if payments[i].PaymentType != models.PaymentTypeDownPayment {
			continue
		}
		days := int(time.Until(payments[i].DueDate).Hours() / 24)
		if days < 13 || days > 14 {
			t.Errorf("down payment due in %d days, want 14", days)
		}
		return
	}
	t.Fatal("expected a down payment")
}

func TestScheduleBankUsesConfiguredReservationWindow(t *testing.T) {
	withSchedulePolicy(t, models.SettingsPolicy{
		HoldHours:                         72,
		MinAmount:                         5000,
		ReservationDueDays:                3,
		DownPaymentDueDays:                14,
		DownPaymentDueDaysWithReservation: 21,
		EmailNotificationsEnabled:         true,
		OverdueRemindersEnabled:           true,
		UpcomingReminderDays:              1,
		DefaultCurrency:                   "HNL",
		DefaultLocale:                     "es",
		Timezone:                          "America/Tegucigalpa",
	})

	payments, err := NewPaymentScheduleService().GenerateSchedule(context.Background(), scheduleTestContract(models.FinancingTypeBank))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var reservation *models.Payment
	for i := range payments {
		if payments[i].PaymentType == models.PaymentTypeReservation {
			reservation = &payments[i]
		}
	}
	if reservation == nil {
		t.Fatal("expected a reservation payment")
	}
	days := int(time.Until(reservation.DueDate).Hours() / 24)
	if days < 2 || days > 3 {
		t.Errorf("reservation due in %d days, want 3", days)
	}
}

// withPolicyZone pins the deployment time zone for a test and restores it.
func withPolicyZone(t *testing.T, name string) {
	t.Helper()
	previous := models.SettingsPolicySnapshot()
	t.Cleanup(func() { models.SetSettingsPolicy(previous) })
	policy := previous
	policy.Timezone = name
	models.SetSettingsPolicy(policy)
}

// TestScheduleDueDatesFollowTheBusinessCalendar guards the business calendar:
// the windows the admin configures are counted in Honduras days, not in the
// days the server's process zone happens to be on. The before/after window
// tolerates the rare case of the test running across business midnight.
func TestScheduleDueDatesFollowTheBusinessCalendar(t *testing.T) {
	withPolicyZone(t, "America/Tegucigalpa")

	window := models.SettingsPolicySnapshot().ReservationDueDays
	before := models.BusinessNow().AddDate(0, 0, window).Format(models.DateOnlyLayout)
	payments, err := NewPaymentScheduleService().GenerateSchedule(context.Background(), scheduleTestContract(models.FinancingTypeDirect))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := models.BusinessNow().AddDate(0, 0, window).Format(models.DateOnlyLayout)

	var reservation *models.Payment
	for i := range payments {
		if payments[i].PaymentType == models.PaymentTypeReservation {
			reservation = &payments[i]
		}
	}
	if reservation == nil {
		t.Fatal("expected a reservation payment")
	}
	got := reservation.DueDate.Format(models.DateOnlyLayout)
	if got != before && got != after {
		t.Errorf("reservation due %s, want business day +%d (%s..%s)", got, window, before, after)
	}
}
