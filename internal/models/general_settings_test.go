package models

import (
	"testing"
	"time"
)

func TestHoldExpiresAtUsesConfiguredHours(t *testing.T) {
	previous := SettingsPolicySnapshot()
	t.Cleanup(func() { restorePolicy(t, previous.HoldHours) })

	// The stored wall clock is 08:00; the expiry is that wall clock plus the
	// hold, regardless of which zone the process runs in.
	created := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	stored := time.Date(2026, 11, 10, 8, 0, 0, 0, time.Local)

	setHoldHours(t, 48)
	if got, want := HoldExpiresAt(created), stored.Add(48*time.Hour); !got.Equal(want) {
		t.Errorf("48h expiry = %v, want %v", got, want)
	}

	setHoldHours(t, 72)
	if got, want := HoldExpiresAt(created), stored.Add(72*time.Hour); !got.Equal(want) {
		t.Errorf("72h expiry = %v, want %v", got, want)
	}

	// An invalid hold falls back to the deployment default, not to the last
	// value: the service validates before storing, so the model only has to
	// guarantee a usable duration.
	setHoldHours(t, 0)
	if got := ReservationHoldHours(); got != DefaultReservationHoldHours {
		t.Errorf("invalid hold should fall back to the default, got %d", got)
	}
}

// setHoldHours updates the process-wide policy for the test and restores it.
func setHoldHours(t *testing.T, hours int) {
	t.Helper()
	policy := SettingsPolicySnapshot()
	policy.HoldHours = hours
	SetSettingsPolicy(policy)
}

func restorePolicy(t *testing.T, hours int) {
	t.Helper()
	setHoldHours(t, hours)
}
