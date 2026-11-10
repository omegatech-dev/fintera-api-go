package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
	"gorm.io/gorm"
)

type stubBrandingRepo struct {
	row *models.Branding
}

func (r *stubBrandingRepo) Get(ctx context.Context) (*models.Branding, error) {
	if r.row == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.row, nil
}

func (r *stubBrandingRepo) Upsert(ctx context.Context, branding *models.Branding) error {
	branding.ID = 1
	r.row = branding
	return nil
}

type stubBrandingStorage struct {
	uploaded int
}

func (s *stubBrandingStorage) Upload(file multipart.File, header *multipart.FileHeader, subDir string) (string, error) {
	s.uploaded++
	return subDir + "/2026/01/test.png", nil
}

func (s *stubBrandingStorage) Delete(relPath string) error { return nil }

func brandingTestService(repo *stubBrandingRepo, store *stubBrandingStorage) *services.BrandingService {
	return services.NewBrandingService(repo, store, nil)
}

// brandingTestRouter mirrors the real router: the Show route is public and the
// write routes sit behind RequireAdmin with the acting role injected.
func brandingTestRouter(role string, svc *services.BrandingService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewBrandingHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Set("userRole", role)
		c.Next()
	})
	r.GET("/api/v1/branding", handler.Show)
	write := r.Group("/api/v1")
	write.Use(middleware.RequireAdmin())
	{
		write.PUT("/branding", handler.Update)
		write.POST("/branding/logo", handler.UploadLogo)
		write.DELETE("/branding/logo", handler.DeleteLogo)
	}
	return r
}

func TestBrandingShowIsPublic(t *testing.T) {
	r := brandingTestRouter("user", brandingTestService(&stubBrandingRepo{}, &stubBrandingStorage{}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/branding", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Branding models.BrandingResponse `json:"branding"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if body.Branding.AppName != models.DefaultAppName {
		t.Errorf("expected default app name, got %q", body.Branding.AppName)
	}
	if body.Branding.LogoURL != nil {
		t.Error("expected nil logo_url when nothing is configured")
	}
}

func TestBrandingUpdateRequiresAdmin(t *testing.T) {
	repo := &stubBrandingRepo{}
	svc := brandingTestService(repo, &stubBrandingStorage{})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/branding", strings.NewReader(`{"branding": {"app_name": "Acme"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	brandingTestRouter("seller", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", rec.Code)
	}
	if repo.row != nil {
		t.Error("expected no changes for non-admin")
	}
}

func TestBrandingUpdateValidatesName(t *testing.T) {
	svc := brandingTestService(&stubBrandingRepo{}, &stubBrandingStorage{})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/branding", strings.NewReader(`{"branding": {"app_name": "   "}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	brandingTestRouter("admin", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d", rec.Code)
	}
}

func TestBrandingUpdateSucceedsForAdmin(t *testing.T) {
	repo := &stubBrandingRepo{}
	svc := brandingTestService(repo, &stubBrandingStorage{})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/branding", strings.NewReader(`{"branding": {"app_name": "Acme"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	brandingTestRouter("admin", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.row == nil || repo.row.AppName != "Acme" {
		t.Error("expected persisted name")
	}
}

func TestBrandingUploadLogo(t *testing.T) {
	store := &stubBrandingStorage{}
	svc := brandingTestService(&stubBrandingRepo{}, store)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("variant", "dark")
	part, _ := writer.CreateFormFile("logo", "logo.png")
	_, _ = part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/branding/logo", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	brandingTestRouter("admin", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.uploaded != 1 {
		t.Errorf("expected one upload, got %d", store.uploaded)
	}

	var resp struct {
		Branding models.BrandingResponse `json:"branding"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Branding.LogoDarkURL == nil {
		t.Error("expected dark logo URL in response")
	}
}

func TestBrandingUploadLogoRejectsDisallowedContent(t *testing.T) {
	store := &stubBrandingStorage{}
	svc := brandingTestService(&stubBrandingRepo{}, store)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("logo", "notes.txt")
	_, _ = part.Write([]byte("plain text, not an image"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/branding/logo", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	brandingTestRouter("admin", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if store.uploaded != 0 {
		t.Error("expected no uploads for disallowed content")
	}
}

func TestBrandingDeleteLogo(t *testing.T) {
	repo := &stubBrandingRepo{row: &models.Branding{AppName: "Acme", LogoPath: testStrPtr("uploads/branding/old.png")}}
	svc := brandingTestService(repo, &stubBrandingStorage{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/branding/logo?variant=light", nil)
	rec := httptest.NewRecorder()
	brandingTestRouter("admin", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Branding models.BrandingResponse `json:"branding"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Branding.LogoURL != nil {
		t.Errorf("expected null logo_url after delete, got %v", resp.Branding.LogoURL)
	}
}

func testStrPtr(s string) *string {
	return &s
}
