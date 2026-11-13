package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// contractRepoStub is a configurable in-memory ContractRepository for handler
// tests. It records writes so the tests can assert what the service persisted.
type contractRepoStub struct {
	byID     *models.Contract
	byGUID   *models.Contract
	err      error
	list     []models.Contract
	total    int64
	stats    *repository.ContractStats
	updated  []*models.Contract
	deleted  []uint
	pending  []models.Contract
	awaiting []models.Contract
}

func (s *contractRepoStub) FindByID(context.Context, uint) (*models.Contract, error) {
	return s.byID, s.err
}

func (s *contractRepoStub) FindByIDWithDetails(context.Context, uint) (*models.Contract, error) {
	return s.byID, s.err
}

func (s *contractRepoStub) FindByGUIDWithDetails(context.Context, string) (*models.Contract, error) {
	return s.byGUID, s.err
}

func (s *contractRepoStub) FindByLot(context.Context, uint) ([]models.Contract, error) {
	return nil, s.err
}

func (s *contractRepoStub) FindByUser(context.Context, uint) ([]models.Contract, error) {
	return s.list, s.err
}

func (s *contractRepoStub) Create(_ context.Context, contract *models.Contract) error {
	if contract.ID == 0 {
		contract.ID = 1
	}
	return s.err
}

func (s *contractRepoStub) Update(_ context.Context, contract *models.Contract) error {
	s.updated = append(s.updated, contract)
	return s.err
}

func (s *contractRepoStub) Delete(_ context.Context, id uint) error {
	s.deleted = append(s.deleted, id)
	return s.err
}

func (s *contractRepoStub) List(_ context.Context, _ *repository.ContractQuery) ([]models.Contract, int64, error) {
	return s.list, s.total, s.err
}

func (s *contractRepoStub) FindActiveByLot(context.Context, uint) (*models.Contract, error) {
	return s.byID, s.err
}

func (s *contractRepoStub) FindPendingReservations(context.Context, int) ([]models.Contract, error) {
	return s.pending, s.err
}

func (s *contractRepoStub) FindReservationsAwaitingVoucher(context.Context, int) ([]models.Contract, error) {
	return s.awaiting, s.err
}

func (s *contractRepoStub) MarkReservationReminderSent(context.Context, uint) error {
	return s.err
}

func (s *contractRepoStub) MarkCommissionPaid(context.Context, uint, uint, float64) error {
	return s.err
}

func (s *contractRepoStub) GetStats(context.Context) (*repository.ContractStats, error) {
	return s.stats, s.err
}

func (s *contractRepoStub) HasActiveContracts(context.Context, uint) (bool, error) {
	return false, s.err
}

type paymentRepoStub struct {
	byContract        []models.Payment
	created           []*models.Payment
	deleted           []uint
	deletedByContract []uint
	err               error
}

func (s *paymentRepoStub) FindByID(context.Context, uint) (*models.Payment, error) {
	return nil, s.err
}

func (s *paymentRepoStub) FindByContract(context.Context, uint) ([]models.Payment, error) {
	return s.byContract, s.err
}

func (s *paymentRepoStub) Create(_ context.Context, payment *models.Payment) error {
	if payment.ID == 0 {
		payment.ID = uint(len(s.created) + 1)
	}
	s.created = append(s.created, payment)
	return s.err
}

func (s *paymentRepoStub) Update(context.Context, *models.Payment) error { return s.err }

func (s *paymentRepoStub) Delete(_ context.Context, id uint) error {
	s.deleted = append(s.deleted, id)
	return s.err
}

func (s *paymentRepoStub) DeleteByContract(_ context.Context, contractID uint) error {
	s.deletedByContract = append(s.deletedByContract, contractID)
	return s.err
}

func (s *paymentRepoStub) List(context.Context, *repository.ListQuery) ([]models.Payment, int64, error) {
	return nil, 0, s.err
}

func (s *paymentRepoStub) FindOverdue(context.Context) ([]models.Payment, error) {
	return nil, s.err
}

func (s *paymentRepoStub) FindOverdueForActiveContracts(context.Context) ([]models.Payment, error) {
	return nil, s.err
}

func (s *paymentRepoStub) FindPaymentsDueTomorrowForActiveContracts(context.Context) ([]models.Payment, error) {
	return nil, s.err
}

func (s *paymentRepoStub) MarkOverdueReminderSent(context.Context, []uint) error { return s.err }

func (s *paymentRepoStub) MarkUpcomingReminderSent(context.Context, []uint) error { return s.err }

func (s *paymentRepoStub) FindPendingByUser(context.Context, uint) ([]models.Payment, error) {
	return nil, s.err
}

func (s *paymentRepoStub) FindPaidByMonth(context.Context, int, int) ([]models.Payment, error) {
	return nil, s.err
}

func (s *paymentRepoStub) GetMonthlyStats(context.Context) (*repository.PaymentStats, error) {
	return nil, s.err
}

func (s *paymentRepoStub) FindByUserID(context.Context, uint) ([]models.Payment, error) {
	return nil, s.err
}

func (s *paymentRepoStub) BatchUpdateInterest(context.Context, map[uint]float64) error { return s.err }

type ledgerRepoStub struct {
	entries []models.ContractLedgerEntry
	balance float64
	err     error
	created []*models.ContractLedgerEntry
	deleted []uint
}

func (s *ledgerRepoStub) Create(_ context.Context, entry *models.ContractLedgerEntry) error {
	s.created = append(s.created, entry)
	return s.err
}

func (s *ledgerRepoStub) FindByContractID(context.Context, uint) ([]models.ContractLedgerEntry, error) {
	return s.entries, s.err
}

func (s *ledgerRepoStub) FindByPaymentID(context.Context, uint) ([]models.ContractLedgerEntry, error) {
	return nil, s.err
}

func (s *ledgerRepoStub) CalculateBalance(context.Context, uint) (float64, error) {
	return s.balance, s.err
}

func (s *ledgerRepoStub) FindOrCreateByPaymentAndType(context.Context, *models.ContractLedgerEntry) error {
	return s.err
}

func (s *ledgerRepoStub) BatchUpsertInterest(context.Context, []models.ContractLedgerEntry) error {
	return s.err
}

func (s *ledgerRepoStub) DeleteByContractID(_ context.Context, contractID uint) error {
	s.deleted = append(s.deleted, contractID)
	return s.err
}

type userRepoStub struct {
	user    *models.User
	err     error
	created []*models.User
	updated []*models.User
}

func (s *userRepoStub) FindByID(context.Context, uint) (*models.User, error) {
	return s.user, s.err
}

func (s *userRepoStub) FindByEmail(context.Context, string) (*models.User, error) {
	return s.user, s.err
}

func (s *userRepoStub) FindByIdentity(context.Context, string) (*models.User, error) {
	return s.user, s.err
}

func (s *userRepoStub) Create(_ context.Context, user *models.User) error {
	s.created = append(s.created, user)
	return s.err
}

func (s *userRepoStub) Update(_ context.Context, user *models.User) error {
	s.updated = append(s.updated, user)
	return s.err
}

func (s *userRepoStub) SetRecoveryCode(context.Context, uint, string, time.Time) error {
	return s.err
}

func (s *userRepoStub) Delete(context.Context, uint) error { return s.err }

func (s *userRepoStub) SoftDelete(context.Context, uint) error { return s.err }

func (s *userRepoStub) Restore(context.Context, uint) error { return s.err }

func (s *userRepoStub) CountByRole(context.Context) (map[string]int64, error) {
	return map[string]int64{}, s.err
}

func (s *userRepoStub) CountActiveClientsByCreator(context.Context, []uint) (map[uint]int64, error) {
	return map[uint]int64{}, s.err
}

func (s *userRepoStub) List(context.Context, *repository.ListQuery) ([]models.User, int64, error) {
	return nil, 0, s.err
}

func (s *userRepoStub) FindAdmins(context.Context) ([]models.User, error) { return nil, s.err }

func (s *userRepoStub) FindAll(context.Context) ([]models.User, error) { return nil, s.err }

func newContractHandlerForTest(t *testing.T) (*ContractHandler, *contractRepoStub, *paymentRepoStub, *ledgerRepoStub, *userRepoStub, sqlmock.Sqlmock) {
	t.Helper()

	contractRepo := &contractRepoStub{}
	paymentRepo := &paymentRepoStub{}
	ledgerRepo := &ledgerRepoStub{}
	userRepo := &userRepoStub{}

	db, mockSQL, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}

	worker := jobs.NewWorker(1)
	t.Cleanup(worker.Shutdown)

	service := services.NewContractService(
		contractRepo,
		&stubLotRepo{},
		userRepo,
		paymentRepo,
		ledgerRepo,
		services.NewNotificationService(nil, nil),
		services.NewEmailService(&config.Config{EnableEmailNotifications: false}),
		services.NewAuditService(gdb),
		worker,
	)

	return NewContractHandler(service, nil), contractRepo, paymentRepo, ledgerRepo, userRepo, mockSQL
}

func contractTestContext(method, body string, params gin.Params, userID uint, role string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = params
	context.Set("userID", userID)
	context.Set("userRole", role)
	return context, recorder
}

func decodeContractResponse(t *testing.T, recorder *httptest.ResponseRecorder) struct {
	Contract models.ContractResponse `json:"contract"`
	Message  string                  `json:"message"`
} {
	t.Helper()
	var payload struct {
		Contract models.ContractResponse `json:"contract"`
		Message  string                  `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v (body %s)", err, recorder.Body.String())
	}
	return payload
}

func TestContractHandler_ShowByGUID(t *testing.T) {
	contract := &models.Contract{
		ID:              7,
		GUID:            "contract-guid",
		Status:          models.ContractStatusApproved,
		ApplicantUserID: 5,
	}
	params := gin.Params{{Key: "guid", Value: "contract-guid"}}

	tests := []struct {
		name       string
		userID     uint
		role       string
		contract   *models.Contract
		err        error
		wantStatus int
	}{
		{name: "admin", userID: 1, role: "admin", contract: contract, wantStatus: http.StatusOK},
		{name: "seller", userID: 2, role: "seller", contract: contract, wantStatus: http.StatusOK},
		{name: "applicant", userID: 5, role: "user", contract: contract, wantStatus: http.StatusOK},
		{name: "unrelated user", userID: 99, role: "user", contract: contract, wantStatus: http.StatusForbidden},
		{name: "missing", userID: 1, role: "admin", err: errors.New("not found"), wantStatus: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
			contractRepo.byGUID = tc.contract
			contractRepo.err = tc.err

			c, w := contractTestContext(http.MethodGet, "", params, tc.userID, tc.role)
			handler.ShowByGUID(c)

			assert.Equal(t, tc.wantStatus, w.Code, w.Body.String())
			if tc.wantStatus == http.StatusOK {
				payload := decodeContractResponse(t, w)
				assert.Equal(t, contract.GUID, payload.Contract.GUID)
			}
		})
	}
}

func TestContractHandler_RequestPrepayment(t *testing.T) {
	contract := &models.Contract{
		ID:              7,
		GUID:            "contract-guid",
		Status:          models.ContractStatusApproved,
		ApplicantUserID: 5,
	}
	params := gin.Params{{Key: "guid", Value: "contract-guid"}}

	t.Run("success", func(t *testing.T) {
		handler, contractRepo, paymentRepo, ledgerRepo, _, _ := newContractHandlerForTest(t)
		contractRepo.byGUID = contract
		ledgerRepo.balance = -10000

		c, w := contractTestContext(http.MethodPost, `{"amount":2500}`, params, 5, "user")
		handler.RequestPrepayment(c)

		assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		assert.Len(t, paymentRepo.created, 1)
		assert.Equal(t, 2500.0, paymentRepo.created[0].Amount)
		assert.Equal(t, models.PaymentTypeCapitalRepayment, paymentRepo.created[0].PaymentType)
	})

	t.Run("non-positive amount", func(t *testing.T) {
		handler, contractRepo, paymentRepo, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byGUID = contract

		c, w := contractTestContext(http.MethodPost, `{"amount":0}`, params, 5, "user")
		handler.RequestPrepayment(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, paymentRepo.created)
	})

	t.Run("forbidden", func(t *testing.T) {
		handler, contractRepo, paymentRepo, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byGUID = contract

		c, w := contractTestContext(http.MethodPost, `{"amount":2500}`, params, 99, "user")
		handler.RequestPrepayment(c)

		assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		assert.Empty(t, paymentRepo.created)
	})

	t.Run("missing contract", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.err = errors.New("not found")

		c, w := contractTestContext(http.MethodPost, `{"amount":2500}`, params, 5, "user")
		handler.RequestPrepayment(c)

		assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	})
}

func TestContractHandler_Update(t *testing.T) {
	params := gin.Params{{Key: "contract_id", Value: "7"}}

	pendingContract := func() *models.Contract {
		return &models.Contract{
			ID:            7,
			Status:        models.ContractStatusPending,
			PaymentTerm:   12,
			FinancingType: models.FinancingTypeDirect,
		}
	}

	t.Run("flat body", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byID = pendingContract()

		c, w := contractTestContext(http.MethodPatch, `{"payment_term":6,"note":"hola"}`, params, 1, "admin")
		handler.Update(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Len(t, contractRepo.updated, 1)
		assert.Equal(t, 6, contractRepo.updated[0].PaymentTerm)
	})

	t.Run("nested body", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byID = pendingContract()

		c, w := contractTestContext(http.MethodPatch, `{"contract":{"payment_term":9,"note":"nested"}}`, params, 1, "admin")
		handler.Update(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Len(t, contractRepo.updated, 1)
		assert.Equal(t, 9, contractRepo.updated[0].PaymentTerm)
	})

	t.Run("schedule locked outside editable states", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{ID: 7, Status: models.ContractStatusApproved, PaymentTerm: 12}

		c, w := contractTestContext(http.MethodPatch, `{"payment_term":6}`, params, 1, "admin")
		handler.Update(c)

		assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		assert.Empty(t, contractRepo.updated)
	})

	t.Run("negative reserve amount", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byID = pendingContract()

		c, w := contractTestContext(http.MethodPatch, `{"reserve_amount":-1}`, params, 1, "admin")
		handler.Update(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, contractRepo.updated)
	})

	t.Run("invalid max payment date", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{ID: 7, Status: models.ContractStatusPending, FinancingType: models.FinancingTypeBank}

		c, w := contractTestContext(http.MethodPatch, `{"max_payment_date":"15/01/2099"}`, params, 1, "admin")
		handler.Update(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, contractRepo.updated)
	})

	t.Run("bank fields applied and cleared", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{ID: 7, Status: models.ContractStatusPending, FinancingType: models.FinancingTypeBank}

		c, w := contractTestContext(http.MethodPatch, `{"bank_name":"  BAC  ","bank_credit_status":" Approved "}`, params, 1, "admin")
		handler.Update(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Len(t, contractRepo.updated, 1)
		assert.NotNil(t, contractRepo.updated[0].BankName)
		assert.Equal(t, "BAC", *contractRepo.updated[0].BankName)
		assert.NotNil(t, contractRepo.updated[0].BankCreditStatus)
		assert.Equal(t, models.BankCreditStatusApproved, *contractRepo.updated[0].BankCreditStatus)

		handler, contractRepo, _, _, _, _ = newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{
			ID:               7,
			Status:           models.ContractStatusPending,
			FinancingType:    models.FinancingTypeBank,
			BankName:         stringPtrForTest("BAC"),
			BankCreditStatus: stringPtrForTest(models.BankCreditStatusApproved),
		}

		c, w = contractTestContext(http.MethodPatch, `{"bank_name":"","bank_credit_status":""}`, params, 1, "admin")
		handler.Update(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Len(t, contractRepo.updated, 1)
		assert.Nil(t, contractRepo.updated[0].BankName)
		assert.Nil(t, contractRepo.updated[0].BankCreditStatus)
	})
}

func stringPtrForTest(value string) *string { return &value }

func TestContractHandler_StateActions(t *testing.T) {
	audit := func(mockSQL sqlmock.Sqlmock) {
		mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	}
	params := gin.Params{
		{Key: "project_id", Value: "1"},
		{Key: "lot_id", Value: "1"},
		{Key: "contract_id", Value: "7"},
	}

	t.Run("approve", func(t *testing.T) {
		handler, contractRepo, _, _, _, mockSQL := newContractHandlerForTest(t)
		amount, down, reserve := 10000.0, 1000.0, 500.0
		contractRepo.byID = &models.Contract{
			ID:              7,
			LotID:           1,
			Status:          models.ContractStatusPending,
			Amount:          &amount,
			DownPayment:     &down,
			ReserveAmount:   &reserve,
			PaymentTerm:     2,
			FinancingType:   models.FinancingTypeDirect,
			ApplicantUserID: 5,
			Lot:             models.Lot{ID: 1, Name: "Lote 1"},
		}
		audit(mockSQL)

		c, w := contractTestContext(http.MethodPost, "", params, 1, "admin")
		handler.Approve(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		payload := decodeContractResponse(t, w)
		assert.Equal(t, models.ContractStatusApproved, payload.Contract.Status)
	})

	t.Run("approve invalid state", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		amount := 10000.0
		contractRepo.byID = &models.Contract{ID: 7, Status: models.ContractStatusApproved, Amount: &amount}

		c, w := contractTestContext(http.MethodPost, "", params, 1, "admin")
		handler.Approve(c)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	})

	t.Run("reject", func(t *testing.T) {
		handler, contractRepo, _, _, _, mockSQL := newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{ID: 7, LotID: 1, Status: models.ContractStatusPending, ApplicantUserID: 5}
		audit(mockSQL)

		c, w := contractTestContext(http.MethodPost, `{"reason":"Bad credit"}`, params, 1, "admin")
		handler.Reject(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		payload := decodeContractResponse(t, w)
		assert.Equal(t, models.ContractStatusRejected, payload.Contract.Status)
	})

	t.Run("cancel", func(t *testing.T) {
		handler, contractRepo, _, _, _, mockSQL := newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{ID: 7, LotID: 1, Status: models.ContractStatusSubmitted, ApplicantUserID: 5}
		audit(mockSQL)

		c, w := contractTestContext(http.MethodPost, `{"note":"User request"}`, params, 1, "admin")
		handler.Cancel(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		payload := decodeContractResponse(t, w)
		assert.Equal(t, models.ContractStatusCancelled, payload.Contract.Status)
	})

	t.Run("delete rejected", func(t *testing.T) {
		handler, contractRepo, paymentRepo, ledgerRepo, _, mockSQL := newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{ID: 7, LotID: 1, Status: models.ContractStatusRejected}
		audit(mockSQL)

		c, w := contractTestContext(http.MethodDelete, "", params, 1, "admin")
		handler.Delete(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, []uint{7}, contractRepo.deleted)
		assert.NotEmpty(t, paymentRepo.deletedByContract)
		assert.NotEmpty(t, ledgerRepo.deleted)
	})

	t.Run("delete only rejected", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{ID: 7, Status: models.ContractStatusPending}

		c, w := contractTestContext(http.MethodDelete, "", params, 1, "admin")
		handler.Delete(c)

		assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		assert.Empty(t, contractRepo.deleted)
	})

	t.Run("delete missing", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.err = errors.New("record not found")

		c, w := contractTestContext(http.MethodDelete, "", params, 1, "admin")
		handler.Delete(c)

		assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	})
}

func TestContractHandler_IndexStatsLedger(t *testing.T) {
	t.Run("index", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.list = []models.Contract{{
			ID:     7,
			GUID:   "contract-guid",
			Status: models.ContractStatusApproved,
			Amount: floatPtrForTest(10000),
			NextPayment: &models.Payment{
				ID:          33,
				ContractID:  7,
				Amount:      4500,
				PaymentType: models.PaymentTypeInstallment,
				Status:      models.PaymentStatusPending,
				Description: stringPtrForHandlerTest("Cuota 2 de 12"),
			},
		}}
		contractRepo.total = 21

		c, w := contractTestContext(http.MethodGet, "", nil, 1, "admin")
		handler.Index(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var payload struct {
			Contracts  []models.ContractResponse `json:"contracts"`
			Pagination struct {
				Page       int   `json:"page"`
				PerPage    int   `json:"per_page"`
				Total      int64 `json:"total"`
				TotalPages int64 `json:"total_pages"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		assert.Len(t, payload.Contracts, 1)
		assert.Equal(t, int64(21), payload.Pagination.Total)
		assert.Equal(t, int64(2), payload.Pagination.TotalPages)
		assert.Equal(t, 1, payload.Pagination.Page)
		assert.Equal(t, 20, payload.Pagination.PerPage)
		assert.Equal(t, "Cuota 2", payload.Contracts[0].NextPaymentLabel)
		if assert.NotNil(t, payload.Contracts[0].NextPayment) {
			assert.Equal(t, uint(33), payload.Contracts[0].NextPayment.ID)
			assert.Equal(t, "Cuota 2", payload.Contracts[0].NextPayment.Label)
		}
	})

	t.Run("stats", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.stats = &repository.ContractStats{Total: 5, Pending: 2, Approved: 2, Rejected: 1}

		c, w := contractTestContext(http.MethodGet, "", nil, 1, "admin")
		handler.GetStats(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var stats repository.ContractStats
		if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
			t.Fatalf("decode: %v", err)
		}
		assert.Equal(t, int64(5), stats.Total)
		assert.Equal(t, int64(2), stats.Pending)
	})

	t.Run("ledger", func(t *testing.T) {
		handler, contractRepo, _, ledgerRepo, _, _ := newContractHandlerForTest(t)
		contractRepo.byID = &models.Contract{ID: 7}
		ledgerRepo.entries = []models.ContractLedgerEntry{{
			ID: 1, ContractID: 7, Amount: -10000, EntryType: models.EntryTypeInitial, Description: "Monto inicial",
		}}

		c, w := contractTestContext(http.MethodGet, "", gin.Params{{Key: "contract_id", Value: "7"}}, 1, "admin")
		handler.Ledger(c)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var payload struct {
			LedgerEntries []models.ContractLedgerEntryResponse `json:"ledger_entries"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		assert.Len(t, payload.LedgerEntries, 1)
	})

	t.Run("ledger missing contract", func(t *testing.T) {
		handler, contractRepo, _, _, _, _ := newContractHandlerForTest(t)
		contractRepo.err = errors.New("not found")

		c, w := contractTestContext(http.MethodGet, "", gin.Params{{Key: "contract_id", Value: "7"}}, 1, "admin")
		handler.Ledger(c)

		assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	})
}

func floatPtrForTest(value float64) *float64 { return &value }

func stringPtrForHandlerTest(value string) *string { return &value }
