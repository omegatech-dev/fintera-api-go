package models

import (
	"testing"
	"time"
)

func TestHoldExpiresAtUsesConfiguredHours(t *testing.T) {
	previous := ReservationHoldHours()
	t.Cleanup(func() { SetReservationHoldHours(previous) })

	created := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)

	SetReservationHoldHours(48)
	if got, want := HoldExpiresAt(created), created.Add(48*time.Hour); !got.Equal(want) {
		t.Errorf("48h expiry = %v, want %v", got, want)
	}

	SetReservationHoldHours(72)
	if got, want := HoldExpiresAt(created), created.Add(72*time.Hour); !got.Equal(want) {
		t.Errorf("72h expiry = %v, want %v", got, want)
	}

	SetReservationHoldHours(0)
	if got := ReservationHoldHours(); got != 72 {
		t.Errorf("invalid hold should be ignored, got %d", got)
	}
	SetReservationHoldHours(previous)
}
