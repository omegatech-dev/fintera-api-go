package models

import (
	"strings"
	"time"
)

// DefaultAppName is the application name used when no branding is configured
const DefaultAppName = "Fintera"

// Branding represents the global, admin-managed application branding.
// A single row (id = 1) is kept per deployment.
type Branding struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AppName      string    `gorm:"column:app_name;not null;default:'Fintera'" json:"app_name"`
	LogoPath     *string   `gorm:"column:logo_path" json:"-"`
	LogoDarkPath *string   `gorm:"column:logo_dark_path" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName specifies the table name for Branding
func (Branding) TableName() string {
	return "branding_settings"
}

// Logo variants accepted by the upload endpoint
const (
	BrandingLogoVariantLight = "light"
	BrandingLogoVariantDark  = "dark"
)

// IsValidBrandingLogoVariant reports whether the variant is known
func IsValidBrandingLogoVariant(variant string) bool {
	return variant == BrandingLogoVariantLight || variant == BrandingLogoVariantDark
}

// BrandingResponse is the JSON response format for branding
type BrandingResponse struct {
	AppName     string    `json:"app_name"`
	LogoURL     *string   `json:"logo_url"`
	LogoDarkURL *string   `json:"logo_dark_url"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ToResponse converts Branding to BrandingResponse, mapping stored relative
// paths (uploads/...) to publicly served URLs (/uploads/...)
func (b *Branding) ToResponse() BrandingResponse {
	return BrandingResponse{
		AppName:     b.AppName,
		LogoURL:     publicUploadURL(b.LogoPath),
		LogoDarkURL: publicUploadURL(b.LogoDarkPath),
		UpdatedAt:   b.UpdatedAt,
	}
}

func publicUploadURL(path *string) *string {
	if path == nil || *path == "" {
		return nil
	}
	url := "/" + strings.TrimPrefix(*path, "/")
	return &url
}
