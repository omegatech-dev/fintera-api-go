package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityDocumentKindsAreOnlyDNIAndRTN(t *testing.T) {
	assert.Equal(t, "dni", IdentityDocumentKindDNI)
	assert.Equal(t, "rtn", IdentityDocumentKindRTN)
}

func TestIdentityDocumentSourcesAreUploadAndLegacy(t *testing.T) {
	assert.Equal(t, "upload", IdentityDocumentSourceUpload)
	assert.Equal(t, "legacy", IdentityDocumentSourceLegacy)
}

func TestIdentityDocumentTableNames(t *testing.T) {
	assert.Equal(t, "identity_documents", IdentityDocument{}.TableName())
	assert.Equal(t, "contract_identity_documents", ContractIdentityDocument{}.TableName())
}

func TestIdentityDocumentToResponseOffersThePreviewOnlyWhenItExists(t *testing.T) {
	uploaded := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC)
	doc := IdentityDocument{
		ID: 12, UserID: 7, Kind: IdentityDocumentKindDNI,
		OriginalFilename: "dni.png", MimeType: "image/png", SizeBytes: 2048,
		CreatedAt: uploaded,
	}

	response := doc.ToResponse()
	assert.Equal(t, uint(12), response.ID)
	require.NotNil(t, response.Kind)
	assert.Equal(t, IdentityDocumentKindDNI, *response.Kind)
	assert.Equal(t, "/api/v1/identity_documents/12/content", response.DownloadURL)
	assert.Empty(t, response.PreviewURL)

	preview := "identity_documents/previews/2026/01/abc.webp"
	doc.PreviewPath = &preview
	response = doc.ToResponse()
	assert.Equal(t, "/api/v1/identity_documents/12/preview", response.PreviewURL)
}

func TestContractIdentityDocumentToResponseBuildsTheContractScopedURL(t *testing.T) {
	name := "adjunto.pdf"
	mimeType := "application/pdf"
	size := int64(4096)
	entry := ContractIdentityDocument{
		ID: 9, ContractID: 4, Source: IdentityDocumentSourceLegacy,
		OriginalFilename: &name, MimeType: &mimeType, SizeBytes: &size,
	}

	response := entry.ToResponse("abc-123")
	assert.Equal(t, uint(9), response.ID)
	assert.Nil(t, response.Kind)
	assert.Equal(t, "/api/v1/contracts/abc-123/legacy_documents/9/content", response.DownloadURL)
	assert.Equal(t, "adjunto.pdf", response.OriginalFilename)
	assert.Equal(t, "application/pdf", response.MimeType)
	assert.Equal(t, int64(4096), response.SizeBytes)
	assert.Empty(t, response.PreviewURL, "a legacy file has no preview")
}

func TestContractIdentityDocumentIsLegacy(t *testing.T) {
	assert.True(t, ContractIdentityDocument{Source: IdentityDocumentSourceLegacy}.IsLegacy())
	assert.False(t, ContractIdentityDocument{Source: IdentityDocumentSourceUpload}.IsLegacy())
}

func TestIdentityDocumentSlotSerializesAnEmptySlot(t *testing.T) {
	kind := IdentityDocumentKindRTN
	payload, err := json.Marshal(IdentityDocumentSlot{Kind: &kind, Source: IdentityDocumentSourceUpload})
	require.NoError(t, err)

	assert.JSONEq(t, `{"kind":"rtn","source":"upload","read_only":false,"document":null}`, string(payload))
}
