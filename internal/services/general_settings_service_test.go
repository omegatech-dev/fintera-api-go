package services

import (
	"errors"
	"strings"
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

func TestApplySettingsDefaultsTrimsLegalFields(t *testing.T) {
	address := "  Tegucigalpa, Honduras  "
	termsES := "  Términos en español  "
	termsEN := "  Terms in English  "
	settings := &models.GeneralSettings{
		LegalName:            "  Omegatech S. de R.L.  ",
		LegalTaxID:           "  08019995123456  ",
		LegalAddress:         &address,
		Website:              "  https://omegatech.dev/  ",
		TermsContentES:       &termsES,
		TermsContentEN:       &termsEN,
		ReservationHoldHours: 48,
	}
	applySettingsDefaults(settings)

	if settings.LegalName != "Omegatech S. de R.L." {
		t.Errorf("legal name = %q", settings.LegalName)
	}
	if settings.LegalTaxID != "08019995123456" {
		t.Errorf("tax id = %q", settings.LegalTaxID)
	}
	if settings.LegalAddress == nil || *settings.LegalAddress != "Tegucigalpa, Honduras" {
		t.Errorf("legal address = %v", settings.LegalAddress)
	}
	if settings.Website != "https://omegatech.dev/" {
		t.Errorf("website = %q", settings.Website)
	}
	if settings.TermsContentES == nil || *settings.TermsContentES != "Términos en español" {
		t.Errorf("terms es = %v", settings.TermsContentES)
	}
	if settings.TermsContentEN == nil || *settings.TermsContentEN != "Terms in English" {
		t.Errorf("terms en = %v", settings.TermsContentEN)
	}
}

func TestValidateGeneralSettingsLegalFields(t *testing.T) {
	longName := strings.Repeat("a", 161)
	longTaxID := strings.Repeat("b", 61)
	longAddress := strings.Repeat("c", 501)
	longWebsite := "https://example.test/" + strings.Repeat("d", 181)
	longContent := strings.Repeat("e", 20001)
	address := "Tegucigalpa"
	content := "Términos"

	valid := models.GeneralSettings{
		LegalName:            "Omegatech S. de R.L.",
		LegalTaxID:           "08019995123456",
		LegalAddress:         &address,
		Website:              "https://omegatech.dev/",
		TermsContentES:       &content,
		TermsContentEN:       &content,
		ReservationHoldHours: 48,
	}
	if err := validateGeneralSettings(&valid); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cases := []struct {
		name     string
		settings models.GeneralSettings
		want     error
	}{
		{"name too long", models.GeneralSettings{LegalName: longName, ReservationHoldHours: 48}, ErrLegalNameTooLong},
		{"tax id too long", models.GeneralSettings{LegalTaxID: longTaxID, ReservationHoldHours: 48}, ErrLegalTaxIDTooLong},
		{"address too long", models.GeneralSettings{LegalAddress: &longAddress, ReservationHoldHours: 48}, ErrLegalAddressTooLong},
		{"website too long", models.GeneralSettings{Website: longWebsite, ReservationHoldHours: 48}, ErrWebsiteTooLong},
		{"website without scheme", models.GeneralSettings{Website: "omegatech.dev", ReservationHoldHours: 48}, ErrInvalidWebsite},
		{"website with ftp scheme", models.GeneralSettings{Website: "ftp://omegatech.dev", ReservationHoldHours: 48}, ErrInvalidWebsite},
		{"terms too long", models.GeneralSettings{TermsContentES: &longContent, ReservationHoldHours: 48}, ErrTermsContentTooLong},
		{"english terms too long", models.GeneralSettings{TermsContentEN: &longContent, ReservationHoldHours: 48}, ErrTermsContentTooLong},
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

func TestParseTermsEffectiveDate(t *testing.T) {
	date, err := ParseTermsEffectiveDate("")
	if err != nil || date != nil {
		t.Fatalf("empty date = %v, err = %v", date, err)
	}

	date, err = ParseTermsEffectiveDate(" 2026-02-01 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if date == nil || date.Format(models.TermsDateLayout) != "2026-02-01" {
		t.Errorf("date = %v", date)
	}

	if _, err := ParseTermsEffectiveDate("01/02/2026"); !errors.Is(err, ErrInvalidTermsDate) {
		t.Errorf("error = %v, want %v", err, ErrInvalidTermsDate)
	}
	if _, err := ParseTermsEffectiveDate("2026-13-01"); !errors.Is(err, ErrInvalidTermsDate) {
		t.Errorf("error = %v, want %v", err, ErrInvalidTermsDate)
	}
}

func TestGeneralSettingsResponseFormatsDateAndNils(t *testing.T) {
	date, _ := ParseTermsEffectiveDate("2026-02-01")
	address := "Tegucigalpa"
	terms := "Contenido"
	response := (&models.GeneralSettings{
		LegalName:            "Omegatech",
		LegalTaxID:           "0801",
		LegalAddress:         &address,
		Website:              "https://omegatech.dev/",
		TermsEffectiveDate:   date,
		TermsContentES:       &terms,
		ReservationHoldHours: 48,
	}).ToResponse()

	if response.TermsEffectiveDate != "2026-02-01" {
		t.Errorf("terms date = %q", response.TermsEffectiveDate)
	}
	if response.LegalAddress != "Tegucigalpa" || response.TermsContentES != "Contenido" {
		t.Errorf("unexpected legal values: %+v", response)
	}
	if response.TermsContentEN != "" {
		t.Errorf("expected empty english terms, got %q", response.TermsContentEN)
	}
}
