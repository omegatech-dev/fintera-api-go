package models

import (
	"testing"
	"time"
)

// withBusinessZone pins the deployment time zone for a test and restores it.
// America/Tegucigalpa is UTC-6 with no DST, so the offset stays stable.
func withBusinessZone(t *testing.T, name string) {
	t.Helper()
	previous := SettingsPolicySnapshot()
	t.Cleanup(func() { SetSettingsPolicy(previous) })
	policy := previous
	policy.Timezone = name
	SetSettingsPolicy(policy)
}

// storedDate mimics how pgx returns a `date` column: the stored calendar date
// labelled as UTC midnight.
func storedDate(t *testing.T, year int, month time.Month, day int) time.Time {
	t.Helper()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestIsOverdueRespectsBusinessCalendar(t *testing.T) {
	withBusinessZone(t, "America/Tegucigalpa")

	today := BusinessNow()
	cases := []struct {
		name string
		due  time.Time
		want bool
	}{
		{"due today is not overdue", storedDate(t, today.Year(), today.Month(), today.Day()), false},
		{"due tomorrow is not overdue", storedDate(t, today.AddDate(0, 0, 1).Year(), today.AddDate(0, 0, 1).Month(), today.AddDate(0, 0, 1).Day()), false},
		{"due yesterday is overdue", storedDate(t, today.AddDate(0, 0, -1).Year(), today.AddDate(0, 0, -1).Month(), today.AddDate(0, 0, -1).Day()), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payment := Payment{Status: PaymentStatusPending, DueDate: tc.due}
			if got := payment.IsOverdue(); got != tc.want {
				t.Errorf("IsOverdue(due %s) = %v, want %v", tc.due.Format(DateOnlyLayout), got, tc.want)
			}
		})
	}

	// A paid payment is never overdue, whatever the date.
	paid := Payment{Status: PaymentStatusPaid, DueDate: storedDate(t, today.AddDate(0, 0, -1).Year(), today.AddDate(0, 0, -1).Month(), today.AddDate(0, 0, -1).Day())}
	if paid.IsOverdue() {
		t.Error("a paid payment must not be overdue")
	}
}

func TestOverdueDaysCountsBusinessDays(t *testing.T) {
	withBusinessZone(t, "America/Tegucigalpa")

	today := BusinessNow()
	threeDaysAgo := storedDate(t, today.AddDate(0, 0, -3).Year(), today.AddDate(0, 0, -3).Month(), today.AddDate(0, 0, -3).Day())
	payment := Payment{Status: PaymentStatusPending, DueDate: threeDaysAgo}
	if got := payment.OverdueDays(); got != 3 {
		t.Errorf("OverdueDays() = %d, want 3", got)
	}
}

func TestDateOnlyPtrFormatsStoredCalendarDate(t *testing.T) {
	// 2026-11-27T00:00:00Z must render as 2026-11-27, not as 2026-11-26, even
	// though the browser is six hours behind.
	value := storedDate(t, 2026, time.November, 27)
	formatted := DateOnlyPtr(&value)
	if formatted == nil || *formatted != "2026-11-27" {
		t.Errorf("DateOnlyPtr() = %v, want 2026-11-27", formatted)
	}
	if DateOnlyPtr(nil) != nil {
		t.Error("DateOnlyPtr(nil) must be nil")
	}
}

func TestIsBeforeBusinessDayIgnoresTheProcessZone(t *testing.T) {
	withBusinessZone(t, "America/Tegucigalpa")

	// The stored due date is the 27th; business "now" is the 26th at 8 p.m.
	// (02:00 UTC of the 27th). The payment is not overdue: the process zone,
	// wherever it runs, must not move the due date.
	due := storedDate(t, 2026, time.November, 27)
	now := time.Date(2026, time.November, 27, 2, 0, 0, 0, time.UTC)
	if IsBeforeBusinessDay(due, now) {
		t.Error("a payment due on the 27th is not overdue while the business day is the 26th")
	}

	// One business day later it is.
	now = time.Date(2026, time.November, 28, 6, 0, 0, 0, time.UTC)
	if !IsBeforeBusinessDay(due, now) {
		t.Error("a payment due on the 27th is overdue on the 28th")
	}
}

func TestBusinessDayDiffIsWholeCalendarDays(t *testing.T) {
	withBusinessZone(t, "America/Tegucigalpa")

	due := storedDate(t, 2026, time.November, 20)
	// 15:00 UTC = 9 a.m. in Honduras: the business day is the 27th.
	now := time.Date(2026, time.November, 27, 15, 0, 0, 0, time.UTC)
	if got := BusinessDayDiff(due, now); got != 7 {
		t.Errorf("BusinessDayDiff() = %d, want 7", got)
	}
	// A future date is not negative: overdue days never go backwards.
	if got := BusinessDayDiff(storedDate(t, 2026, time.November, 30), now); got != 0 {
		t.Errorf("BusinessDayDiff(future) = %d, want 0", got)
	}
}
