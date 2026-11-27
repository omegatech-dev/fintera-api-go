package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// commissionRepoStub is an in-memory CommissionRepository limited to the credit
// operations used by the commission handler tests.
type commissionRepoStub struct {
	repository.CommissionRepository
	credit      *models.SellerCommissionCredit
	settled     bool
	settings    *models.CommissionSettings
	sellerRates map[uint]float64
}

func (s *commissionRepoStub) GetSettings(_ context.Context) (*models.CommissionSettings, error) {
	if s.settings != nil {
		copy := *s.settings
		return &copy, nil
	}
	defaults := models.DefaultCommissionSettings()
	return &defaults, nil
}

func (s *commissionRepoStub) FindSellerRate(_ context.Context, sellerID uint) (*models.SellerCommissionRate, error) {
	if rate, ok := s.sellerRates[sellerID]; ok {
		return &models.SellerCommissionRate{SellerID: sellerID, Rate: rate}, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *commissionRepoStub) FindCreditByID(_ context.Context, id uint) (*models.SellerCommissionCredit, error) {
	if s.credit == nil || s.credit.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *s.credit
	return &copy, nil
}

func (s *commissionRepoStub) ListCredits(_ context.Context, sellerID uint) ([]models.SellerCommissionCredit, error) {
	if s.credit == nil {
		return nil, nil
	}
	if sellerID > 0 && s.credit.SellerID != sellerID {
		return nil, nil
	}
	return []models.SellerCommissionCredit{*s.credit}, nil
}

func (s *commissionRepoStub) SettleCredit(_ context.Context, _, actorID uint, settledAt time.Time) (bool, error) {
	if s.settled {
		return false, nil
	}
	s.settled = true
	s.credit.Status = models.CommissionCreditStatusSettled
	s.credit.AppliedAmount = s.credit.Amount
	s.credit.SettledAt = &settledAt
	s.credit.SettledBy = &actorID
	return true, nil
}

func (s *commissionRepoStub) ListAppliedTotalByContractIDs(context.Context, []uint) (map[uint]float64, error) {
	return map[uint]float64{}, nil
}

func (s *commissionRepoStub) ListApplicationsByCredits(_ context.Context, creditIDs []uint) (map[uint][]models.CreditApplication, error) {
	return map[uint][]models.CreditApplication{}, nil
}

func newCommissionTestRouter(handler *CommissionHandler, role string, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", role)
		c.Next()
	})
	router.POST("/commissions/credits/:id/apply", handler.ApplyCredit)
	router.GET("/commissions/credits", handler.ListCredits)
	router.GET("/commissions/rules", handler.ShowRules)
	return router
}

func pendingCommissionCredit() *models.SellerCommissionCredit {
	return &models.SellerCommissionCredit{
		ID:         5,
		SellerID:   7,
		ContractID: 11,
		Amount:     1500,
		Status:     models.CommissionCreditStatusPending,
		Seller:     &models.User{ID: 7, FullName: "Ana Vendedora"},
		Contract:   &models.Contract{ID: 11, GUID: "contract-guid"},
	}
}

func TestCommissionHandler_ApplyCredit(t *testing.T) {
	repo := &commissionRepoStub{credit: pendingCommissionCredit()}
	// A credit with no remaining balance can still be marked settled manually.
	repo.credit.AppliedAmount = repo.credit.Amount
	handler := NewCommissionHandler(services.NewCommissionService(repo, nil, nil))
	router := newCommissionTestRouter(handler, "admin", 3)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/commissions/credits/5/apply", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Credit  models.SellerCommissionCreditResponse `json:"credit"`
		Message string                                `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, models.CommissionCreditStatusSettled, body.Credit.Status)
	assert.NotNil(t, body.Credit.SettledAt)
	assert.Equal(t, uint(7), body.Credit.SellerID)
	assert.Equal(t, "Ana Vendedora", body.Credit.Seller)
	assert.Equal(t, "contract-guid", body.Credit.ContractGUID)
	assert.NotEmpty(t, body.Message)
}

func TestCommissionHandler_ApplyCreditWithBalanceReturns422(t *testing.T) {
	repo := &commissionRepoStub{credit: pendingCommissionCredit()}
	service := services.NewCommissionService(repo, nil, nil)
	handler := NewCommissionHandler(service)
	router := newCommissionTestRouter(handler, "admin", 3)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/commissions/credits/5/apply", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.False(t, repo.settled)
	assert.Equal(t, 0.0, repo.credit.AppliedAmount)
}

func TestCommissionHandler_ApplyCredit_NotFound(t *testing.T) {
	repo := &commissionRepoStub{credit: pendingCommissionCredit()}
	handler := NewCommissionHandler(services.NewCommissionService(repo, nil, nil))
	router := newCommissionTestRouter(handler, "admin", 3)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/commissions/credits/99/apply", nil)
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCommissionHandler_ApplyCredit_AlreadySettled(t *testing.T) {
	credit := pendingCommissionCredit()
	credit.Status = models.CommissionCreditStatusSettled
	repo := &commissionRepoStub{credit: credit, settled: true}
	handler := NewCommissionHandler(services.NewCommissionService(repo, nil, nil))
	router := newCommissionTestRouter(handler, "admin", 3)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/commissions/credits/5/apply", nil)
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCommissionHandler_ShowRulesIncludesSellerRate(t *testing.T) {
	repo := &commissionRepoStub{sellerRates: map[uint]float64{8: 6}}
	handler := NewCommissionHandler(services.NewCommissionService(repo, nil, nil))
	router := newCommissionTestRouter(handler, "seller", 8)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/commissions/rules", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Rules models.CommissionRulesResponse `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Rules.SellerRate)
	assert.InDelta(t, 6.0, *body.Rules.SellerRate, 0.01)
	assert.InDelta(t, 5.0, body.Rules.DefaultRate, 0.01)
}

func TestCommissionHandler_ShowRulesOmitsSellerRateForAdmin(t *testing.T) {
	repo := &commissionRepoStub{sellerRates: map[uint]float64{3: 6}}
	handler := NewCommissionHandler(services.NewCommissionService(repo, nil, nil))
	router := newCommissionTestRouter(handler, "admin", 3)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/commissions/rules", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Rules models.CommissionRulesResponse `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Nil(t, body.Rules.SellerRate)
}

func TestCommissionHandler_ListCreditsFiltersForSellers(t *testing.T) {
	repo := &commissionRepoStub{credit: pendingCommissionCredit()}
	handler := NewCommissionHandler(services.NewCommissionService(repo, nil, nil))
	router := newCommissionTestRouter(handler, "seller", 8)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/commissions/credits", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Credits []models.SellerCommissionCreditResponse `json:"credits"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Empty(t, body.Credits, "a seller must not see another seller's credits")
}
