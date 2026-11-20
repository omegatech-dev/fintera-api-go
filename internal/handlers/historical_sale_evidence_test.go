package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeEvidenceSaleRepository struct {
	repository.HistoricalSaleRepository
	sales []models.HistoricalSale
}

func (r *fakeEvidenceSaleRepository) Update(_ context.Context, sale *models.HistoricalSale) error {
	for i := range r.sales {
		if r.sales[i].ID == sale.ID {
			r.sales[i] = *sale
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeEvidenceSaleRepository) FindByID(_ context.Context, id uint) (*models.HistoricalSale, error) {
	for i := range r.sales {
		if r.sales[i].ID == id {
			copy := r.sales[i]
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeEvidenceSaleRepository) FindByIDWithDetails(ctx context.Context, id uint) (*models.HistoricalSale, error) {
	return r.FindByID(ctx, id)
}

func newEvidenceSaleHandler(t *testing.T) (*HistoricalSaleHandler, *fakeEvidenceSaleRepository, *storage.LocalStorage) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	repo := &fakeEvidenceSaleRepository{}
	service := services.NewHistoricalSaleService(repo, nil, nil, nil)
	return NewHistoricalSaleHandler(service, store), repo, store
}

func (r *fakeEvidenceSaleRepository) seedSale(id, createdBy uint, evidence *string) {
	r.sales = append(r.sales, models.HistoricalSale{
		ID:           id,
		ClientName:   "Cliente Histórico",
		SalePrice:    100000,
		CreatedBy:    createdBy,
		EvidenceFile: evidence,
	})
}

func evidenceUploadRequest(t *testing.T, id, actorID uint, role, filename, contentType string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if filename != "" {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
		header.Set("Content-Type", contentType)
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write([]byte("fake file content"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/historical_sales/"+strconv.FormatUint(uint64(id), 10)+"/evidence", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Set("userID", actorID)
	c.Set("userRole", role)
	return c, w
}

func evidenceDownloadRequest(t *testing.T, id, actorID uint, role, query string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	url := "/historical_sales/" + strconv.FormatUint(uint64(id), 10) + "/evidence"
	if query != "" {
		url += "?" + query
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, url, nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Set("userID", actorID)
	c.Set("userRole", role)
	return c, w
}

func evidenceStringPtr(value string) *string { return &value }

func TestUploadEvidenceSellerOwnRecord(t *testing.T) {
	handler, repo, store := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, nil)

	c, w := evidenceUploadRequest(t, 1, 4, "seller", "respaldo.png", "image/png")
	handler.UploadEvidence(c)

	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		HistoricalSale models.HistoricalSaleResponse `json:"historical_sale"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.NotNil(t, payload.HistoricalSale.EvidenceFile)
	assert.True(t, store.Exists(*payload.HistoricalSale.EvidenceFile))
}

func TestUploadEvidenceSellerOtherRecordForbidden(t *testing.T) {
	handler, repo, _ := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, nil)

	c, w := evidenceUploadRequest(t, 1, 5, "seller", "respaldo.png", "image/png")
	handler.UploadEvidence(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUploadEvidenceAdminAnyRecord(t *testing.T) {
	handler, repo, store := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, nil)

	c, w := evidenceUploadRequest(t, 1, 1, "admin", "respaldo.pdf", "application/pdf")
	handler.UploadEvidence(c)

	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		HistoricalSale models.HistoricalSaleResponse `json:"historical_sale"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.NotNil(t, payload.HistoricalSale.EvidenceFile)
	assert.True(t, store.Exists(*payload.HistoricalSale.EvidenceFile))
}

func TestUploadEvidenceRejectsUnsupportedType(t *testing.T) {
	handler, repo, _ := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, nil)

	c, w := evidenceUploadRequest(t, 1, 4, "seller", "respaldo.exe", "application/octet-stream")
	handler.UploadEvidence(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Nil(t, repo.sales[0].EvidenceFile)
}

func TestUploadEvidenceMissingFile(t *testing.T) {
	handler, repo, _ := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, nil)

	c, w := evidenceUploadRequest(t, 1, 4, "seller", "", "")
	handler.UploadEvidence(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Nil(t, repo.sales[0].EvidenceFile)
}

func TestUploadEvidenceRejectsOversizedFile(t *testing.T) {
	handler, repo, _ := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, nil)

	// A body larger than the 10 MB limit makes the multipart reader fail.
	oversized := bytes.Repeat([]byte("a"), int(storage.MaxFileSize())+1024)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="grande.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(oversized)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/historical_sales/1/evidence", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set("userID", uint(4))
	c.Set("userRole", "seller")
	handler.UploadEvidence(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Nil(t, repo.sales[0].EvidenceFile)
}

func TestDownloadEvidenceInline(t *testing.T) {
	handler, repo, store := newEvidenceSaleHandler(t)
	path, err := store.UploadFromBytes([]byte("png-data"), "respaldo.png", "evidence")
	require.NoError(t, err)
	repo.seedSale(1, 4, &path)

	c, w := evidenceDownloadRequest(t, 1, 1, "admin", "")
	handler.DownloadEvidence(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "inline")
	assert.Equal(t, "png-data", w.Body.String())
}

func TestDownloadEvidenceAsAttachment(t *testing.T) {
	handler, repo, store := newEvidenceSaleHandler(t)
	path, err := store.UploadFromBytes([]byte("png-data"), "respaldo.png", "evidence")
	require.NoError(t, err)
	repo.seedSale(1, 4, &path)

	c, w := evidenceDownloadRequest(t, 1, 4, "seller", "download=1")
	handler.DownloadEvidence(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
}

func TestDownloadEvidenceWithoutFile(t *testing.T) {
	handler, repo, _ := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, nil)

	c, w := evidenceDownloadRequest(t, 1, 1, "admin", "")
	handler.DownloadEvidence(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDownloadEvidenceExternalURL(t *testing.T) {
	handler, repo, _ := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, evidenceStringPtr("https://example.com/respaldo.jpg"))

	c, w := evidenceDownloadRequest(t, 1, 1, "admin", "")
	handler.DownloadEvidence(c)

	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		EvidenceURL string `json:"evidence_url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.Equal(t, "https://example.com/respaldo.jpg", payload.EvidenceURL)
}

func TestDownloadEvidenceMissingOnDisk(t *testing.T) {
	handler, repo, _ := newEvidenceSaleHandler(t)
	repo.seedSale(1, 4, evidenceStringPtr("evidence/2020/01/gone.png"))

	c, w := evidenceDownloadRequest(t, 1, 1, "admin", "")
	handler.DownloadEvidence(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDownloadEvidenceSellerOtherRecordForbidden(t *testing.T) {
	handler, repo, store := newEvidenceSaleHandler(t)
	path, err := store.UploadFromBytes([]byte("png-data"), "respaldo.png", "evidence")
	require.NoError(t, err)
	repo.seedSale(1, 4, &path)

	c, w := evidenceDownloadRequest(t, 1, 5, "seller", "")
	handler.DownloadEvidence(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
