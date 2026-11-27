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
)

type commissionsContractRepo struct {
	repository.ContractRepository
	contracts []models.Contract
}

func (r *commissionsContractRepo) List(_ context.Context, query *repository.ContractQuery) ([]models.Contract, int64, error) {
	if !query.FilterBySeller {
		return r.contracts, int64(len(r.contracts)), nil
	}
	var own []models.Contract
	for _, contract := range r.contracts {
		if contract.SellerID != nil && *contract.SellerID == query.UserID {
			own = append(own, contract)
		}
	}
	return own, int64(len(own)), nil
}

// StreamForCommissions chunks this stub's List result so the batched
// GenerateCommissions path works with the existing fixtures.
func (r *commissionsContractRepo) StreamForCommissions(ctx context.Context, query *repository.ContractQuery, batchSize int, fn func(batch []models.Contract) error) error {
	contracts, _, err := r.List(ctx, query)
	if err != nil {
		return err
	}
	if batchSize <= 0 {
		batchSize = len(contracts)
	}
	for start := 0; start < len(contracts); start += batchSize {
		end := start + batchSize
		if end > len(contracts) {
			end = len(contracts)
		}
		if err := fn(contracts[start:end]); err != nil {
			return err
		}
	}
	return nil
}

type commissionsCreditRepo struct {
	repository.CommissionRepository
	credits []models.SellerCommissionCredit
}

func (r *commissionsCreditRepo) ListCredits(_ context.Context, sellerID uint) ([]models.SellerCommissionCredit, error) {
	result := []models.SellerCommissionCredit{}
	for _, credit := range r.credits {
		if sellerID == 0 || credit.SellerID == sellerID {
			result = append(result, credit)
		}
	}
	return result, nil
}

func (r *commissionsCreditRepo) ListPendingCreditsBySeller(context.Context) (map[uint]float64, error) {
	pending := map[uint]float64{}
	for _, credit := range r.credits {
		if credit.Status != models.CommissionCreditStatusSettled {
			pending[credit.SellerID] += credit.RemainingAmount()
		}
	}
	return pending, nil
}

func (r *commissionsCreditRepo) ListAppliedTotalByContractIDs(_ context.Context, creditIDs []uint) (map[uint]float64, error) {
	return map[uint]float64{}, nil
}

func (r *commissionsCreditRepo) ListApplicationsByCredits(_ context.Context, creditIDs []uint) (map[uint][]models.CreditApplication, error) {
	return map[uint][]models.CreditApplication{}, nil
}

// commissionsFixtureCredits returns the seller commission credits paired with
// commissionsFixture: Ana owns 1500 pending and 500 settled, Carlos 900 pending.
func commissionsFixtureCredits() []models.SellerCommissionCredit {
	ana := uint(7)
	carlos := uint(8)
	return []models.SellerCommissionCredit{
		{SellerID: ana, ContractID: 1, Amount: 1500, Status: models.CommissionCreditStatusPending, Seller: &models.User{ID: ana, FullName: "Ana Vendedora"}, Contract: &models.Contract{ID: 1, GUID: "credit-1"}},
		{SellerID: ana, ContractID: 2, Amount: 500, Status: models.CommissionCreditStatusSettled, Seller: &models.User{ID: ana, FullName: "Ana Vendedora"}, Contract: &models.Contract{ID: 2, GUID: "credit-2"}},
		{SellerID: carlos, ContractID: 3, Amount: 900, Status: models.CommissionCreditStatusPending, Seller: &models.User{ID: carlos, FullName: "Carlos Vendedor"}, Contract: &models.Contract{ID: 3, GUID: "credit-3"}},
	}
}

// commissionsFixture builds 29 commissions: 25 for seller 7 (Ana), 3 for
// seller 8 (Carlos) and 1 without seller. Five of Ana's are paid. Credits:
// Ana has 1500 pending and 500 settled, Carlos has 900 pending.
func commissionsFixture() *services.ReportService {
	approvedAt := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	paidAt := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	ana := uint(7)
	carlos := uint(8)

	var contracts []models.Contract
	for i := 0; i < 25; i++ {
		amount := 100000.0
		contract := models.Contract{
			ID:               uint(100 + i),
			GUID:             "ana-" + string(rune('a'+i)),
			Amount:           &amount,
			CommissionAmount: 1000,
			ApprovedAt:       &approvedAt,
			SellerID:         &ana,
			Seller:           &models.User{ID: ana, FullName: "Ana Vendedora"},
		}
		if i < 5 {
			contract.CommissionPaidAt = &paidAt
		}
		contracts = append(contracts, contract)
	}
	for i := 0; i < 3; i++ {
		amount := 200000.0
		contracts = append(contracts, models.Contract{
			ID:               uint(200 + i),
			GUID:             "carlos-" + string(rune('a'+i)),
			Amount:           &amount,
			CommissionAmount: 2000,
			ApprovedAt:       &approvedAt,
			SellerID:         &carlos,
			Seller:           &models.User{ID: carlos, FullName: "Carlos Vendedor"},
		})
	}
	unknownAmount := 50000.0
	contracts = append(contracts, models.Contract{
		ID:               300,
		GUID:             "unknown",
		Amount:           &unknownAmount,
		CommissionAmount: 500,
		ApprovedAt:       &approvedAt,
	})

	credits := commissionsFixtureCredits()

	service := services.NewReportService(nil, &commissionsContractRepo{contracts: contracts}, nil, nil)
	service.SetCommissionRepository(&commissionsCreditRepo{credits: credits})
	return service
}

func newCommissionsTestRouter(role string, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewReportHandler(commissionsFixture())

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", role)
		c.Next()
	})
	router.GET("/reports/commissions", handler.Commissions)
	return router
}

type commissionsPayload struct {
	Commissions []struct {
		ContractID uint  `json:"contract_id"`
		SellerID   *uint `json:"seller_id"`
	} `json:"commissions"`
	Pagination struct {
		Page        int `json:"page"`
		PerPage     int `json:"per_page"`
		Total       int `json:"total"`
		TotalPages  int `json:"total_pages"`
		PeriodTotal int `json:"period_total"`
	} `json:"pagination"`
	Totals struct {
		Commission  float64 `json:"commission"`
		Amount      float64 `json:"amount"`
		Paid        float64 `json:"paid"`
		Pending     float64 `json:"pending"`
		PaidCount   int     `json:"paid_count"`
		AverageRate float64 `json:"average_rate"`
		NetPending  float64 `json:"net_pending"`
	} `json:"totals"`
	Sellers []struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"sellers"`
	Credits           []map[string]any `json:"credits"`
	CreditsPagination struct {
		Page       int `json:"page"`
		PerPage    int `json:"per_page"`
		Total      int `json:"total"`
		TotalPages int `json:"total_pages"`
	} `json:"credits_pagination"`
	PendingCredits float64 `json:"pending_credits"`
	NetPending     float64 `json:"net_pending"`
}

func getCommissions(t *testing.T, router *gin.Engine, path string) (commissionsPayload, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var payload commissionsPayload
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload, recorder
}

func TestCommissionsDefaultPageIsCappedAt20(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?start_date=2026-03-01&end_date=2026-03-31")

	assert.Len(t, payload.Commissions, 20)
	assert.Equal(t, 1, payload.Pagination.Page)
	assert.Equal(t, 20, payload.Pagination.PerPage)
	assert.Equal(t, 29, payload.Pagination.Total)
	assert.Equal(t, 2, payload.Pagination.TotalPages)
}

func TestCommissionsPerPageAboveMaximumIsClamped(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?per_page=50")

	assert.Len(t, payload.Commissions, 20)
	assert.Equal(t, 20, payload.Pagination.PerPage)
}

func TestCommissionsPageBelowOneIsFirstPage(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?page=0&per_page=5")

	assert.Equal(t, 1, payload.Pagination.Page)
	assert.Len(t, payload.Commissions, 5)
}

func TestCommissionsPageBeyondLastIsEmpty(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?page=9")

	assert.Empty(t, payload.Commissions)
	assert.Equal(t, 9, payload.Pagination.Page)
	assert.Equal(t, 29, payload.Pagination.Total)
	assert.Equal(t, 2, payload.Pagination.TotalPages)
}

func TestCommissionsTotalsDoNotChangeBetweenPages(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	first, _ := getCommissions(t, router, "/reports/commissions?page=1")
	second, _ := getCommissions(t, router, "/reports/commissions?page=2")

	assert.Equal(t, first.Totals, second.Totals)
	assert.Equal(t, first.NetPending, second.NetPending)
	assert.Equal(t, 31500.0, first.Totals.Commission)
	assert.Equal(t, 3150000.0, first.Totals.Amount)
	assert.Equal(t, 5000.0, first.Totals.Paid)
	assert.Equal(t, 26500.0, first.Totals.Pending)
	assert.Equal(t, 5, first.Totals.PaidCount)
	assert.Equal(t, 1.0, first.Totals.AverageRate)
	assert.Equal(t, 2400.0, first.PendingCredits)
	assert.Equal(t, 24100.0, first.Totals.NetPending)
	assert.Equal(t, first.Totals.NetPending, first.NetPending)
}

func TestCommissionsSellerFilterUsesSellerID(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?seller_id=7")

	assert.Equal(t, 25, payload.Pagination.Total)
	for _, row := range payload.Commissions {
		require.NotNil(t, row.SellerID)
		assert.Equal(t, uint(7), *row.SellerID)
	}
	assert.Equal(t, 25000.0, payload.Totals.Commission)
	assert.Equal(t, 5000.0, payload.Totals.Paid)
	assert.Equal(t, 20000.0, payload.Totals.Pending)
	assert.Equal(t, 1500.0, payload.PendingCredits)
	assert.Equal(t, 18500.0, payload.NetPending)
	require.Len(t, payload.Credits, 2)
	assert.Equal(t, 2, payload.CreditsPagination.Total)
}

func TestCommissionsSellerWithoutRowsIsEmpty(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?seller_id=99")

	assert.Empty(t, payload.Commissions)
	assert.Equal(t, 0, payload.Pagination.Total)
	assert.Equal(t, 0, payload.Pagination.TotalPages)
	assert.Zero(t, payload.Totals.Commission)
	assert.Zero(t, payload.Totals.AverageRate)
	assert.Len(t, payload.Sellers, 2)
}

func TestCommissionsSellersListIgnoresSellerFilter(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?seller_id=8&per_page=1")

	require.Len(t, payload.Sellers, 2)
	assert.Equal(t, "Ana Vendedora", payload.Sellers[0].Name)
	assert.Equal(t, uint(7), payload.Sellers[0].ID)
	assert.Equal(t, "Carlos Vendedor", payload.Sellers[1].Name)
	assert.Equal(t, uint(8), payload.Sellers[1].ID)
}

func TestCommissionsSellerIDMustBeValid(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	for _, path := range []string{"/reports/commissions?seller_id=abc", "/reports/commissions?seller_id=0"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusBadRequest, recorder.Code, path)
	}
}

func TestCommissionsCreditsArePaginated(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?credits_per_page=2&credits_page=2")

	assert.Len(t, payload.Credits, 1)
	assert.Equal(t, 2, payload.CreditsPagination.Page)
	assert.Equal(t, 2, payload.CreditsPagination.PerPage)
	assert.Equal(t, 3, payload.CreditsPagination.Total)
	assert.Equal(t, 2, payload.CreditsPagination.TotalPages)
	assert.Equal(t, 2400.0, payload.PendingCredits)
}

func TestCommissionsCreditsCappedAt20(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?credits_per_page=100")

	assert.Equal(t, 20, payload.CreditsPagination.PerPage)
	assert.Len(t, payload.Credits, 3)
}

func TestCommissionsResponseHasAllKeys(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	_, recorder := getCommissions(t, router, "/reports/commissions")

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	for _, key := range []string{"commissions", "pagination", "totals", "sellers", "credits", "credits_pagination", "pending_credits", "net_pending"} {
		assert.Contains(t, body, key)
	}
}

func TestCommissionsSellerSeesOnlyOwnCredits(t *testing.T) {
	router := newCommissionsTestRouter("seller", 8)

	payload, _ := getCommissions(t, router, "/reports/commissions")

	assert.Equal(t, 3, payload.Pagination.Total)
	assert.Equal(t, 900.0, payload.PendingCredits)
	assert.Equal(t, 1, payload.CreditsPagination.Total)
}

func TestCommissionsSellerIgnoresForeignSellerID(t *testing.T) {
	router := newCommissionsTestRouter("seller", 8)

	payload, recorder := getCommissions(t, router, "/reports/commissions?seller_id=7&seller_id=abc")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 3, payload.Pagination.Total)
	for _, row := range payload.Commissions {
		require.NotNil(t, row.SellerID)
		assert.Equal(t, uint(8), *row.SellerID)
	}
	assert.Equal(t, 6000.0, payload.Totals.Pending)
	assert.Equal(t, 900.0, payload.PendingCredits)
	assert.Equal(t, 5100.0, payload.NetPending)
	assert.Equal(t, payload.Totals.Pending-payload.PendingCredits, payload.Totals.NetPending)
	require.Len(t, payload.Credits, 1)
	assert.Equal(t, float64(8), payload.Credits[0]["seller_id"])
	require.Len(t, payload.Sellers, 1)
	assert.Equal(t, uint(8), payload.Sellers[0].ID)
}

func TestCommissionsSearchFiltersPageAndPaginationOnly(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	baseline, _ := getCommissions(t, router, "/reports/commissions")
	payload, _ := getCommissions(t, router, "/reports/commissions?search=carlos")

	assert.Len(t, payload.Commissions, 3)
	assert.Equal(t, 3, payload.Pagination.Total)
	assert.Equal(t, 1, payload.Pagination.TotalPages)
	assert.Equal(t, 29, payload.Pagination.PeriodTotal)
	for _, row := range payload.Commissions {
		require.NotNil(t, row.SellerID)
		assert.Equal(t, uint(8), *row.SellerID)
	}
	assert.Equal(t, baseline.Totals, payload.Totals)
	assert.Equal(t, baseline.Sellers, payload.Sellers)
	assert.Equal(t, baseline.PendingCredits, payload.PendingCredits)
	assert.Equal(t, baseline.NetPending, payload.NetPending)
	assert.Equal(t, 31500.0, payload.Totals.Commission)
}

func TestCommissionsSearchMatchesContractWithAndWithoutHash(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	for _, term := range []string{"%23100", "100"} {
		payload, _ := getCommissions(t, router, "/reports/commissions?search="+term)

		require.Len(t, payload.Commissions, 1, term)
		assert.Equal(t, uint(100), payload.Commissions[0].ContractID, term)
		assert.Equal(t, 1, payload.Pagination.Total, term)
		assert.Equal(t, 29, payload.Pagination.PeriodTotal, term)
	}
}

func TestCommissionsEmptySearchDoesNotFilter(t *testing.T) {
	router := newCommissionsTestRouter("admin", 1)

	payload, _ := getCommissions(t, router, "/reports/commissions?search=%20%20")

	assert.Len(t, payload.Commissions, 20)
	assert.Equal(t, 29, payload.Pagination.Total)
	assert.Equal(t, 2, payload.Pagination.TotalPages)
	assert.Equal(t, 29, payload.Pagination.PeriodTotal)
}

func TestCommissionsPageHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		query       string
		wantPage    int
		wantPerPage int
	}{
		{query: "", wantPage: 1, wantPerPage: 20},
		{query: "page=0&per_page=0", wantPage: 1, wantPerPage: 20},
		{query: "page=-3&per_page=-1", wantPage: 1, wantPerPage: 20},
		{query: "page=abc&per_page=abc", wantPage: 1, wantPerPage: 20},
		{query: "page=4&per_page=7", wantPage: 4, wantPerPage: 7},
		{query: "per_page=21", wantPage: 1, wantPerPage: 20},
	}

	for _, test := range tests {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/?"+test.query, nil)

		page, perPage := parseCommissionPage(context, "page", "per_page")
		assert.Equal(t, test.wantPage, page, test.query)
		assert.Equal(t, test.wantPerPage, perPage, test.query)
	}
}

func TestPaginateRowsReturnsEmptyPageWhenNoRows(t *testing.T) {
	rows, meta := paginateRows([]int{}, 1, 20)

	assert.NotNil(t, rows)
	assert.Empty(t, rows)
	assert.Equal(t, 0, meta["total_pages"])
}
