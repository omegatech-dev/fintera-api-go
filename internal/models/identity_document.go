package models

import (
	"strconv"
	"time"
)

// Identity document kinds. A person has at most one document of each kind,
// and no other kind can be stored.
const (
	IdentityDocumentKindDNI = "dni"
	IdentityDocumentKindRTN = "rtn"
)

// Identity document sources. "upload" is a document stored through the
// identity document endpoints; "legacy" is a file that was attached to a
// contract before this feature existed and was migrated read-only.
const (
	IdentityDocumentSourceUpload = "upload"
	IdentityDocumentSourceLegacy = "legacy"
)

// IdentityDocument is the DNI or the RTN file of a person. The row is the
// person's slot: one document per kind, replaced on every explicit upload.
type IdentityDocument struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `gorm:"not null;index" json:"user_id"`
	Kind             string    `gorm:"size:16;not null" json:"kind"`
	FilePath         string    `gorm:"column:file_path;not null" json:"file_path"`
	PreviewPath      *string   `gorm:"column:preview_path" json:"preview_path"`
	OriginalFilename string    `gorm:"column:original_filename;not null" json:"original_filename"`
	MimeType         string    `gorm:"column:mime_type;size:120;not null" json:"mime_type"`
	SizeBytes        int64     `gorm:"column:size_bytes;not null" json:"size_bytes"`
	UploadedBy       *uint     `gorm:"column:uploaded_by" json:"uploaded_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName specifies the table name for IdentityDocument.
func (IdentityDocument) TableName() string {
	return "identity_documents"
}

// ContractIdentityDocument links a contract to the identity documents of its
// applicant. It also carries the migrated legacy files, which have no
// person-level row because they cannot be attributed to a slot: their kind is
// NULL and they are read-only.
type ContractIdentityDocument struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	ContractID         uint      `gorm:"not null;index" json:"contract_id"`
	IdentityDocumentID *uint     `gorm:"column:identity_document_id" json:"identity_document_id"`
	Kind               *string   `gorm:"column:kind;size:16" json:"kind"`
	Source             string    `gorm:"column:source;size:16;not null;default:upload" json:"source"`
	FilePath           *string   `gorm:"column:file_path" json:"file_path"`
	OriginalFilename   *string   `gorm:"column:original_filename" json:"original_filename"`
	MimeType           *string   `gorm:"column:mime_type;size:120" json:"mime_type"`
	SizeBytes          *int64    `gorm:"column:size_bytes" json:"size_bytes"`
	CreatedAt          time.Time `json:"created_at"`
}

// TableName specifies the table name for ContractIdentityDocument.
func (ContractIdentityDocument) TableName() string {
	return "contract_identity_documents"
}

// IsLegacy reports whether the entry is a migrated legacy file.
func (c ContractIdentityDocument) IsLegacy() bool {
	return c.Source == IdentityDocumentSourceLegacy
}

// IdentityDocumentFile describes a stored identity document file.
type IdentityDocumentFile struct {
	ID               uint      `json:"id"`
	Kind             *string   `json:"kind,omitempty"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	UploadedAt       time.Time `json:"uploaded_at"`
	PreviewURL       string    `json:"preview_url,omitempty"`
	DownloadURL      string    `json:"download_url"`
}

// IdentityDocumentSlot is one entry of an identity document listing. A slot
// without a document on file reports a nil document instead of being omitted.
type IdentityDocumentSlot struct {
	Kind     *string               `json:"kind"`
	Source   string                `json:"source"`
	ReadOnly bool                  `json:"read_only"`
	Document *IdentityDocumentFile `json:"document"`
}

// IdentityDocumentUserResponse is the payload of the person-scoped listing.
type IdentityDocumentUserResponse struct {
	UserID uint                   `json:"user_id"`
	Slots  []IdentityDocumentSlot `json:"slots"`
}

// IdentityDocumentContractResponse is the payload of the contract-scoped
// listing: the applicant's two slots plus the migrated legacy files.
type IdentityDocumentContractResponse struct {
	ContractID uint                   `json:"contract_id"`
	UserID     uint                   `json:"user_id"`
	Slots      []IdentityDocumentSlot `json:"slots"`
	Legacy     []IdentityDocumentSlot `json:"legacy"`
}

// ToResponse converts a stored document into its file payload. The preview is
// only offered when the file could be rasterized.
func (d *IdentityDocument) ToResponse() IdentityDocumentFile {
	kind := d.Kind
	resp := IdentityDocumentFile{
		ID:               d.ID,
		Kind:             &kind,
		OriginalFilename: d.OriginalFilename,
		MimeType:         d.MimeType,
		SizeBytes:        d.SizeBytes,
		UploadedAt:       d.CreatedAt,
		DownloadURL:      IdentityDocumentContentURL(d.ID),
	}
	if d.PreviewPath != nil && *d.PreviewPath != "" {
		resp.PreviewURL = IdentityDocumentPreviewURL(d.ID)
	}
	return resp
}

// ToResponse converts a legacy entry into its file payload. Legacy files have
// no kind and no preview: they are the original attachments of a contract.
func (c *ContractIdentityDocument) ToResponse(contractGUID string) IdentityDocumentFile {
	resp := IdentityDocumentFile{
		ID:          c.ID,
		DownloadURL: IdentityDocumentLegacyContentURL(contractGUID, c.ID),
	}
	if c.OriginalFilename != nil {
		resp.OriginalFilename = *c.OriginalFilename
	}
	if c.MimeType != nil {
		resp.MimeType = *c.MimeType
	}
	if c.SizeBytes != nil {
		resp.SizeBytes = *c.SizeBytes
	}
	resp.UploadedAt = c.CreatedAt
	return resp
}

// IdentityDocumentContentURL is the authorized route that serves the stored
// original of an identity document.
func IdentityDocumentContentURL(id uint) string {
	return "/api/v1/identity_documents/" + strconv.FormatUint(uint64(id), 10) + "/content"
}

// IdentityDocumentPreviewURL is the authorized route that serves the inline
// preview of an identity document.
func IdentityDocumentPreviewURL(id uint) string {
	return "/api/v1/identity_documents/" + strconv.FormatUint(uint64(id), 10) + "/preview"
}

// IdentityDocumentLegacyContentURL is the authorized route that serves a
// migrated legacy file. It is contract-scoped because a legacy file belongs
// to one contract and to no person.
func IdentityDocumentLegacyContentURL(contractGUID string, entryID uint) string {
	return "/api/v1/contracts/" + contractGUID +
		"/legacy_documents/" + strconv.FormatUint(uint64(entryID), 10) + "/content"
}
