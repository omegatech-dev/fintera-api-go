package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// sellerScopePredicate is the scope the contract listing already uses: a
// seller reaches the contracts it created and the contracts assigned to it and
// accepted. The identity documents reuse it verbatim so the listing and the
// detail cannot drift apart.
const sellerScopePredicate = "(contracts.creator_id = ? OR (contracts.seller_id = ? AND contracts.assignment_status = ?))"

// IdentityDocumentRepository defines the interface for identity document data
// access.
type IdentityDocumentRepository interface {
	// FindByUserAndKind returns the document a person has for a kind. It
	// returns nil without an error when the slot is empty.
	FindByUserAndKind(ctx context.Context, userID uint, kind string) (*models.IdentityDocument, error)
	// FindByID returns a document by id.
	FindByID(ctx context.Context, id uint) (*models.IdentityDocument, error)
	// FindByIDForViewer returns a document when the viewer is an admin or a
	// seller with a contract with the owner of the document in its scope. A
	// customer account gets nothing, not even for its own document.
	FindByIDForViewer(ctx context.Context, id, viewerID uint, isAdmin, isSeller bool) (*models.IdentityDocument, error)
	// ListByUser returns the documents a person has.
	ListByUser(ctx context.Context, userID uint) ([]models.IdentityDocument, error)
	// ListLinksByContract returns the identity document links of a contract.
	ListLinksByContract(ctx context.Context, contractID uint) ([]models.ContractIdentityDocument, error)
	// ListLegacyByContract returns the migrated legacy entries of a contract.
	ListLegacyByContract(ctx context.Context, contractID uint) ([]models.ContractIdentityDocument, error)
	// Upsert creates or replaces the document of a person for a kind.
	Upsert(ctx context.Context, doc *models.IdentityDocument) error
	// Delete removes a document.
	Delete(ctx context.Context, id uint) error
	// ReplaceLink creates or replaces the link of a contract for a kind.
	ReplaceLink(ctx context.Context, link *models.ContractIdentityDocument) error
	// CreateLink inserts a link row. It is used by the legacy migration,
	// whose entries have no kind.
	CreateLink(ctx context.Context, link *models.ContractIdentityDocument) error
	// FindLegacyEntryForViewer returns a legacy entry of a contract when the
	// viewer is an admin or a seller with the contract in its scope.
	FindLegacyEntryForViewer(ctx context.Context, contractID, entryID uint, viewerID uint, isAdmin, isSeller bool) (*models.ContractIdentityDocument, error)
	// DeleteLinkForContractAndKind removes the link of a contract for a kind.
	DeleteLinkForContractAndKind(ctx context.Context, contractID uint, kind string) error
	// HasContractInScope reports whether the viewer has a contract with the
	// applicant in the seller scope.
	HasContractInScope(ctx context.Context, applicantUserID, viewerID uint) (bool, error)
	// ContractInScope reports whether a contract is in the seller scope of the
	// viewer.
	ContractInScope(ctx context.Context, contractID, viewerID uint) (bool, error)
	// CountContractsWithLegacyDocuments counts the contracts that still carry
	// a non-empty document_paths.
	CountContractsWithLegacyDocuments(ctx context.Context) (int64, error)
	// ListContractsWithLegacyDocuments returns the contracts that still carry
	// a non-empty document_paths.
	ListContractsWithLegacyDocuments(ctx context.Context) ([]models.Contract, error)
	// LegacyPathExists reports whether a contract already has a legacy entry
	// for a stored path.
	LegacyPathExists(ctx context.Context, contractID uint, path string) (bool, error)
}

type identityDocumentRepository struct {
	db *gorm.DB
}

// NewIdentityDocumentRepository creates a new identity document repository.
func NewIdentityDocumentRepository(db *gorm.DB) IdentityDocumentRepository {
	return &identityDocumentRepository{db: db}
}

func (r *identityDocumentRepository) FindByUserAndKind(ctx context.Context, userID uint, kind string) (*models.IdentityDocument, error) {
	var doc models.IdentityDocument
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND kind = ?", userID, kind).
		First(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *identityDocumentRepository) FindByID(ctx context.Context, id uint) (*models.IdentityDocument, error) {
	var doc models.IdentityDocument
	err := r.db.WithContext(ctx).First(&doc, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *identityDocumentRepository) FindByIDForViewer(ctx context.Context, id, viewerID uint, isAdmin, isSeller bool) (*models.IdentityDocument, error) {
	if !isAdmin && (!isSeller || viewerID == 0) {
		return nil, nil
	}
	var doc models.IdentityDocument
	db := r.db.WithContext(ctx).Where("identity_documents.id = ?", id)
	if !isAdmin {
		db = db.Where(
			"EXISTS (SELECT 1 FROM contracts WHERE contracts.applicant_user_id = identity_documents.user_id AND "+sellerScopePredicate+")",
			viewerID, viewerID, models.AssignmentStatusAccepted,
		)
	}
	err := db.First(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *identityDocumentRepository) ListByUser(ctx context.Context, userID uint) ([]models.IdentityDocument, error) {
	var docs []models.IdentityDocument
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("kind ASC").
		Find(&docs).Error
	return docs, err
}

func (r *identityDocumentRepository) ListLinksByContract(ctx context.Context, contractID uint) ([]models.ContractIdentityDocument, error) {
	var links []models.ContractIdentityDocument
	err := r.db.WithContext(ctx).
		Where("contract_id = ? AND source = ?", contractID, models.IdentityDocumentSourceUpload).
		Order("id ASC").
		Find(&links).Error
	return links, err
}

func (r *identityDocumentRepository) ListLegacyByContract(ctx context.Context, contractID uint) ([]models.ContractIdentityDocument, error) {
	var links []models.ContractIdentityDocument
	err := r.db.WithContext(ctx).
		Where("contract_id = ? AND source = ?", contractID, models.IdentityDocumentSourceLegacy).
		Order("id ASC").
		Find(&links).Error
	return links, err
}

func (r *identityDocumentRepository) Upsert(ctx context.Context, doc *models.IdentityDocument) error {
	return r.db.WithContext(ctx).Save(doc).Error
}

func (r *identityDocumentRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.IdentityDocument{}, id).Error
}

func (r *identityDocumentRepository) ReplaceLink(ctx context.Context, link *models.ContractIdentityDocument) error {
	var existing models.ContractIdentityDocument
	err := r.db.WithContext(ctx).
		Where("contract_id = ? AND kind = ?", link.ContractID, link.Kind).
		First(&existing).Error
	if err == nil {
		link.ID = existing.ID
		return r.db.WithContext(ctx).Save(link).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *identityDocumentRepository) CreateLink(ctx context.Context, link *models.ContractIdentityDocument) error {
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *identityDocumentRepository) FindLegacyEntryForViewer(ctx context.Context, contractID, entryID uint, viewerID uint, isAdmin, isSeller bool) (*models.ContractIdentityDocument, error) {
	if !isAdmin && (!isSeller || viewerID == 0) {
		return nil, nil
	}
	var entry models.ContractIdentityDocument
	db := r.db.WithContext(ctx).
		Where("id = ? AND contract_id = ? AND source = ?", entryID, contractID, models.IdentityDocumentSourceLegacy)
	if !isAdmin {
		db = db.Where(
			"EXISTS (SELECT 1 FROM contracts WHERE contracts.id = contract_identity_documents.contract_id AND "+sellerScopePredicate+")",
			viewerID, viewerID, models.AssignmentStatusAccepted,
		)
	}
	err := db.First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *identityDocumentRepository) DeleteLinkForContractAndKind(ctx context.Context, contractID uint, kind string) error {
	return r.db.WithContext(ctx).
		Where("contract_id = ? AND kind = ?", contractID, kind).
		Delete(&models.ContractIdentityDocument{}).Error
}

func (r *identityDocumentRepository) HasContractInScope(ctx context.Context, applicantUserID, viewerID uint) (bool, error) {
	if viewerID == 0 {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Where("applicant_user_id = ?", applicantUserID).
		Where(sellerScopePredicate, viewerID, viewerID, models.AssignmentStatusAccepted).
		Count(&count).Error
	return count > 0, err
}

func (r *identityDocumentRepository) ContractInScope(ctx context.Context, contractID, viewerID uint) (bool, error) {
	if viewerID == 0 {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Where("id = ?", contractID).
		Where(sellerScopePredicate, viewerID, viewerID, models.AssignmentStatusAccepted).
		Count(&count).Error
	return count > 0, err
}

func (r *identityDocumentRepository) CountContractsWithLegacyDocuments(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Where("document_paths IS NOT NULL AND document_paths <> '' AND document_paths <> '[]'").
		Count(&count).Error
	return count, err
}

func (r *identityDocumentRepository) ListContractsWithLegacyDocuments(ctx context.Context) ([]models.Contract, error) {
	var contracts []models.Contract
	err := r.db.WithContext(ctx).
		Where("document_paths IS NOT NULL AND document_paths <> '' AND document_paths <> '[]'").
		Order("id ASC").
		Find(&contracts).Error
	return contracts, err
}

func (r *identityDocumentRepository) LegacyPathExists(ctx context.Context, contractID uint, path string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ContractIdentityDocument{}).
		Where("contract_id = ? AND source = ? AND file_path = ?", contractID, models.IdentityDocumentSourceLegacy, path).
		Count(&count).Error
	return count > 0, err
}

// LegacyDocumentPaths decodes the free-form attachment list of a contract.
func LegacyDocumentPaths(contract *models.Contract) []string {
	if contract.DocumentPaths == nil || *contract.DocumentPaths == "" {
		return nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(*contract.DocumentPaths), &paths); err != nil {
		return nil
	}
	return paths
}
