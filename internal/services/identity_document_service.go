package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/storage"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"gorm.io/gorm"
)

// identityDocumentSubDir is the storage directory of the identity documents.
// It has no static route, so the files are only reachable through the
// authorized content endpoints.
const identityDocumentSubDir = "identity_documents"

// identityDocumentPreviewSubDir holds the WebP previews, apart from the
// originals, so both are unambiguous on disk.
const identityDocumentPreviewSubDir = "identity_documents/previews"

// IdentityDocumentViewer identifies the caller of an identity document
// operation. The contract owner is never a viewer: the identity documents are
// internal staff records, so a customer account is rejected even when it
// created its own contract.
type IdentityDocumentViewer struct {
	UserID   uint
	IsAdmin  bool
	IsSeller bool
}

// IdentityDocumentUpload is an identity document carried by a request. The
// service opens the header itself so the file lifecycle stays in one place.
type IdentityDocumentUpload struct {
	Kind   string
	Header *multipart.FileHeader
}

// IdentityDocumentFileContent is an opened file ready to be streamed.
type IdentityDocumentFileContent struct {
	File        *os.File
	MimeType    string
	Filename    string
	SizeBytes   int64
	Disposition string
}

// LegacyMigrationResult summarizes the migration of the files that were
// attached to a contract before the identity documents existed.
type LegacyMigrationResult struct {
	Contracts int64
	Created   int64
	Skipped   int64
	Missing   int64
	Errors    int64
}

// IdentityDocumentService stores, serves and requires the DNI and the RTN of a
// person.
type IdentityDocumentService struct {
	repo         repository.IdentityDocumentRepository
	contractRepo repository.ContractRepository
	storage      *storage.LocalStorage
	auditSvc     *AuditService
	// required enables the contract-creation requirement. It is off during
	// the rollout, while the two frontends are still being deployed.
	required bool
}

// NewIdentityDocumentService creates the identity document service.
func NewIdentityDocumentService(
	repo repository.IdentityDocumentRepository,
	contractRepo repository.ContractRepository,
	store *storage.LocalStorage,
	auditSvc *AuditService,
	required bool,
) *IdentityDocumentService {
	return &IdentityDocumentService{
		repo:         repo,
		contractRepo: contractRepo,
		storage:      store,
		auditSvc:     auditSvc,
		required:     required,
	}
}

// Required reports whether the contract-creation requirement is enforced.
func (s *IdentityDocumentService) Required() bool {
	return s != nil && s.required
}

// ValidateUpload checks one upload before anything is written. The error
// names the offending file.
func ValidateUpload(upload IdentityDocumentUpload) error {
	header := upload.Header
	if header == nil {
		return &ErrIdentityDocumentType{Filename: upload.Kind, ContentType: ""}
	}
	contentType := resolveIdentityDocumentContentType(header)
	if !storage.IsValidIdentityContentType(contentType) {
		return &ErrIdentityDocumentType{Filename: header.Filename, ContentType: contentType}
	}
	if header.Size > storage.MaxFileSize() {
		return &ErrIdentityDocumentSize{
			Filename: header.Filename,
			Size:     header.Size,
			Max:      storage.MaxFileSize(),
		}
	}
	return nil
}

// ValidateUploads checks every upload of a request before anything is
// written, so a rejected request leaves no half-uploaded document behind.
func ValidateUploads(uploads []IdentityDocumentUpload) error {
	for _, upload := range uploads {
		if err := ValidateUpload(upload); err != nil {
			return err
		}
	}
	return nil
}

// resolveIdentityDocumentContentType returns the declared MIME type, falling
// back to the extension when the client sent a generic one.
func resolveIdentityDocumentContentType(header *multipart.FileHeader) string {
	declared := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if declared != "" && declared != "application/octet-stream" {
		return declared
	}
	return mimeFromExtension(header.Filename)
}

// mimeFromExtension maps a file name to its MIME type.
func mimeFromExtension(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	default:
		return "application/octet-stream"
	}
}

// Upsert stores the document of a person for a kind and replaces the previous
// one. Replacing a document is an explicit act: the caller sees the current
// document before it changes.
func (s *IdentityDocumentService) Upsert(ctx context.Context, userID uint, kind string, header *multipart.FileHeader, actorID uint) (*models.IdentityDocument, error) {
	if kind != models.IdentityDocumentKindDNI && kind != models.IdentityDocumentKindRTN {
		return nil, &ErrIdentityDocumentType{Filename: kind, ContentType: "kind desconocido"}
	}
	upload := IdentityDocumentUpload{Kind: kind, Header: header}
	if err := ValidateUpload(upload); err != nil {
		return nil, err
	}

	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	contentType := resolveIdentityDocumentContentType(header)

	previous, err := s.repo.FindByUserAndKind(ctx, userID, kind)
	if err != nil {
		return nil, err
	}

	storedPath, err := s.storage.UploadFromBytes(data, header.Filename, identityDocumentSubDir)
	if err != nil {
		return nil, err
	}

	doc := &models.IdentityDocument{
		UserID:           userID,
		Kind:             kind,
		FilePath:         storedPath,
		OriginalFilename: header.Filename,
		MimeType:         contentType,
		SizeBytes:        int64(len(data)),
	}
	if actorID > 0 {
		doc.UploadedBy = &actorID
	}

	// The preview is optional: the formats the Go decoders cannot read (PDF,
	// HEIC, HEIF) are stored without one instead of being rejected.
	if preview, previewErr := IdentityDocumentToWebP(data); previewErr == nil {
		if previewPath, pathErr := s.storage.UploadFromBytes(preview, "preview.webp", identityDocumentPreviewSubDir); pathErr == nil {
			doc.PreviewPath = &previewPath
		} else {
			logIgnoredError("store identity document preview", pathErr)
		}
	}

	if previous != nil {
		doc.ID = previous.ID
		doc.CreatedAt = previous.CreatedAt
		if err := s.repo.Upsert(ctx, doc); err != nil {
			return nil, err
		}
		// The row is updated before the replaced file is deleted, so a failed
		// delete leaves an orphan file and never a row pointing at a missing
		// one.
		logIgnoredError("delete replaced identity document file", s.storage.Delete(previous.FilePath))
		if previous.PreviewPath != nil && *previous.PreviewPath != "" {
			logIgnoredError("delete replaced identity document preview", s.storage.Delete(*previous.PreviewPath))
		}
	} else if err := s.repo.Upsert(ctx, doc); err != nil {
		return nil, err
	}

	s.logUpload(ctx, actorID, userID, doc, "Documento de identificación subido")
	return doc, nil
}

// logUpload writes the audit trail of an identity document upload.
func (s *IdentityDocumentService) logUpload(ctx context.Context, actorID, userID uint, doc *models.IdentityDocument, description string) {
	if s.auditSvc == nil {
		return
	}
	logIgnoredError("write identity document audit log", s.auditSvc.Log(
		ctx, actorID, "UPLOAD_IDENTITY_DOCUMENT", "IdentityDocument", doc.ID,
		fmt.Sprintf("%s (%s) para el usuario %d", description, doc.Kind, userID), "", "",
	))
}

// MissingKinds returns the kinds the applicant still needs, given the
// documents already on file and the ones carried by the request.
func (s *IdentityDocumentService) MissingKinds(ctx context.Context, userID uint, uploads []IdentityDocumentUpload) ([]string, error) {
	docs, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(docs))
	for _, doc := range docs {
		have[doc.Kind] = true
	}
	for _, upload := range uploads {
		if upload.Kind != "" {
			have[upload.Kind] = true
		}
	}
	missing := make([]string, 0, 2)
	for _, kind := range []string{models.IdentityDocumentKindDNI, models.IdentityDocumentKindRTN} {
		if !have[kind] {
			missing = append(missing, kind)
		}
	}
	return missing, nil
}

// CheckRequirement returns an error when the requirement is enforced and the
// applicant is missing a document.
func (s *IdentityDocumentService) CheckRequirement(ctx context.Context, userID uint, uploads []IdentityDocumentUpload) error {
	if !s.Required() {
		return nil
	}
	missing, err := s.MissingKinds(ctx, userID, uploads)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return ErrIdentityDocumentsRequired{Missing: missing}
	}
	return nil
}

// FillMissing stores the uploaded documents only for the kinds the applicant
// does not have yet. A document that is already on file is kept and reported
// as already present: creating a contract never replaces a stored document.
func (s *IdentityDocumentService) FillMissing(ctx context.Context, userID uint, uploads []IdentityDocumentUpload, actorID uint) (filled []string, alreadyPresent []string, err error) {
	if err := ValidateUploads(uploads); err != nil {
		return nil, nil, err
	}
	docs, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	have := make(map[string]bool, len(docs))
	for _, doc := range docs {
		have[doc.Kind] = true
	}
	for _, upload := range uploads {
		if upload.Kind == "" || upload.Header == nil {
			continue
		}
		if have[upload.Kind] {
			alreadyPresent = append(alreadyPresent, upload.Kind)
			continue
		}
		if _, err := s.Upsert(ctx, userID, upload.Kind, upload.Header, actorID); err != nil {
			return nil, nil, err
		}
		have[upload.Kind] = true
		filled = append(filled, upload.Kind)
	}
	return filled, alreadyPresent, nil
}

// CanReadPerson reports whether the viewer may read the documents of a
// person. A seller reaches them only when it has a contract with that
// applicant in its scope; a customer account never does, not even for its own
// documents.
func (s *IdentityDocumentService) CanReadPerson(ctx context.Context, viewer IdentityDocumentViewer, userID uint) (bool, error) {
	if viewer.IsAdmin {
		return true, nil
	}
	if !viewer.IsSeller || viewer.UserID == 0 {
		return false, nil
	}
	return s.repo.HasContractInScope(ctx, userID, viewer.UserID)
}

// CanReadContract reports whether the viewer may read the documents of a
// contract.
func (s *IdentityDocumentService) CanReadContract(ctx context.Context, viewer IdentityDocumentViewer, contractID uint) (bool, error) {
	if viewer.IsAdmin {
		return true, nil
	}
	if !viewer.IsSeller || viewer.UserID == 0 {
		return false, nil
	}
	return s.repo.ContractInScope(ctx, contractID, viewer.UserID)
}

// DocumentsByUser returns the documents a person has on file.
func (s *IdentityDocumentService) DocumentsByUser(ctx context.Context, userID uint) ([]models.IdentityDocument, error) {
	return s.repo.ListByUser(ctx, userID)
}

// ListByUser returns the two slots of a person. A slot without a document on
// file is reported as empty instead of being omitted.
func (s *IdentityDocumentService) ListByUser(ctx context.Context, userID uint) (*models.IdentityDocumentUserResponse, error) {
	docs, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	byKind := make(map[string]models.IdentityDocument, len(docs))
	for _, doc := range docs {
		byKind[doc.Kind] = doc
	}
	slots := make([]models.IdentityDocumentSlot, 0, 2)
	for _, kind := range []string{models.IdentityDocumentKindDNI, models.IdentityDocumentKindRTN} {
		slot := models.IdentityDocumentSlot{Kind: &kind, Source: models.IdentityDocumentSourceUpload}
		if doc, ok := byKind[kind]; ok {
			file := doc.ToResponse()
			slot.Document = &file
		}
		slots = append(slots, slot)
	}
	return &models.IdentityDocumentUserResponse{UserID: userID, Slots: slots}, nil
}

// isContractNotFound reports whether the repository error means the row does
// not exist. It accepts both the GORM sentinel and the service sentinel so a
// repository that maps not-found itself is handled the same way.
func isContractNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrNotFound)
}

// FindContractByGUID resolves a contract by its public GUID. It returns nil
// without an error when the contract does not exist.
func (s *IdentityDocumentService) FindContractByGUID(ctx context.Context, guid string) (*models.Contract, error) {
	contract, err := s.contractRepo.FindByGUIDWithDetails(ctx, guid)
	if isContractNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return contract, nil
}

// UploadForContract stores the document of the applicant of a contract.
func (s *IdentityDocumentService) UploadForContract(ctx context.Context, contractID uint, kind string, header *multipart.FileHeader, actorID uint) (*models.IdentityDocument, error) {
	contract, err := s.contractRepo.FindByID(ctx, contractID)
	if err != nil {
		if isContractNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if contract == nil {
		return nil, ErrNotFound
	}
	return s.Upsert(ctx, contract.ApplicantUserID, kind, header, actorID)
}

// ListByContract returns the applicant's slots and the migrated legacy files
// of a contract.
func (s *IdentityDocumentService) ListByContract(ctx context.Context, contractID uint) (*models.IdentityDocumentContractResponse, error) {
	contract, err := s.contractRepo.FindByID(ctx, contractID)
	if err != nil {
		if isContractNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if contract == nil {
		return nil, ErrNotFound
	}
	userDocs, err := s.ListByUser(ctx, contract.ApplicantUserID)
	if err != nil {
		return nil, err
	}
	legacyEntries, err := s.repo.ListLegacyByContract(ctx, contractID)
	if err != nil {
		return nil, err
	}
	legacy := make([]models.IdentityDocumentSlot, 0, len(legacyEntries))
	for i := range legacyEntries {
		file := legacyEntries[i].ToResponse(contract.GUID)
		legacy = append(legacy, models.IdentityDocumentSlot{
			Kind:     nil,
			Source:   models.IdentityDocumentSourceLegacy,
			ReadOnly: true,
			Document: &file,
		})
	}
	return &models.IdentityDocumentContractResponse{
		ContractID: contractID,
		UserID:     contract.ApplicantUserID,
		Slots:      userDocs.Slots,
		Legacy:     legacy,
	}, nil
}

// Content opens the stored original of an identity document for the viewer.
func (s *IdentityDocumentService) Content(ctx context.Context, viewer IdentityDocumentViewer, id uint) (*IdentityDocumentFileContent, error) {
	doc, err := s.repo.FindByIDForViewer(ctx, id, viewer.UserID, viewer.IsAdmin, viewer.IsSeller)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNotFound
	}
	return s.openStored(doc.FilePath, doc.MimeType, doc.OriginalFilename, doc.SizeBytes, "attachment")
}

// Preview opens the inline preview of an identity document for the viewer.
// A document the server could not rasterize falls back to the original served
// inline.
func (s *IdentityDocumentService) Preview(ctx context.Context, viewer IdentityDocumentViewer, id uint) (*IdentityDocumentFileContent, error) {
	doc, err := s.repo.FindByIDForViewer(ctx, id, viewer.UserID, viewer.IsAdmin, viewer.IsSeller)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNotFound
	}
	if doc.PreviewPath != nil && *doc.PreviewPath != "" {
		content, err := s.openStored(*doc.PreviewPath, "image/webp", doc.OriginalFilename, 0, "inline")
		if err == nil {
			return content, nil
		}
		logIgnoredError("open identity document preview", err)
	}
	return s.openStored(doc.FilePath, doc.MimeType, doc.OriginalFilename, doc.SizeBytes, "inline")
}

// LegacyContent opens a migrated legacy file for the viewer. The file belongs
// to one contract and to no person, so the contract scope applies.
func (s *IdentityDocumentService) LegacyContent(ctx context.Context, viewer IdentityDocumentViewer, contractID, entryID uint) (*IdentityDocumentFileContent, error) {
	entry, err := s.repo.FindLegacyEntryForViewer(ctx, contractID, entryID, viewer.UserID, viewer.IsAdmin, viewer.IsSeller)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, ErrNotFound
	}
	if entry.FilePath == nil || *entry.FilePath == "" {
		return nil, ErrNotFound
	}
	mimeType := "application/octet-stream"
	if entry.MimeType != nil && *entry.MimeType != "" {
		mimeType = *entry.MimeType
	}
	filename := "documento"
	if entry.OriginalFilename != nil && *entry.OriginalFilename != "" {
		filename = *entry.OriginalFilename
	}
	size := int64(0)
	if entry.SizeBytes != nil {
		size = *entry.SizeBytes
	}
	return s.openStored(*entry.FilePath, mimeType, filename, size, "inline")
}

// openStored opens a stored file for streaming, resolving it through the
// storage root so a malformed path cannot escape it.
func (s *IdentityDocumentService) openStored(relativePath, mimeType, filename string, size int64, disposition string) (*IdentityDocumentFileContent, error) {
	if _, err := s.storage.SafeFullPath(relativePath); err != nil {
		return nil, err
	}
	file, err := s.storage.Download(relativePath)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		if info, statErr := file.Stat(); statErr == nil {
			size = info.Size()
		}
	}
	if mimeType == "" {
		mimeType = mimeFromExtension(filename)
	}
	return &IdentityDocumentFileContent{
		File:        file,
		MimeType:    mimeType,
		Filename:    filename,
		SizeBytes:   size,
		Disposition: disposition,
	}, nil
}

// MigrateLegacyDocuments creates a read-only entry for every file that was
// attached to a contract before the identity documents existed. The files are
// referenced where they already are: nothing is copied and nothing is moved.
func (s *IdentityDocumentService) MigrateLegacyDocuments(ctx context.Context) (LegacyMigrationResult, error) {
	var result LegacyMigrationResult

	// Count first: most deployments have nothing to migrate, and a task that
	// does nothing must not walk the contracts table.
	count, err := s.repo.CountContractsWithLegacyDocuments(ctx)
	if err != nil {
		return result, err
	}
	result.Contracts = count
	if count == 0 {
		return result, nil
	}

	contracts, err := s.repo.ListContractsWithLegacyDocuments(ctx)
	if err != nil {
		return result, err
	}

	for i := range contracts {
		contract := contracts[i]
		paths := repository.LegacyDocumentPaths(&contract)
		for _, path := range paths {
			if strings.TrimSpace(path) == "" {
				continue
			}
			exists, err := s.repo.LegacyPathExists(ctx, contract.ID, path)
			if err != nil {
				result.Errors++
				continue
			}
			if exists {
				result.Skipped++
				continue
			}
			entry := s.buildLegacyEntry(contract.ID, path)
			if entry.SizeBytes == nil {
				result.Missing++
				logger.Warn("legacy identity document file is missing", "contract_id", contract.ID, "path", path)
			}
			if err := s.repo.CreateLink(ctx, entry); err != nil {
				result.Errors++
				logger.Warn("legacy identity document entry failed", "contract_id", contract.ID, "path", path, "error", err)
				continue
			}
			result.Created++
		}
	}

	return result, nil
}

// buildLegacyEntry builds the read-only row of a stored contract attachment.
func (s *IdentityDocumentService) buildLegacyEntry(contractID uint, path string) *models.ContractIdentityDocument {
	entry := &models.ContractIdentityDocument{
		ContractID: contractID,
		Source:     models.IdentityDocumentSourceLegacy,
		FilePath:   &path,
	}
	name := filepath.Base(path)
	entry.OriginalFilename = &name
	mimeType := mimeFromExtension(name)
	entry.MimeType = &mimeType
	if size, sizeErr := s.storage.GetSize(path); sizeErr == nil {
		entry.SizeBytes = &size
	}
	return entry
}

// AttachLegacyFiles exposes the attachments of a freshly created contract in
// the identity document listing. The files stay where the contract upload put
// them: the row only references the path, so the attachment is visible without
// waiting for the one-off migration task.
func (s *IdentityDocumentService) AttachLegacyFiles(ctx context.Context, contractID uint, paths []string) error {
	if s == nil || len(paths) == 0 {
		return nil
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		exists, err := s.repo.LegacyPathExists(ctx, contractID, path)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := s.repo.CreateLink(ctx, s.buildLegacyEntry(contractID, path)); err != nil {
			return err
		}
	}
	return nil
}

// IsIdentityDocumentNotFound reports whether the error means the document
// does not exist or is not visible to the caller.
func IsIdentityDocumentNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
