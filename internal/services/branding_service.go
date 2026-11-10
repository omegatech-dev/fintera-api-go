package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"gorm.io/gorm"
)

// Branding logo upload constraints
const (
	brandingLogoSubDir = "uploads/branding"
	// MaxBrandingLogoBytes is the maximum accepted logo upload size (2 MB)
	MaxBrandingLogoBytes = int64(2 << 20)
	brandingLogoMaxBytes = MaxBrandingLogoBytes
)

// Allowed content types for branding logos (raster images only; SVG is
// rejected to avoid serving script-bearing files from the API origin)
var brandingLogoContentTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
}

// Branding errors
var (
	ErrInvalidBrandingLogo   = errors.New("formato de logo inválido (solo PNG, JPEG o WebP)")
	ErrBrandingLogoTooLarge  = errors.New("el logo excede el tamaño máximo de 2 MB")
	ErrBrandingLogoRequired  = errors.New("no se ha subido ningún logo")
	ErrInvalidBrandingOption = errors.New("variante de logo inválida")
)

// BrandingStorage is the subset of file storage the branding service needs
type BrandingStorage interface {
	Upload(file multipart.File, header *multipart.FileHeader, subDir string) (string, error)
	Delete(relativePath string) error
}

// BrandingService manages the global application branding
type BrandingService struct {
	repo  BrandingRepository
	store BrandingStorage
	audit *AuditService
}

// BrandingRepository is the subset of persistence the branding service needs
type BrandingRepository interface {
	Get(ctx context.Context) (*models.Branding, error)
	Upsert(ctx context.Context, branding *models.Branding) error
}

// NewBrandingService creates a new branding service
func NewBrandingService(repo BrandingRepository, store BrandingStorage, audit *AuditService) *BrandingService {
	return &BrandingService{repo: repo, store: store, audit: audit}
}

// Get returns the current branding, falling back to defaults when the
// singleton row does not exist or has an empty name
func (s *BrandingService) Get(ctx context.Context) (*models.Branding, error) {
	branding, err := s.repo.Get(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.Branding{ID: 1, AppName: models.DefaultAppName}, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(branding.AppName) == "" {
		branding.AppName = models.DefaultAppName
	}
	return branding, nil
}

// UpdateName validates and persists the application name
func (s *BrandingService) UpdateName(ctx context.Context, name string, actorID uint) (*models.Branding, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("el nombre de la aplicación es obligatorio")
	}
	if len([]rune(trimmed)) > 60 {
		return nil, fmt.Errorf("el nombre de la aplicación no puede exceder 60 caracteres")
	}

	branding, err := s.getOrCreate(ctx)
	if err != nil {
		return nil, err
	}

	previous := branding.AppName
	branding.AppName = trimmed

	if err := s.repo.Upsert(ctx, branding); err != nil {
		return nil, err
	}

	s.logChange(ctx, actorID, fmt.Sprintf("Nombre actualizado de %q a %q", previous, trimmed))
	return branding, nil
}

// UpdateLogo stores a new logo file for the given variant
func (s *BrandingService) UpdateLogo(ctx context.Context, variant string, file multipart.File, header *multipart.FileHeader, actorID uint) (*models.Branding, error) {
	if !models.IsValidBrandingLogoVariant(variant) {
		return nil, ErrInvalidBrandingOption
	}
	if file == nil || header == nil {
		return nil, ErrBrandingLogoRequired
	}
	if header.Size > brandingLogoMaxBytes {
		return nil, ErrBrandingLogoTooLarge
	}
	if err := validateLogoContent(file); err != nil {
		return nil, err
	}

	relPath, err := s.store.Upload(file, header, brandingLogoSubDir)
	if err != nil {
		return nil, fmt.Errorf("error al guardar el logo")
	}

	branding, err := s.getOrCreate(ctx)
	if err != nil {
		_ = s.store.Delete(relPath)
		return nil, err
	}

	var previous *string
	if variant == models.BrandingLogoVariantDark {
		previous = branding.LogoDarkPath
		branding.LogoDarkPath = &relPath
	} else {
		previous = branding.LogoPath
		branding.LogoPath = &relPath
	}

	if err := s.repo.Upsert(ctx, branding); err != nil {
		_ = s.store.Delete(relPath)
		return nil, err
	}

	if previous != nil {
		_ = s.store.Delete(*previous)
	}

	s.logChange(ctx, actorID, fmt.Sprintf("Logo %s actualizado", variant))
	return branding, nil
}

// RemoveLogo clears the stored logo for the given variant and deletes the file
func (s *BrandingService) RemoveLogo(ctx context.Context, variant string, actorID uint) (*models.Branding, error) {
	if !models.IsValidBrandingLogoVariant(variant) {
		return nil, ErrInvalidBrandingOption
	}

	branding, err := s.getOrCreate(ctx)
	if err != nil {
		return nil, err
	}

	var previous *string
	if variant == models.BrandingLogoVariantDark {
		previous = branding.LogoDarkPath
		branding.LogoDarkPath = nil
	} else {
		previous = branding.LogoPath
		branding.LogoPath = nil
	}

	if err := s.repo.Upsert(ctx, branding); err != nil {
		return nil, err
	}

	if previous != nil {
		_ = s.store.Delete(*previous)
	}

	s.logChange(ctx, actorID, fmt.Sprintf("Logo %s eliminado", variant))
	return branding, nil
}

func (s *BrandingService) getOrCreate(ctx context.Context) (*models.Branding, error) {
	branding, err := s.repo.Get(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.Branding{ID: 1, AppName: models.DefaultAppName}, nil
	}
	if err != nil {
		return nil, err
	}
	return branding, nil
}

func (s *BrandingService) logChange(ctx context.Context, actorID uint, details string) {
	if s.audit == nil {
		return
	}
	if err := s.audit.Log(ctx, actorID, "UPDATE_BRANDING", "Branding", 1, details, "", ""); err != nil {
		logger.Warn("failed to audit branding change", "error", err)
	}
}

// validateLogoContent sniffs the first bytes of the file so a spoofed
// Content-Type cannot smuggle disallowed content
func validateLogoContent(file multipart.File) error {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return ErrInvalidBrandingLogo
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ErrInvalidBrandingLogo
	}
	detected := http.DetectContentType(buf[:n])
	if !brandingLogoContentTypes[detected] {
		return ErrInvalidBrandingLogo
	}
	return nil
}
