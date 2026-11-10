package services

import (
	"errors"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
)

func TestValidateGeneralSettings(t *testing.T) {
	valid := models.GeneralSettings{
		SupportWhatsApp:      "+50499998888",
		SupportEmail:         "soporte@fintera.test",
		ReservationHoldHours: 48,
	}
	if err := validateGeneralSettings(&valid); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := validateGeneralSettings(&models.GeneralSettings{ReservationHoldHours: 48}); err != nil {
		t.Fatalf("empty contact should be valid: %v", err)
	}

	cases := []struct {
		name     string
		settings models.GeneralSettings
		want     error
	}{
		{"short whatsapp", models.GeneralSettings{SupportWhatsApp: "123", ReservationHoldHours: 48}, ErrInvalidSupportWhatsApp},
		{"letters in whatsapp", models.GeneralSettings{SupportWhatsApp: "504abc1234", ReservationHoldHours: 48}, ErrInvalidSupportWhatsApp},
		{"bad email", models.GeneralSettings{SupportEmail: "nope", ReservationHoldHours: 48}, ErrInvalidSupportEmail},
		{"zero hold", models.GeneralSettings{ReservationHoldHours: 0}, ErrInvalidHoldHours},
		{"hold too large", models.GeneralSettings{ReservationHoldHours: 721}, ErrInvalidHoldHours},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGeneralSettings(&tc.settings)
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestApplySettingsDefaults(t *testing.T) {
	settings := &models.GeneralSettings{SupportWhatsApp: "  +504  "}
	applySettingsDefaults(settings)
	if settings.ID != 1 {
		t.Errorf("id = %d, want 1", settings.ID)
	}
	if settings.SupportWhatsApp != "+504" {
		t.Errorf("whatsapp = %q", settings.SupportWhatsApp)
	}
	if settings.ReservationHoldHours != models.DefaultReservationHoldHours {
		t.Errorf("hold = %d, want %d", settings.ReservationHoldHours, models.DefaultReservationHoldHours)
	}
}
