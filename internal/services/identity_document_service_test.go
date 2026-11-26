package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/textproto"
	"path/filepath"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIdentityRepo is an in-memory IdentityDocumentRepository. The embedded
// interface keeps the methods a test does not exercise inert.
type fakeIdentityRepo struct {
	repository.IdentityDocumentRepository

	docs        map[string]*models.IdentityDocument
	legacy      []models.ContractIdentityDocument
	withLegacy  []models.Contract
	legacyCount int64
	created     []models.ContractIdentityDocument
	personScope bool
	contractOK  bool
}

func newFakeIdentityRepo() *fakeIdentityRepo {
	return &fakeIdentityRepo{docs: make(map[string]*models.IdentityDocument)}
}

func docKey(userID uint, kind string) string {
	return fmt.Sprintf("%d:%s", userID, kind)
}

func (f *fakeIdentityRepo) FindByUserAndKind(_ context.Context, userID uint, kind string) (*models.IdentityDocument, error) {
	if doc, ok := f.docs[docKey(userID, kind)]; ok {
		copied := *doc
		return &copied, nil
	}
	return nil, nil
}

func (f *fakeIdentityRepo) FindByID(_ context.Context, id uint) (*models.IdentityDocument, error) {
	for _, doc := range f.docs {
		if doc.ID == id {
			copied := *doc
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *fakeIdentityRepo) FindByIDForViewer(_ context.Context, id, _ uint, _, _ bool) (*models.IdentityDocument, error) {
	return f.FindByID(context.Background(), id)
}

func (f *fakeIdentityRepo) ListByUser(_ context.Context, userID uint) ([]models.IdentityDocument, error) {
	var result []models.IdentityDocument
	for _, kind := range []string{models.IdentityDocumentKindDNI, models.IdentityDocumentKindRTN} {
		if doc, ok := f.docs[docKey(userID, kind)]; ok {
			result = append(result, *doc)
		}
	}
	return result, nil
}

func (f *fakeIdentityRepo) ListLinksByContract(context.Context, uint) ([]models.ContractIdentityDocument, error) {
	return nil, nil
}

func (f *fakeIdentityRepo) ListLegacyByContract(context.Context, uint) ([]models.ContractIdentityDocument, error) {
	return f.legacy, nil
}

var fakeIdentityRepoNextID uint = 100

func (f *fakeIdentityRepo) Upsert(_ context.Context, doc *models.IdentityDocument) error {
	if doc.ID == 0 {
		fakeIdentityRepoNextID++
		doc.ID = fakeIdentityRepoNextID
	}
	copied := *doc
	f.docs[docKey(doc.UserID, doc.Kind)] = &copied
	return nil
}

func (f *fakeIdentityRepo) Delete(context.Context, uint) error { return nil }

func (f *fakeIdentityRepo) ReplaceLink(context.Context, *models.ContractIdentityDocument) error {
	return nil
}

func (f *fakeIdentityRepo) CreateLink(_ context.Context, link *models.ContractIdentityDocument) error {
	f.created = append(f.created, *link)
	return nil
}

func (f *fakeIdentityRepo) DeleteLinkForContractAndKind(context.Context, uint, string) error {
	return nil
}

func (f *fakeIdentityRepo) FindLegacyEntryForViewer(_ context.Context, contractID, entryID, _ uint, _, _ bool) (*models.ContractIdentityDocument, error) {
	for i := range f.legacy {
		if f.legacy[i].ID == entryID && f.legacy[i].ContractID == contractID {
			copied := f.legacy[i]
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *fakeIdentityRepo) HasContractInScope(context.Context, uint, uint) (bool, error) {
	return f.personScope, nil
}

func (f *fakeIdentityRepo) ContractInScope(context.Context, uint, uint) (bool, error) {
	return f.contractOK, nil
}

func (f *fakeIdentityRepo) CountContractsWithLegacyDocuments(context.Context) (int64, error) {
	return f.legacyCount, nil
}

func (f *fakeIdentityRepo) ListContractsWithLegacyDocuments(context.Context) ([]models.Contract, error) {
	return f.withLegacy, nil
}

func (f *fakeIdentityRepo) LegacyPathExists(_ context.Context, contractID uint, path string) (bool, error) {
	for _, entry := range f.created {
		if entry.ContractID == contractID && entry.FilePath != nil && *entry.FilePath == path {
			return true, nil
		}
	}
	return false, nil
}

// fakeContractRepo answers only FindByID. The embedded interface keeps the
// rest inert.
type fakeContractRepo struct {
	repository.ContractRepository

	contract *models.Contract
	created  []*models.Contract
}

func (f *fakeContractRepo) FindByID(context.Context, uint) (*models.Contract, error) {
	if f.contract == nil {
		return nil, ErrNotFound
	}
	return f.contract, nil
}

func (f *fakeContractRepo) Create(_ context.Context, contract *models.Contract) error {
	f.created = append(f.created, contract)
	return nil
}

func newIdentityTestStorage(t *testing.T) *storage.LocalStorage {
	t.Helper()
	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	return store
}

func newIdentityTestService(t *testing.T, repo *fakeIdentityRepo, contractRepo *fakeContractRepo, required bool) *IdentityDocumentService {
	t.Helper()
	return NewIdentityDocumentService(repo, contractRepo, newIdentityTestStorage(t), nil, required)
}

// newUploadFile builds the multipart header a handler would receive.
func newUploadFile(t *testing.T, filename, contentType string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	reader := multipart.NewReader(&body, writer.Boundary())
	form, err := reader.ReadForm(int64(len(data)) + 2048)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	require.NotEmpty(t, form.File["file"])
	return form.File["file"][0]
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestUpsertStoresTheFileAndItsPreview(t *testing.T) {
	repo := newFakeIdentityRepo()
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	doc, err := svc.Upsert(context.Background(), 7, models.IdentityDocumentKindDNI,
		newUploadFile(t, "dni.png", "image/png", tinyPNG(t)), 3)
	require.NoError(t, err)

	assert.Equal(t, models.IdentityDocumentKindDNI, doc.Kind)
	assert.Equal(t, "dni.png", doc.OriginalFilename)
	assert.Equal(t, "image/png", doc.MimeType)
	assert.NotEmpty(t, doc.FilePath)
	require.NotNil(t, doc.PreviewPath)
	assert.True(t, svc.storage.Exists(doc.FilePath))
	assert.True(t, svc.storage.Exists(*doc.PreviewPath))
	require.NotNil(t, doc.UploadedBy)
	assert.Equal(t, uint(3), *doc.UploadedBy)
}

func TestUpsertReplacesThePreviousDocumentAndDeletesItsFile(t *testing.T) {
	repo := newFakeIdentityRepo()
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	first, err := svc.Upsert(context.Background(), 7, models.IdentityDocumentKindRTN,
		newUploadFile(t, "rtn.png", "image/png", tinyPNG(t)), 3)
	require.NoError(t, err)
	firstID := first.ID
	oldPath := first.FilePath
	oldPreview := *first.PreviewPath

	second, err := svc.Upsert(context.Background(), 7, models.IdentityDocumentKindRTN,
		newUploadFile(t, "rtn-nuevo.png", "image/png", tinyPNG(t)), 3)
	require.NoError(t, err)

	assert.Equal(t, firstID, second.ID, "the row is replaced, not duplicated")
	assert.Equal(t, "rtn-nuevo.png", second.OriginalFilename)
	assert.NotEqual(t, oldPath, second.FilePath)
	assert.False(t, svc.storage.Exists(oldPath), "the replaced file is deleted")
	assert.False(t, svc.storage.Exists(oldPreview), "the replaced preview is deleted")
	assert.True(t, svc.storage.Exists(second.FilePath))
}

func TestUpsertStoresPDFWithoutPreview(t *testing.T) {
	repo := newFakeIdentityRepo()
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	doc, err := svc.Upsert(context.Background(), 7, models.IdentityDocumentKindDNI,
		newUploadFile(t, "dni.pdf", "application/pdf", []byte("%PDF-1.4 fake")), 3)
	require.NoError(t, err)

	assert.Nil(t, doc.PreviewPath)
	assert.True(t, svc.storage.Exists(doc.FilePath))
	response := doc.ToResponse()
	assert.Empty(t, response.PreviewURL)
	assert.NotEmpty(t, response.DownloadURL)
}

func TestUpsertRejectsAKindOutsideDNIAndRTN(t *testing.T) {
	repo := newFakeIdentityRepo()
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	_, err := svc.Upsert(context.Background(), 7, "pasaporte",
		newUploadFile(t, "p.png", "image/png", tinyPNG(t)), 3)
	require.Error(t, err)

	var typeErr *ErrIdentityDocumentType
	require.True(t, errors.As(err, &typeErr))
	assert.Empty(t, repo.docs)
}

func TestUpsertRejectsAnUnsupportedTypeNamingTheFile(t *testing.T) {
	repo := newFakeIdentityRepo()
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	_, err := svc.Upsert(context.Background(), 7, models.IdentityDocumentKindDNI,
		newUploadFile(t, "dni.zip", "application/zip", []byte("PK\x03\x04")), 3)
	require.Error(t, err)

	var typeErr *ErrIdentityDocumentType
	require.True(t, errors.As(err, &typeErr))
	assert.Equal(t, "dni.zip", typeErr.Filename)
	assert.Contains(t, err.Error(), "dni.zip")
	assert.Empty(t, repo.docs)
}

func TestValidateUploadRejectsAFileOverTheLimit(t *testing.T) {
	header := &multipart.FileHeader{
		Filename: "dni.png",
		Size:     storage.MaxFileSize() + 1,
		Header:   textproto.MIMEHeader{"Content-Type": []string{"image/png"}},
	}

	err := ValidateUpload(IdentityDocumentUpload{Kind: models.IdentityDocumentKindDNI, Header: header})
	require.Error(t, err)

	var sizeErr *ErrIdentityDocumentSize
	require.True(t, errors.As(err, &sizeErr))
	assert.Equal(t, "dni.png", sizeErr.Filename)
	assert.Contains(t, err.Error(), "dni.png")
}

func TestMissingKindsCountsDocumentsOnFileAndInTheRequest(t *testing.T) {
	repo := newFakeIdentityRepo()
	repo.docs[docKey(7, models.IdentityDocumentKindDNI)] = &models.IdentityDocument{
		ID: 1, UserID: 7, Kind: models.IdentityDocumentKindDNI,
	}
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, true)

	missing, err := svc.MissingKinds(context.Background(), 7, []IdentityDocumentUpload{
		{Kind: models.IdentityDocumentKindRTN, Header: nil},
	})
	require.NoError(t, err)
	assert.Empty(t, missing)
}

func TestMissingKindsReportsBothWhenThePersonHasNothing(t *testing.T) {
	svc := newIdentityTestService(t, newFakeIdentityRepo(), &fakeContractRepo{}, true)

	missing, err := svc.MissingKinds(context.Background(), 7, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{models.IdentityDocumentKindDNI, models.IdentityDocumentKindRTN}, missing)
}

func TestCheckRequirementIsANoOpWhenTheFlagIsOff(t *testing.T) {
	svc := newIdentityTestService(t, newFakeIdentityRepo(), &fakeContractRepo{}, false)

	require.NoError(t, svc.CheckRequirement(context.Background(), 7, nil))
}

func TestCheckRequirementNamesTheMissingKindsWhenTheFlagIsOn(t *testing.T) {
	svc := newIdentityTestService(t, newFakeIdentityRepo(), &fakeContractRepo{}, true)

	err := svc.CheckRequirement(context.Background(), 7, nil)
	require.Error(t, err)

	var required ErrIdentityDocumentsRequired
	require.True(t, errors.As(err, &required))
	assert.Equal(t, []string{models.IdentityDocumentKindDNI, models.IdentityDocumentKindRTN}, required.Missing)
}

func TestFillMissingKeepsADocumentAlreadyOnFile(t *testing.T) {
	repo := newFakeIdentityRepo()
	stored := &models.IdentityDocument{
		ID: 11, UserID: 7, Kind: models.IdentityDocumentKindDNI,
		FilePath: "identity_documents/2026/01/old.png", OriginalFilename: "viejo.png",
	}
	repo.docs[docKey(7, models.IdentityDocumentKindDNI)] = stored
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	filled, already, err := svc.FillMissing(context.Background(), 7, []IdentityDocumentUpload{
		{Kind: models.IdentityDocumentKindDNI, Header: newUploadFile(t, "nuevo.png", "image/png", tinyPNG(t))},
		{Kind: models.IdentityDocumentKindRTN, Header: newUploadFile(t, "rtn.png", "image/png", tinyPNG(t))},
	}, 3)
	require.NoError(t, err)

	assert.Equal(t, []string{models.IdentityDocumentKindRTN}, filled)
	assert.Equal(t, []string{models.IdentityDocumentKindDNI}, already)
	kept := repo.docs[docKey(7, models.IdentityDocumentKindDNI)]
	assert.Equal(t, "viejo.png", kept.OriginalFilename, "the stored document is not replaced")
}

func TestListByUserReportsBothSlots(t *testing.T) {
	repo := newFakeIdentityRepo()
	repo.docs[docKey(7, models.IdentityDocumentKindRTN)] = &models.IdentityDocument{
		ID: 5, UserID: 7, Kind: models.IdentityDocumentKindRTN,
		FilePath: "identity_documents/2026/01/rtn.png", MimeType: "image/png",
		OriginalFilename: "rtn.png",
	}
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	result, err := svc.ListByUser(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, result.Slots, 2)

	assert.Equal(t, models.IdentityDocumentKindDNI, *result.Slots[0].Kind)
	assert.Nil(t, result.Slots[0].Document, "an empty slot is reported, not omitted")
	assert.Equal(t, models.IdentityDocumentKindRTN, *result.Slots[1].Kind)
	require.NotNil(t, result.Slots[1].Document)
	assert.Equal(t, "rtn.png", result.Slots[1].Document.OriginalFilename)
}

func TestListByContractAddsTheLegacyEntriesAsReadOnly(t *testing.T) {
	repo := newFakeIdentityRepo()
	legacyPath := "contracts/2025/03/adjunto.pdf"
	legacyName := "adjunto.pdf"
	legacyMime := "application/pdf"
	repo.legacy = []models.ContractIdentityDocument{{
		ID: 9, ContractID: 4, Source: models.IdentityDocumentSourceLegacy,
		FilePath: &legacyPath, OriginalFilename: &legacyName, MimeType: &legacyMime,
	}}
	svc := newIdentityTestService(t, repo, &fakeContractRepo{contract: &models.Contract{
		ID: 4, GUID: "abc-123", ApplicantUserID: 7,
	}}, false)

	result, err := svc.ListByContract(context.Background(), 4)
	require.NoError(t, err)

	require.Len(t, result.Slots, 2)
	require.Len(t, result.Legacy, 1)
	assert.True(t, result.Legacy[0].ReadOnly)
	assert.Nil(t, result.Legacy[0].Kind)
	require.NotNil(t, result.Legacy[0].Document)
	assert.Equal(t, "adjunto.pdf", result.Legacy[0].Document.OriginalFilename)
	assert.Contains(t, result.Legacy[0].Document.DownloadURL, "/contracts/abc-123/legacy_documents/9/content")
}

func TestCanReadPersonAllowsAdminsAndScopedSellersOnly(t *testing.T) {
	repo := newFakeIdentityRepo()
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	allowed, err := svc.CanReadPerson(context.Background(), IdentityDocumentViewer{UserID: 1, IsAdmin: true}, 7)
	require.NoError(t, err)
	assert.True(t, allowed)

	repo.personScope = false
	allowed, err = svc.CanReadPerson(context.Background(), IdentityDocumentViewer{UserID: 2, IsSeller: true}, 7)
	require.NoError(t, err)
	assert.False(t, allowed, "a seller without a contract in scope is rejected")

	repo.personScope = true
	allowed, err = svc.CanReadPerson(context.Background(), IdentityDocumentViewer{UserID: 2, IsSeller: true}, 7)
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = svc.CanReadPerson(context.Background(), IdentityDocumentViewer{}, 7)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestCanReadRejectsTheContractOwnerEvenWhenItCreatedTheContract(t *testing.T) {
	// The portal creates the reservation with the customer as creator, so the
	// seller predicate would match. The role check is what keeps the identity
	// documents internal.
	repo := newFakeIdentityRepo()
	repo.personScope = true
	repo.contractOK = true
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	owner := IdentityDocumentViewer{UserID: 7}
	allowed, err := svc.CanReadPerson(context.Background(), owner, 7)
	require.NoError(t, err)
	assert.False(t, allowed)

	allowed, err = svc.CanReadContract(context.Background(), owner, 4)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestMigrateLegacyDocumentsDoesNothingWhenThereIsNothingToMigrate(t *testing.T) {
	repo := newFakeIdentityRepo()
	repo.legacyCount = 0
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	result, err := svc.MigrateLegacyDocuments(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(0), result.Contracts)
	assert.Empty(t, repo.created)
}

func TestMigrateLegacyDocumentsCreatesOneReadOnlyRowPerFile(t *testing.T) {
	store := newIdentityTestStorage(t)
	first, err := store.UploadFromBytes([]byte("%PDF-1.4 one"), "dni-frente.pdf", "contracts")
	require.NoError(t, err)
	second, err := store.UploadFromBytes([]byte("%PDF-1.4 two"), "dni-reverso.pdf", "contracts")
	require.NoError(t, err)

	paths := fmt.Sprintf(`["%s","%s"]`, filepath.ToSlash(first), filepath.ToSlash(second))
	repo := newFakeIdentityRepo()
	repo.legacyCount = 1
	repo.withLegacy = []models.Contract{{ID: 4, DocumentPaths: &paths}}
	svc := NewIdentityDocumentService(repo, &fakeContractRepo{}, store, nil, false)

	result, err := svc.MigrateLegacyDocuments(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(1), result.Contracts)
	assert.Equal(t, int64(2), result.Created)
	assert.Len(t, repo.created, 2)
	for _, entry := range repo.created {
		assert.Equal(t, models.IdentityDocumentSourceLegacy, entry.Source)
		assert.Nil(t, entry.Kind)
		assert.NotNil(t, entry.FilePath)
		assert.NotNil(t, entry.SizeBytes)
		assert.Greater(t, *entry.SizeBytes, int64(0))
		assert.True(t, store.Exists(*entry.FilePath), "the file is referenced, not copied")
	}
}

func TestMigrateLegacyDocumentsSkipsAPathItAlreadyMigrated(t *testing.T) {
	store := newIdentityTestStorage(t)
	path, err := store.UploadFromBytes([]byte("%PDF-1.4"), "dni.pdf", "contracts")
	require.NoError(t, err)
	paths := fmt.Sprintf(`["%s"]`, filepath.ToSlash(path))

	repo := newFakeIdentityRepo()
	repo.legacyCount = 1
	repo.withLegacy = []models.Contract{{ID: 4, DocumentPaths: &paths}}
	svc := NewIdentityDocumentService(repo, &fakeContractRepo{}, store, nil, false)

	_, err = svc.MigrateLegacyDocuments(context.Background())
	require.NoError(t, err)
	require.Len(t, repo.created, 1)

	// A second run finds the row and creates nothing.
	result, err := svc.MigrateLegacyDocuments(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Skipped)
	assert.Equal(t, int64(0), result.Created)
	assert.Len(t, repo.created, 1)
}

func TestMigrateLegacyDocumentsKeepsGoingWhenAFileIsMissing(t *testing.T) {
	repo := newFakeIdentityRepo()
	repo.legacyCount = 1
	missing := "contracts/2025/01/borrado.pdf"
	paths := fmt.Sprintf(`["%s"]`, missing)
	repo.withLegacy = []models.Contract{{ID: 4, DocumentPaths: &paths}}
	svc := newIdentityTestService(t, repo, &fakeContractRepo{}, false)

	result, err := svc.MigrateLegacyDocuments(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(1), result.Missing)
	assert.Equal(t, int64(1), result.Created, "the row is still created so the loss is visible")
	require.Len(t, repo.created, 1)
	assert.Equal(t, missing, *repo.created[0].FilePath)
}

func TestLegacyDocumentPathsIgnoresAnEmptyColumn(t *testing.T) {
	assert.Empty(t, repository.LegacyDocumentPaths(&models.Contract{}))
	empty := ""
	assert.Empty(t, repository.LegacyDocumentPaths(&models.Contract{DocumentPaths: &empty}))
	broken := "{not json"
	assert.Empty(t, repository.LegacyDocumentPaths(&models.Contract{DocumentPaths: &broken}))
}
