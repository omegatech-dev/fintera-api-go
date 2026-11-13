package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

type stubGeneralSettingsRepository struct {
	row *models.GeneralSettings
}

func (r *stubGeneralSettingsRepository) Get(ctx context.Context) (*models.GeneralSettings, error) {
	if r.row == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.row, nil
}

func (r *stubGeneralSettingsRepository) Upsert(ctx context.Context, settings *models.GeneralSettings) error {
	settings.ID = 1
	stored := *settings
	r.row = &stored
	return nil
}

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

func TestSanitizeTermsContentKeepsAllowedFormatting(t *testing.T) {
	input := `<h2>Título</h2><p><strong>Negrita</strong> y <em>cursiva</em> con ` +
		`<a href="https://fintera.test/terminos">enlace</a>, <u>subrayado</u> y <s>tachado</s>.</p>` +
		`<ul><li>Uno</li><li>Dos</li></ul><blockquote>Cita</blockquote>`
	clean := sanitizeTermsContent(&input)
	if clean == nil {
		t.Fatal("expected sanitized content")
	}
	for _, expected := range []string{"<h2>Título</h2>", "<strong>Negrita</strong>", "<em>cursiva</em>", "<u>subrayado</u>", "<s>tachado</s>", "<ul>", "<blockquote>"} {
		if !strings.Contains(*clean, expected) {
			t.Errorf("sanitized content missing %q: %s", expected, *clean)
		}
	}
	if !strings.Contains(*clean, `href="https://fintera.test/terminos"`) {
		t.Errorf("allowed link removed: %s", *clean)
	}
}

func TestSanitizeTermsContentDropsScriptsAndEventHandlers(t *testing.T) {
	input := `<p onclick="steal()">Texto</p><script>alert("xss")</script>` +
		`<img src="x" onerror="alert(1)">` +
		`<a href="javascript:alert(1)">malicioso</a>`
	clean := sanitizeTermsContent(&input)
	if clean == nil {
		t.Fatal("expected sanitized content")
	}
	lower := strings.ToLower(*clean)
	for _, forbidden := range []string{"script", "onclick", "onerror", "javascript:", "<img"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("sanitized content still contains %q: %s", forbidden, *clean)
		}
	}
	if !strings.Contains(*clean, "Texto") || !strings.Contains(*clean, "malicioso") {
		t.Errorf("visible text was removed: %s", *clean)
	}
}

func TestSanitizeTermsContentNormalizesEmptyMarkup(t *testing.T) {
	empty := "<p><br></p>"
	if clean := sanitizeTermsContent(&empty); clean != nil {
		t.Errorf("empty markup = %v, want nil", *clean)
	}
	if clean := sanitizeTermsContent(nil); clean != nil {
		t.Errorf("nil = %v, want nil", clean)
	}
}

func TestValidateGeneralSettingsCountsVisibleTermsText(t *testing.T) {
	visible := strings.Repeat("a", 20000)
	withMarkup := "<p><strong>" + visible + "</strong></p>"
	if len(withMarkup) <= 20000 {
		t.Fatal("expected markup to exceed the raw HTML limit")
	}
	if err := validateGeneralSettings(&models.GeneralSettings{TermsContentES: &withMarkup, ReservationHoldHours: 48}); err != nil {
		t.Fatalf("visible text at limit should be valid: %v", err)
	}

	tooLong := "<p>" + strings.Repeat("a", 20001) + "</p>"
	if err := validateGeneralSettings(&models.GeneralSettings{TermsContentES: &tooLong, ReservationHoldHours: 48}); !errors.Is(err, ErrTermsContentTooLong) {
		t.Errorf("error = %v, want %v", err, ErrTermsContentTooLong)
	}
}

func TestUpdateSanitizesTermsBeforePersisting(t *testing.T) {
	repo := &stubGeneralSettingsRepository{}
	service := NewGeneralSettingsService(repo, nil)
	unsafe := `<h2>Términos</h2><script>alert(1)</script><p onclick="x()"><strong>Seguro</strong></p>`
	settings, err := service.Update(context.Background(), 1, models.GeneralSettings{
		ReservationHoldHours: 48,
		TermsContentES:       &unsafe,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.TermsContentES == nil || strings.Contains(*settings.TermsContentES, "script") || strings.Contains(*settings.TermsContentES, "onclick") {
		t.Errorf("unsafe content returned: %v", settings.TermsContentES)
	}
	if repo.row == nil || repo.row.TermsContentES == nil || !strings.Contains(*repo.row.TermsContentES, "<strong>Seguro</strong>") {
		t.Errorf("unexpected persisted row: %+v", repo.row)
	}
}

func TestSanitizeTermsContentPreservesLegacyPlainText(t *testing.T) {
	legacy := "Primer párrafo con\rsalto.\n\nSegundo párrafo con 3 < 5."
	clean := sanitizeTermsContent(&legacy)
	if clean == nil {
		t.Fatal("expected legacy content to survive sanitization")
	}
	if !strings.Contains(*clean, "Primer párrafo") || !strings.Contains(*clean, "Segundo párrafo") {
		t.Errorf("legacy text lost: %q", *clean)
	}
	if !strings.Contains(*clean, "\n\n") {
		t.Errorf("paragraph breaks lost: %q", *clean)
	}
}
