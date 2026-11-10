package services

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"os"
	"strings"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

type fakeBrandingRepo struct {
	row *models.Branding
}

func (r *fakeBrandingRepo) Get(ctx context.Context) (*models.Branding, error) {
	if r.row == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.row, nil
}

func (r *fakeBrandingRepo) Upsert(ctx context.Context, branding *models.Branding) error {
	branding.ID = 1
	r.row = branding
	return nil
}

type fakeBrandingStorage struct {
	files map[string][]byte
}

func newFakeBrandingStorage() *fakeBrandingStorage {
	return &fakeBrandingStorage{files: map[string][]byte{}}
}

func (s *fakeBrandingStorage) Upload(file multipart.File, header *multipart.FileHeader, subDir string) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	relPath := subDir + "/2026/01/" + header.Filename
	s.files[relPath] = data
	return relPath, nil
}

func (s *fakeBrandingStorage) Delete(relPath string) error {
	delete(s.files, relPath)
	return nil
}

func newBrandingTestService(repo *fakeBrandingRepo, store *fakeBrandingStorage) *BrandingService {
	return NewBrandingService(repo, store, nil)
}

func brandingTestFile(t *testing.T, content []byte, filename string) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "logo-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("failed to stat temp file: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("failed to rewind temp file: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f, &multipart.FileHeader{Filename: filename, Size: info.Size()}
}

var pngBytes = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}

func TestBrandingGetReturnsDefaultsWhenMissing(t *testing.T) {
	svc := newBrandingTestService(&fakeBrandingRepo{}, newFakeBrandingStorage())

	branding, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if branding.AppName != models.DefaultAppName {
		t.Errorf("expected default app name %q, got %q", models.DefaultAppName, branding.AppName)
	}
	if branding.LogoPath != nil || branding.LogoDarkPath != nil {
		t.Errorf("expected nil logo paths, got %v/%v", branding.LogoPath, branding.LogoDarkPath)
	}
}

func TestBrandingGetReturnsStoredRow(t *testing.T) {
	repo := &fakeBrandingRepo{row: &models.Branding{AppName: "Acme", LogoPath: strPtr("uploads/branding/a.png")}}
	svc := newBrandingTestService(repo, newFakeBrandingStorage())

	branding, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if branding.AppName != "Acme" {
		t.Errorf("expected Acme, got %q", branding.AppName)
	}
	resp := branding.ToResponse()
	if resp.LogoURL == nil || *resp.LogoURL != "/uploads/branding/a.png" {
		t.Errorf("expected public logo URL, got %v", resp.LogoURL)
	}
}

func TestBrandingUpdateNameValidation(t *testing.T) {
	repo := &fakeBrandingRepo{}
	store := newFakeBrandingStorage()
	svc := newBrandingTestService(repo, store)
	ctx := context.Background()

	if _, err := svc.UpdateName(ctx, "   ", 1); err == nil {
		t.Error("expected error for empty name")
	}
	if _, err := svc.UpdateName(ctx, strings.Repeat("a", 61), 1); err == nil {
		t.Error("expected error for name longer than 60 characters")
	}

	branding, err := svc.UpdateName(ctx, "  Acme  ", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if branding.AppName != "Acme" {
		t.Errorf("expected trimmed name Acme, got %q", branding.AppName)
	}
	if repo.row == nil || repo.row.AppName != "Acme" {
		t.Error("expected persisted row")
	}
}

func TestBrandingUpdateLogoReplacesFile(t *testing.T) {
	repo := &fakeBrandingRepo{}
	store := newFakeBrandingStorage()
	svc := newBrandingTestService(repo, store)
	ctx := context.Background()

	file, header := brandingTestFile(t, pngBytes, "one.png")
	branding, err := svc.UpdateLogo(ctx, models.BrandingLogoVariantLight, file, header, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if branding.LogoPath == nil {
		t.Fatal("expected light logo path to be set")
	}
	first := *branding.LogoPath
	if _, ok := store.files[first]; !ok {
		t.Errorf("expected stored file %s", first)
	}

	// Replace: the previous file must be deleted
	file, header = brandingTestFile(t, pngBytes, "two.png")
	branding, err = svc.UpdateLogo(ctx, models.BrandingLogoVariantLight, file, header, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if *branding.LogoPath == first {
		t.Error("expected a new path after replace")
	}
	if _, ok := store.files[first]; ok {
		t.Error("expected previous file to be deleted")
	}
}

func TestBrandingUpdateLogoDarkKeepsLight(t *testing.T) {
	repo := &fakeBrandingRepo{}
	store := newFakeBrandingStorage()
	svc := newBrandingTestService(repo, store)
	ctx := context.Background()

	file, header := brandingTestFile(t, pngBytes, "light.png")
	light, err := svc.UpdateLogo(ctx, models.BrandingLogoVariantLight, file, header, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	lightPath := *light.LogoPath

	file, header = brandingTestFile(t, pngBytes, "dark.png")
	dark, err := svc.UpdateLogo(ctx, models.BrandingLogoVariantDark, file, header, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dark.LogoDarkPath == nil {
		t.Fatal("expected dark logo path to be set")
	}
	if *dark.LogoPath != lightPath {
		t.Error("expected light logo to remain unchanged")
	}
}

func TestBrandingUpdateLogoRejections(t *testing.T) {
	repo := &fakeBrandingRepo{}
	store := newFakeBrandingStorage()
	svc := newBrandingTestService(repo, store)
	ctx := context.Background()

	if _, err := svc.UpdateLogo(ctx, "bogus", nil, nil, 1); err != ErrInvalidBrandingOption {
		t.Errorf("expected ErrInvalidBrandingOption, got %v", err)
	}

	file, header := brandingTestFile(t, []byte("just text"), "notes.txt")
	if _, err := svc.UpdateLogo(ctx, models.BrandingLogoVariantLight, file, header, 1); err != ErrInvalidBrandingLogo {
		t.Errorf("expected ErrInvalidBrandingLogo, got %v", err)
	}

	big := bytes.Repeat(pngBytes, 300000) // ~2.4 MB
	file, header = brandingTestFile(t, big, "big.png")
	if _, err := svc.UpdateLogo(ctx, models.BrandingLogoVariantLight, file, header, 1); err != ErrBrandingLogoTooLarge {
		t.Errorf("expected ErrBrandingLogoTooLarge, got %v", err)
	}

	if len(store.files) != 0 {
		t.Errorf("expected no stored files after rejections, got %v", store.files)
	}
	if repo.row != nil && (repo.row.LogoPath != nil || repo.row.LogoDarkPath != nil) {
		t.Error("expected branding paths to remain unset after rejections")
	}
}

func TestBrandingRemoveLogo(t *testing.T) {
	repo := &fakeBrandingRepo{}
	store := newFakeBrandingStorage()
	svc := newBrandingTestService(repo, store)
	ctx := context.Background()

	file, header := brandingTestFile(t, pngBytes, "light.png")
	branding, err := svc.UpdateLogo(ctx, models.BrandingLogoVariantLight, file, header, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	path := *branding.LogoPath

	branding, err = svc.RemoveLogo(ctx, models.BrandingLogoVariantLight, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if branding.LogoPath != nil {
		t.Error("expected light logo path to be cleared")
	}
	if _, ok := store.files[path]; ok {
		t.Error("expected file to be deleted")
	}

	// Removing an unset logo is a no-op success
	if _, err := svc.RemoveLogo(ctx, models.BrandingLogoVariantLight, 1); err != nil {
		t.Errorf("expected no error removing unset logo, got %v", err)
	}
}

func strPtr(s string) *string {
	return &s
}
