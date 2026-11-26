package handlers

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handlerIdentityRepo is the in-memory IdentityDocumentRepository of the
// handler tests. The embedded interface keeps the unused methods inert.
type handlerIdentityRepo struct {
	repository.IdentityDocumentRepository

	docs        map[string]*models.IdentityDocument
	legacy      []models.ContractIdentityDocument
	personScope bool
	contractOK  bool
}

func newHandlerIdentityRepo() *handlerIdentityRepo {
	return &handlerIdentityRepo{docs: map[string]*models.IdentityDocument{}}
}

func handlerDocKey(userID uint, kind string) string {
	return fmt.Sprintf("%d:%s", userID, kind)
}

func (r *handlerIdentityRepo) FindByUserAndKind(_ context.Context, userID uint, kind string) (*models.IdentityDocument, error) {
	return r.docs[handlerDocKey(userID, kind)], nil
}

func (r *handlerIdentityRepo) FindByID(_ context.Context, id uint) (*models.IdentityDocument, error) {
	for _, doc := range r.docs {
		if doc.ID == id {
			return doc, nil
		}
	}
	return nil, nil
}

func (r *handlerIdentityRepo) FindByIDForViewer(_ context.Context, id, _ uint, isAdmin, isSeller bool) (*models.IdentityDocument, error) {
	if !isAdmin && !isSeller {
		return nil, nil
	}
	if !isAdmin && !r.personScope {
		return nil, nil
	}
	return r.FindByID(context.Background(), id)
}

func (r *handlerIdentityRepo) ListByUser(_ context.Context, userID uint) ([]models.IdentityDocument, error) {
	var result []models.IdentityDocument
	for _, kind := range []string{models.IdentityDocumentKindDNI, models.IdentityDocumentKindRTN} {
		if doc, ok := r.docs[handlerDocKey(userID, kind)]; ok {
			result = append(result, *doc)
		}
	}
	return result, nil
}

func (r *handlerIdentityRepo) ListLinksByContract(context.Context, uint) ([]models.ContractIdentityDocument, error) {
	return nil, nil
}

func (r *handlerIdentityRepo) ListLegacyByContract(context.Context, uint) ([]models.ContractIdentityDocument, error) {
	return r.legacy, nil
}

func (r *handlerIdentityRepo) Upsert(_ context.Context, doc *models.IdentityDocument) error {
	r.docs[handlerDocKey(doc.UserID, doc.Kind)] = doc
	return nil
}

func (r *handlerIdentityRepo) Delete(context.Context, uint) error { return nil }

func (r *handlerIdentityRepo) ReplaceLink(context.Context, *models.ContractIdentityDocument) error {
	return nil
}

func (r *handlerIdentityRepo) CreateLink(context.Context, *models.ContractIdentityDocument) error {
	return nil
}

func (r *handlerIdentityRepo) DeleteLinkForContractAndKind(context.Context, uint, string) error {
	return nil
}

func (r *handlerIdentityRepo) FindLegacyEntryForViewer(_ context.Context, contractID, entryID, _ uint, isAdmin, isSeller bool) (*models.ContractIdentityDocument, error) {
	if !isAdmin && (!isSeller || !r.contractOK) {
		return nil, nil
	}
	for i := range r.legacy {
		if r.legacy[i].ID == entryID && r.legacy[i].ContractID == contractID {
			return &r.legacy[i], nil
		}
	}
	return nil, nil
}

func (r *handlerIdentityRepo) HasContractInScope(context.Context, uint, uint) (bool, error) {
	return r.personScope, nil
}

func (r *handlerIdentityRepo) ContractInScope(context.Context, uint, uint) (bool, error) {
	return r.contractOK, nil
}

func (r *handlerIdentityRepo) CountContractsWithLegacyDocuments(context.Context) (int64, error) {
	return 0, nil
}

func (r *handlerIdentityRepo) ListContractsWithLegacyDocuments(context.Context) ([]models.Contract, error) {
	return nil, nil
}

func (r *handlerIdentityRepo) LegacyPathExists(context.Context, uint, string) (bool, error) {
	return false, nil
}

// handlerContractRepo answers FindByID and FindByGUIDWithDetails.
type handlerContractRepo struct {
	repository.ContractRepository

	contract *models.Contract
}

func (r *handlerContractRepo) FindByID(_ context.Context, id uint) (*models.Contract, error) {
	if r.contract == nil || r.contract.ID != id {
		return nil, services.ErrNotFound
	}
	return r.contract, nil
}

func (r *handlerContractRepo) FindByGUIDWithDetails(_ context.Context, guid string) (*models.Contract, error) {
	if r.contract == nil || r.contract.GUID != guid {
		return nil, services.ErrNotFound
	}
	return r.contract, nil
}

type identityTestEnv struct {
	handler *IdentityDocumentHandler
	repo    *handlerIdentityRepo
	store   *storage.LocalStorage
}

func newIdentityTestEnv(t *testing.T, contract *models.Contract, required bool) *identityTestEnv {
	t.Helper()
	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	repo := newHandlerIdentityRepo()
	svc := services.NewIdentityDocumentService(repo, &handlerContractRepo{contract: contract}, store, nil, required)
	return &identityTestEnv{handler: NewIdentityDocumentHandler(svc), repo: repo, store: store}
}

func identityCtx(t *testing.T, method, url, role string, userID uint, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(method, url, nil)
	c.Params = params
	if role != "" {
		c.Set("userRole", role)
	}
	if userID > 0 {
		c.Set("userID", userID)
	}
	return c, w
}

func tinyIdentityPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{B: 220, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// formFile is one file part of a multipart request.
type formFile struct {
	field       string
	name        string
	contentType string
	data        []byte
}

// formRequest builds a real multipart request so c.Request.FormFile works.
func formRequest(t *testing.T, method, url string, fields map[string]string, files ...formFile) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		require.NoError(t, writer.WriteField(name, value))
	}
	for _, file := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, file.field, file.name))
		header.Set("Content-Type", file.contentType)
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write(file.data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(method, url, &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestListByUserAllowsAnAdmin(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	c, w := identityCtx(t, http.MethodGet, "/api/v1/users/7/identity_documents", models.RoleAdmin, 1,
		gin.Params{{Key: "user_id", Value: "7"}})

	env.handler.ListByUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"slots"`)
	assert.Contains(t, w.Body.String(), `"kind":"dni"`)
	assert.Contains(t, w.Body.String(), `"kind":"rtn"`)
}

func TestListByUserAllowsASellerWithAContractInScope(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	env.repo.personScope = true
	c, w := identityCtx(t, http.MethodGet, "/api/v1/users/7/identity_documents", models.RoleSeller, 2,
		gin.Params{{Key: "user_id", Value: "7"}})

	env.handler.ListByUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListByUserRejectsASellerWithoutAContractInScope(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	env.repo.personScope = false
	c, w := identityCtx(t, http.MethodGet, "/api/v1/users/7/identity_documents", models.RoleSeller, 2,
		gin.Params{{Key: "user_id", Value: "7"}})

	env.handler.ListByUser(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListByUserRejectsTheContractOwner(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	// The owner did create the contract, so the scope query would match.
	env.repo.personScope = true
	c, w := identityCtx(t, http.MethodGet, "/api/v1/users/7/identity_documents", models.RoleUser, 7,
		gin.Params{{Key: "user_id", Value: "7"}})

	env.handler.ListByUser(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListByContractAllowsAnAdmin(t *testing.T) {
	env := newIdentityTestEnv(t, &models.Contract{ID: 4, GUID: "abc-123", ApplicantUserID: 7}, false)
	c, w := identityCtx(t, http.MethodGet, "/api/v1/contracts/abc-123/identity_documents", models.RoleAdmin, 1,
		gin.Params{{Key: "guid", Value: "abc-123"}})

	env.handler.ListByContract(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"contract_id":4`)
	assert.Contains(t, w.Body.String(), `"legacy":[]`)
}

func TestListByContractRejectsASellerWithoutScope(t *testing.T) {
	env := newIdentityTestEnv(t, &models.Contract{ID: 4, GUID: "abc-123", ApplicantUserID: 7}, false)
	env.repo.contractOK = false
	c, w := identityCtx(t, http.MethodGet, "/api/v1/contracts/abc-123/identity_documents", models.RoleSeller, 2,
		gin.Params{{Key: "guid", Value: "abc-123"}})

	env.handler.ListByContract(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListByContractAnswers404ForAnUnknownGUID(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	c, w := identityCtx(t, http.MethodGet, "/api/v1/contracts/nope/identity_documents", models.RoleAdmin, 1,
		gin.Params{{Key: "guid", Value: "nope"}})

	env.handler.ListByContract(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUploadForUserRejectsAnUnknownKind(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	c, w := identityCtx(t, http.MethodPost, "/api/v1/users/7/identity_documents", models.RoleAdmin, 1,
		gin.Params{{Key: "user_id", Value: "7"}})
	c.Request = formRequest(t, http.MethodPost, "/", map[string]string{"kind": "pasaporte"},
		formFile{"file", "p.png", "image/png", tinyIdentityPNG(t)})

	env.handler.UploadForUser(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_DOCUMENT_KIND")
}

func TestUploadForUserRejectsAnUnsupportedFileNamingIt(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	c, w := identityCtx(t, http.MethodPost, "/api/v1/users/7/identity_documents", models.RoleAdmin, 1,
		gin.Params{{Key: "user_id", Value: "7"}})
	c.Request = formRequest(t, http.MethodPost, "/", map[string]string{"kind": "dni"},
		formFile{"file", "dni.zip", "application/zip", []byte("PK\x03\x04")})

	env.handler.UploadForUser(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_DOCUMENT_TYPE")
	assert.Contains(t, w.Body.String(), "dni.zip")
}

func TestUploadForUserStoresTheDocument(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	c, w := identityCtx(t, http.MethodPost, "/api/v1/users/7/identity_documents", models.RoleAdmin, 1,
		gin.Params{{Key: "user_id", Value: "7"}})
	c.Request = formRequest(t, http.MethodPost, "/", map[string]string{"kind": "dni"},
		formFile{"file", "dni.png", "image/png", tinyIdentityPNG(t)})

	env.handler.UploadForUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"original_filename":"dni.png"`)
	require.NotNil(t, env.repo.docs[handlerDocKey(7, models.IdentityDocumentKindDNI)])
}

func TestUploadForContractStoresTheDocumentOfTheApplicant(t *testing.T) {
	env := newIdentityTestEnv(t, &models.Contract{ID: 4, GUID: "abc-123", ApplicantUserID: 7}, false)
	env.repo.contractOK = true
	c, w := identityCtx(t, http.MethodPost, "/api/v1/contracts/abc-123/identity_documents", models.RoleSeller, 2,
		gin.Params{{Key: "guid", Value: "abc-123"}})
	c.Request = formRequest(t, http.MethodPost, "/", map[string]string{"kind": "rtn"},
		formFile{"file", "rtn.png", "image/png", tinyIdentityPNG(t)})

	env.handler.UploadForContract(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, env.repo.docs[handlerDocKey(7, models.IdentityDocumentKindRTN)],
		"the document belongs to the applicant, not to the seller")
}

func TestContentServesTheOriginalAsAttachment(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	path, err := env.store.UploadFromBytes(tinyIdentityPNG(t), "dni.png", "identity_documents")
	require.NoError(t, err)
	env.repo.docs[handlerDocKey(7, models.IdentityDocumentKindDNI)] = &models.IdentityDocument{
		ID: 12, UserID: 7, Kind: models.IdentityDocumentKindDNI,
		FilePath: path, OriginalFilename: "dni.png", MimeType: "image/png", SizeBytes: 100,
	}

	c, w := identityCtx(t, http.MethodGet, "/api/v1/identity_documents/12/content", models.RoleAdmin, 1,
		gin.Params{{Key: "id", Value: "12"}})

	env.handler.Content(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
}

func TestPreviewServesTheWebPInline(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	previewPath, err := env.store.UploadFromBytes([]byte("RIFF....WEBP"), "preview.webp", "identity_documents/previews")
	require.NoError(t, err)
	env.repo.docs[handlerDocKey(7, models.IdentityDocumentKindDNI)] = &models.IdentityDocument{
		ID: 12, UserID: 7, Kind: models.IdentityDocumentKindDNI,
		FilePath: "identity_documents/2026/01/original.heic", PreviewPath: &previewPath,
		OriginalFilename: "dni.heic", MimeType: "image/heic",
	}

	c, w := identityCtx(t, http.MethodGet, "/api/v1/identity_documents/12/preview", models.RoleAdmin, 1,
		gin.Params{{Key: "id", Value: "12"}})

	env.handler.Preview(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/webp", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "inline")
}

func TestContentRejectsTheContractOwner(t *testing.T) {
	env := newIdentityTestEnv(t, nil, false)
	env.repo.personScope = true
	env.repo.docs[handlerDocKey(7, models.IdentityDocumentKindDNI)] = &models.IdentityDocument{
		ID: 12, UserID: 7, Kind: models.IdentityDocumentKindDNI, FilePath: "identity_documents/x.png",
	}
	c, w := identityCtx(t, http.MethodGet, "/api/v1/identity_documents/12/content", models.RoleUser, 7,
		gin.Params{{Key: "id", Value: "12"}})

	env.handler.Content(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLegacyContentServesTheMigratedFile(t *testing.T) {
	env := newIdentityTestEnv(t, &models.Contract{ID: 4, GUID: "abc-123", ApplicantUserID: 7}, false)
	path, err := env.store.UploadFromBytes([]byte("%PDF-1.4"), "adjunto.pdf", "contracts")
	require.NoError(t, err)
	name := "adjunto.pdf"
	mimeType := "application/pdf"
	env.repo.legacy = []models.ContractIdentityDocument{{
		ID: 9, ContractID: 4, Source: models.IdentityDocumentSourceLegacy,
		FilePath: &path, OriginalFilename: &name, MimeType: &mimeType,
	}}

	c, w := identityCtx(t, http.MethodGet, "/api/v1/contracts/abc-123/legacy_documents/9/content", models.RoleAdmin, 1,
		gin.Params{{Key: "guid", Value: "abc-123"}, {Key: "document_id", Value: "9"}})

	env.handler.LegacyContent(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
}

func TestLegacyContentAnswers404ForAnUnknownEntry(t *testing.T) {
	env := newIdentityTestEnv(t, &models.Contract{ID: 4, GUID: "abc-123", ApplicantUserID: 7}, false)
	c, w := identityCtx(t, http.MethodGet, "/api/v1/contracts/abc-123/legacy_documents/9/content", models.RoleAdmin, 1,
		gin.Params{{Key: "guid", Value: "abc-123"}, {Key: "document_id", Value: "9"}})

	env.handler.LegacyContent(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
