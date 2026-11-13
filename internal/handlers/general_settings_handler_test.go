package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
	"gorm.io/gorm"
)

type stubGeneralSettingsRepo struct {
	row *models.GeneralSettings
}

func (r *stubGeneralSettingsRepo) Get(ctx context.Context) (*models.GeneralSettings, error) {
	if r.row == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.row, nil
}

func (r *stubGeneralSettingsRepo) Upsert(ctx context.Context, settings *models.GeneralSettings) error {
	settings.ID = 1
	stored := *settings
	r.row = &stored
	return nil
}

func generalSettingsTestRouter(svc *services.GeneralSettingsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewGeneralSettingsHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Set("userRole", "admin")
		c.Next()
	})
	r.GET("/api/v1/settings", handler.Show)
	r.PUT("/api/v1/settings", handler.Update)
	return r
}

func decodeSettingsResponse(t *testing.T, rec *httptest.ResponseRecorder) models.GeneralSettingsResponse {
	t.Helper()
	var body struct {
		Settings models.GeneralSettingsResponse `json:"settings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return body.Settings
}

func TestGeneralSettingsShowExposesLegalFields(t *testing.T) {
	r := generalSettingsTestRouter(services.NewGeneralSettingsService(&stubGeneralSettingsRepo{}, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	settings := decodeSettingsResponse(t, rec)
	if settings.LegalName != "" || settings.Website != "" || settings.TermsContentES != "" {
		t.Errorf("expected empty legal defaults, got %+v", settings)
	}
	if settings.TermsEffectiveDate != "" {
		t.Errorf("expected empty effective date, got %q", settings.TermsEffectiveDate)
	}
	if settings.ReservationHoldHours != models.DefaultReservationHoldHours {
		t.Errorf("expected default hold hours, got %d", settings.ReservationHoldHours)
	}
}

func TestGeneralSettingsUpdatePersistsLegalFields(t *testing.T) {
	repo := &stubGeneralSettingsRepo{}
	r := generalSettingsTestRouter(services.NewGeneralSettingsService(repo, nil))

	payload := `{"settings": {
		"support_whatsapp": "+50499998888",
		"support_email": "soporte@fintera.test",
		"reservation_hold_hours": 48,
		"legal_name": "  Omegatech S. de R.L.  ",
		"legal_tax_id": " 08019995123456 ",
		"legal_address": " Tegucigalpa, Honduras ",
		"website": " https://omegatech.dev/ ",
		"terms_effective_date": "2026-02-01",
		"terms_content_es": " Términos y condiciones en español ",
		"terms_content_en": " Terms and conditions in English "
	}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	settings := decodeSettingsResponse(t, rec)
	if settings.LegalName != "Omegatech S. de R.L." {
		t.Errorf("legal name = %q", settings.LegalName)
	}
	if settings.LegalTaxID != "08019995123456" {
		t.Errorf("legal tax id = %q", settings.LegalTaxID)
	}
	if settings.LegalAddress != "Tegucigalpa, Honduras" {
		t.Errorf("legal address = %q", settings.LegalAddress)
	}
	if settings.Website != "https://omegatech.dev/" {
		t.Errorf("website = %q", settings.Website)
	}
	if settings.TermsEffectiveDate != "2026-02-01" {
		t.Errorf("terms effective date = %q", settings.TermsEffectiveDate)
	}
	if settings.TermsContentES != "Términos y condiciones en español" {
		t.Errorf("terms es = %q", settings.TermsContentES)
	}
	if settings.TermsContentEN != "Terms and conditions in English" {
		t.Errorf("terms en = %q", settings.TermsContentEN)
	}

	if repo.row == nil {
		t.Fatal("expected settings to be persisted")
	}
	if repo.row.LegalName != "Omegatech S. de R.L." || repo.row.TermsContentES == nil {
		t.Errorf("unexpected persisted row: %+v", repo.row)
	}
	if repo.row.TermsEffectiveDate == nil || repo.row.TermsEffectiveDate.Format(models.TermsDateLayout) != "2026-02-01" {
		t.Errorf("unexpected persisted date: %v", repo.row.TermsEffectiveDate)
	}
}

func TestGeneralSettingsUpdateClearsOptionalFields(t *testing.T) {
	address := "Tegucigalpa"
	terms := "Contenido"
	repo := &stubGeneralSettingsRepo{row: &models.GeneralSettings{
		ID:                   1,
		LegalAddress:         &address,
		TermsContentES:       &terms,
		ReservationHoldHours: 48,
	}}
	r := generalSettingsTestRouter(services.NewGeneralSettingsService(repo, nil))

	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(`{
		"settings": {"legal_address": "  ", "terms_content_es": "", "reservation_hold_hours": 48}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	settings := decodeSettingsResponse(t, rec)
	if settings.LegalAddress != "" || settings.TermsContentES != "" {
		t.Errorf("expected cleared optional fields, got %+v", settings)
	}
	if repo.row.LegalAddress != nil || repo.row.TermsContentES != nil {
		t.Errorf("expected NULL persisted values, got %+v", repo.row)
	}
}

func TestGeneralSettingsUpdateRejectsInvalidFields(t *testing.T) {
	longTerms := strings.Repeat("x", 20001)
	cases := []struct {
		name    string
		payload string
	}{
		{"invalid website", `{"settings": {"website": "omegatech.dev", "reservation_hold_hours": 48}}`},
		{"ftp website", `{"settings": {"website": "ftp://omegatech.dev", "reservation_hold_hours": 48}}`},
		{"invalid date", `{"settings": {"terms_effective_date": "01/02/2026", "reservation_hold_hours": 48}}`},
		{"legal name too long", `{"settings": {"legal_name": "` + strings.Repeat("a", 161) + `", "reservation_hold_hours": 48}}`},
		{"terms too long", `{"settings": {"terms_content_es": "` + longTerms + `", "reservation_hold_hours": 48}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubGeneralSettingsRepo{}
			r := generalSettingsTestRouter(services.NewGeneralSettingsService(repo, nil))

			req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if repo.row != nil {
				t.Error("expected no persistence for invalid payload")
			}
		})
	}
}
