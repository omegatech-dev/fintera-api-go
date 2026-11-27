package models

import (
	"testing"
	"time"
)

// TestHoldExpiryIsInvariantWhenTheProcessRunsInUTC pins the production
// guarantee: a process running in UTC produces the exact instant the previous
// implementation produced, so the fix is a no-op in production.
func TestHoldExpiryIsInvariantWhenTheProcessRunsInUTC(t *testing.T) {
	created := time.Date(2026, 11, 26, 22, 10, 25, 0, time.UTC)

	for _, hold := range []time.Duration{15 * time.Hour, 48 * time.Hour, 72 * time.Hour} {
		previous := SettingsPolicySnapshot()
		t.Cleanup(func() { SetSettingsPolicy(previous) })
		policy := SettingsPolicySnapshot()
		policy.HoldHours = int(hold.Hours())
		SetSettingsPolicy(policy)

		if got, want := holdExpiryIn(created, time.UTC), created.Add(hold); !got.Equal(want) {
			t.Errorf("UTC process: holdExpiryIn = %v, want %v", got, want)
		}
	}
}

// TestHoldExpiryRebuildsTheWallClock verifies the local fix: a value read back
// from a `timestamp without time zone` column is reinterpreted in the write
// zone, so the hold starts at the configured duration.
func TestHoldExpiryRebuildsTheWallClock(t *testing.T) {
	honduras, err := time.LoadLocation("America/Tegucigalpa")
	if err != nil {
		t.Skip("tzdata not available")
	}
	previous := SettingsPolicySnapshot()
	t.Cleanup(func() { SetSettingsPolicy(previous) })
	policy := SettingsPolicySnapshot()
	policy.HoldHours = 15
	SetSettingsPolicy(policy)

	// pgx hands back the stored wall clock labelled as UTC midnight fields.
	stored := time.Date(2026, 11, 26, 22, 10, 25, 0, time.UTC)

	got := holdExpiryIn(stored, honduras)
	// The true write instant was 2026-11-27 04:10:25 UTC; plus 15h.
	want := time.Date(2026, 11, 27, 19, 10, 25, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("holdExpiryIn = %v, want %v", got, want)
	}
}

// TestHoldExpiryLeavesAZoneLocalValueUntouched verifies that rebuilding does
// not move a value that is already expressed in the write zone: the hold is
// measured from the stored wall clock either way.
func TestHoldExpiryLeavesAZoneLocalValueUntouched(t *testing.T) {
	honduras, err := time.LoadLocation("America/Tegucigalpa")
	if err != nil {
		t.Skip("tzdata not available")
	}
	previous := SettingsPolicySnapshot()
	t.Cleanup(func() { SetSettingsPolicy(previous) })
	policy := SettingsPolicySnapshot()
	policy.HoldHours = 15
	SetSettingsPolicy(policy)

	created := time.Date(2026, 11, 26, 22, 10, 25, 0, honduras)
	if got, want := holdExpiryIn(created, honduras), created.Add(15*time.Hour); !got.Equal(want) {
		t.Errorf("holdExpiryIn = %v, want %v", got, want)
	}
}
