package models

import (
	"path/filepath"
	"strings"
	"time"
)

// Drawing file types.
const (
	DrawingTypeDWG = "dwg"
	DrawingTypeDXF = "dxf"
	DrawingTypePDF = "pdf"
	DrawingTypeSVG = "svg"
)

// ProjectDrawing is a CAD/document plan published for a project.
type ProjectDrawing struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProjectID  uint      `gorm:"not null;index" json:"project_id"`
	Name       string    `gorm:"not null" json:"name"`
	FilePath   string    `gorm:"column:file_path;not null" json:"file_path"`
	FileType   string    `gorm:"column:file_type;not null" json:"file_type"`
	Version    *string   `json:"version"`
	SizeBytes  int64     `gorm:"column:size_bytes;not null" json:"size_bytes"`
	Note       *string   `json:"note"`
	UploadedBy *uint     `gorm:"column:uploaded_by" json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName specifies the table name for ProjectDrawing.
func (ProjectDrawing) TableName() string {
	return "project_drawings"
}

// ProjectDrawingResponse is the public drawing payload. It deliberately omits
// UploadedBy.
type ProjectDrawingResponse struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	FileType    string  `json:"file_type"`
	Version     *string `json:"version"`
	Note        *string `json:"note"`
	SizeBytes   int64   `json:"size_bytes"`
	DownloadURL string  `json:"download_url"`
}

// ToResponse converts a drawing to its public payload.
func (d *ProjectDrawing) ToResponse() ProjectDrawingResponse {
	return ProjectDrawingResponse{
		ID:          d.ID,
		Name:        d.Name,
		FileType:    d.FileType,
		Version:     d.Version,
		Note:        d.Note,
		SizeBytes:   d.SizeBytes,
		DownloadURL: PublicFileURL(d.FilePath),
	}
}

// ProjectPhoto is a photo in the project gallery.
type ProjectPhoto struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProjectID  uint      `gorm:"not null;index" json:"project_id"`
	FilePath   string    `gorm:"column:file_path;not null" json:"file_path"`
	ThumbPath  *string   `gorm:"column:thumb_path" json:"thumb_path"`
	Position   int       `gorm:"not null;default:0" json:"position"`
	Alt        *string   `json:"alt"`
	UploadedBy *uint     `gorm:"column:uploaded_by" json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName specifies the table name for ProjectPhoto.
func (ProjectPhoto) TableName() string {
	return "project_photos"
}

// LotPhoto is a photo in a lot's gallery.
type LotPhoto struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	LotID      uint      `gorm:"not null;index" json:"lot_id"`
	FilePath   string    `gorm:"column:file_path;not null" json:"file_path"`
	ThumbPath  *string   `gorm:"column:thumb_path" json:"thumb_path"`
	Position   int       `gorm:"not null;default:0" json:"position"`
	Alt        *string   `json:"alt"`
	UploadedBy *uint     `gorm:"column:uploaded_by" json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName specifies the table name for LotPhoto.
func (LotPhoto) TableName() string {
	return "lot_photos"
}

// PhotoResponse is the public photo payload shared by project and lot
// galleries.
type PhotoResponse struct {
	URL      string  `json:"url"`
	ThumbURL *string `json:"thumb_url,omitempty"`
	Alt      *string `json:"alt,omitempty"`
}

// ToResponse converts a project photo to its public payload.
func (p *ProjectPhoto) ToResponse() PhotoResponse {
	resp := PhotoResponse{URL: PublicFileURL(p.FilePath), Alt: p.Alt}
	if p.ThumbPath != nil && *p.ThumbPath != "" {
		thumb := PublicFileURL(*p.ThumbPath)
		resp.ThumbURL = &thumb
	}
	return resp
}

// ToResponse converts a lot photo to its public payload.
func (p *LotPhoto) ToResponse() PhotoResponse {
	resp := PhotoResponse{URL: PublicFileURL(p.FilePath), Alt: p.Alt}
	if p.ThumbPath != nil && *p.ThumbPath != "" {
		thumb := PublicFileURL(*p.ThumbPath)
		resp.ThumbURL = &thumb
	}
	return resp
}

// PublicFileURL converts a stored relative path (uploads/...) into the URL
// served by the static bucket.
func PublicFileURL(path string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return ""
	}
	return "/" + strings.TrimPrefix(clean, "/")
}
