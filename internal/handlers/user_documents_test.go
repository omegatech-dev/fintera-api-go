package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/stretchr/testify/assert"
)

// fakePDFGenerator returns a fixed buffer so the success path can be asserted
// without the real report templates.
type fakePDFGenerator struct {
	buf *bytes.Buffer
	err error
}

func (f *fakePDFGenerator) GenerateContractPDF(context.Context, uint) (*bytes.Buffer, error) {
	return f.buf, f.err
}

func (f *fakePDFGenerator) GenerateCustomerRecordPDF(context.Context, uint) (*bytes.Buffer, error) {
	return f.buf, f.err
}

func (f *fakePDFGenerator) GenerateUserBalancePDF(context.Context, uint) (*bytes.Buffer, error) {
	return f.buf, f.err
}

// mockDocContractRepo is a minimal ContractRepository for document handler
// tests. Only FindByID is exercised; the rest are inert stubs.
type mockDocContractRepo struct {
	contract *models.Contract
	err      error
}

func (m *mockDocContractRepo) FindByID(context.Context, uint) (*models.Contract, error) {
	return m.contract, m.err
}

func (m *mockDocContractRepo) FindByIDWithDetails(context.Context, uint) (*models.Contract, error) {
	return m.contract, m.err
}

func (m *mockDocContractRepo) FindByGUIDWithDetails(context.Context, string) (*models.Contract, error) {
	return nil, nil
}

func (m *mockDocContractRepo) FindByLot(context.Context, uint) ([]models.Contract, error) {
	return nil, nil
}

func (m *mockDocContractRepo) FindByUser(context.Context, uint) ([]models.Contract, error) {
	return nil, nil
}

func (m *mockDocContractRepo) Create(context.Context, *models.Contract) error { return nil }

func (m *mockDocContractRepo) Update(context.Context, *models.Contract) error { return nil }

func (m *mockDocContractRepo) Delete(context.Context, uint) error { return nil }

func (m *mockDocContractRepo) List(context.Context, *repository.ContractQuery) ([]models.Contract, int64, error) {
	return nil, 0, nil
}

func (m *mockDocContractRepo) FindActiveByLot(context.Context, uint) (*models.Contract, error) {
	return nil, nil
}

func (m *mockDocContractRepo) FindPendingReservations(context.Context, int) ([]models.Contract, error) {
	return nil, nil
}

func (m *mockDocContractRepo) FindReservationsAwaitingVoucher(context.Context, int) ([]models.Contract, error) {
	return nil, nil
}

func (m *mockDocContractRepo) MarkReservationReminderSent(context.Context, uint) error { return nil }

func (m *mockDocContractRepo) MarkCommissionPaid(context.Context, uint, uint, float64) error {
	return nil
}

func (m *mockDocContractRepo) GetStats(context.Context) (*repository.ContractStats, error) {
	return &repository.ContractStats{}, nil
}

func (m *mockDocContractRepo) HasActiveContracts(context.Context, uint) (bool, error) {
	return false, nil
}

func newDocTestHandler(repo repository.ContractRepository) *UserHandler {
	contractSvc := services.NewContractService(repo, nil, nil, nil, nil, nil, nil, nil, nil)
	return NewUserHandler(nil, nil, contractSvc, nil)
}

func newDocTestHandlerWithPDF(
	repo repository.ContractRepository,
	pdf documentPDFGenerator,
) *UserHandler {
	contractSvc := services.NewContractService(repo, nil, nil, nil, nil, nil, nil, nil, nil)
	return NewUserHandler(nil, nil, contractSvc, pdf)
}

func newDocGinContext(userID, contractID, role string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "user_id", Value: userID}}
	url := "/users/" + userID + "/document"
	if contractID != "" {
		url += "?contract_id=" + contractID
	}
	c.Request, _ = http.NewRequest(http.MethodGet, url, nil)
	if role != "" {
		c.Set("userRole", role)
	}
	return c, w
}

func TestAuthorizedContract_RequiresContractID(t *testing.T) {
	handler := newDocTestHandler(&mockDocContractRepo{})
	c, w := newDocGinContext("1", "", "user")

	_, ok := handler.authorizedContract(c)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthorizedContract_RejectsInvalidContractID(t *testing.T) {
	handler := newDocTestHandler(&mockDocContractRepo{})
	c, w := newDocGinContext("1", "abc", "user")

	_, ok := handler.authorizedContract(c)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthorizedContract_ReturnsNotFound(t *testing.T) {
	handler := newDocTestHandler(&mockDocContractRepo{err: errors.New("record not found")})
	c, w := newDocGinContext("1", "7", "user")

	_, ok := handler.authorizedContract(c)

	assert.False(t, ok)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthorizedContract_OwnerIsAllowed(t *testing.T) {
	repo := &mockDocContractRepo{contract: &models.Contract{ID: 7, ApplicantUserID: 1}}
	handler := newDocTestHandler(repo)
	c, w := newDocGinContext("1", "7", "user")

	contract, ok := handler.authorizedContract(c)

	assert.True(t, ok)
	assert.Equal(t, uint(7), contract.ID)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthorizedContract_ForeignContractIsForbidden(t *testing.T) {
	repo := &mockDocContractRepo{contract: &models.Contract{ID: 7, ApplicantUserID: 2}}
	handler := newDocTestHandler(repo)
	c, w := newDocGinContext("1", "7", "user")

	_, ok := handler.authorizedContract(c)

	assert.False(t, ok)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthorizedContract_StaffCanAccessAnyContract(t *testing.T) {
	for _, role := range []string{"admin", "seller"} {
		repo := &mockDocContractRepo{contract: &models.Contract{ID: 7, ApplicantUserID: 2}}
		handler := newDocTestHandler(repo)
		c, w := newDocGinContext("1", "7", role)

		contract, ok := handler.authorizedContract(c)

		assert.True(t, ok, "role %s should be allowed", role)
		assert.Equal(t, uint(7), contract.ID)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestPromiseContractPDF_ForeignContractIsForbidden(t *testing.T) {
	repo := &mockDocContractRepo{contract: &models.Contract{ID: 7, ApplicantUserID: 2}}
	handler := newDocTestHandler(repo)
	c, w := newDocGinContext("1", "7", "user")

	handler.PromiseContractPDF(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCustomerRecordPDF_ForeignContractIsForbidden(t *testing.T) {
	repo := &mockDocContractRepo{contract: &models.Contract{ID: 7, ApplicantUserID: 2}}
	handler := newDocTestHandler(repo)
	c, w := newDocGinContext("1", "7", "user")

	handler.CustomerRecordPDF(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestBalancePDF_RejectsInvalidUserID(t *testing.T) {
	handler := newDocTestHandler(&mockDocContractRepo{})
	c, w := newDocGinContext("abc", "", "user")

	handler.BalancePDF(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBalancePDF_RequiresReportService(t *testing.T) {
	handler := newDocTestHandler(&mockDocContractRepo{})
	c, w := newDocGinContext("1", "", "user")

	handler.BalancePDF(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDocumentPDFs_OwnerCanDownload(t *testing.T) {
	pdf := &fakePDFGenerator{buf: bytes.NewBufferString("%PDF-1.4")}
	repo := &mockDocContractRepo{contract: &models.Contract{ID: 7, ApplicantUserID: 1}}
	handler := newDocTestHandlerWithPDF(repo, pdf)

	cases := []struct {
		name     string
		call     func(*gin.Context)
		path     string
		contract string
	}{
		{"promise", handler.PromiseContractPDF, "/promise_contract_pdf", "7"},
		{"record", handler.CustomerRecordPDF, "/customer_record_pdf", "7"},
		{"balance", handler.BalancePDF, "/balance_pdf", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "user_id", Value: "1"}}
			url := "/users/1" + tc.path
			if tc.contract != "" {
				url += "?contract_id=" + tc.contract
			}
			c.Request, _ = http.NewRequest(http.MethodGet, url, nil)
			c.Set("userRole", "user")

			tc.call(c)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
			assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
		})
	}
}
