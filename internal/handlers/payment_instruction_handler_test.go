package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/stretchr/testify/mock"
)

type mockInstructionRepo struct {
	mock.Mock
}

func (m *mockInstructionRepo) Create(ctx context.Context, instruction *models.ProjectPaymentInstruction) error {
	args := m.Called(ctx, instruction)
	return args.Error(0)
}

func (m *mockInstructionRepo) FindByID(ctx context.Context, id uint) (*models.ProjectPaymentInstruction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ProjectPaymentInstruction), args.Error(1)
}

func (m *mockInstructionRepo) ListByProject(ctx context.Context, projectID uint) ([]models.ProjectPaymentInstruction, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]models.ProjectPaymentInstruction), args.Error(1)
}

func (m *mockInstructionRepo) Update(ctx context.Context, instruction *models.ProjectPaymentInstruction) error {
	args := m.Called(ctx, instruction)
	return args.Error(0)
}

func (m *mockInstructionRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockInstructionRepo) NextPosition(ctx context.Context, projectID uint) (int, error) {
	args := m.Called(ctx, projectID)
	return args.Int(0), args.Error(1)
}

func (m *mockInstructionRepo) ClearDefault(ctx context.Context, projectID, exceptID uint) error {
	args := m.Called(ctx, projectID, exceptID)
	return args.Error(0)
}

func (m *mockInstructionRepo) CountByProject(ctx context.Context, projectID uint) (int64, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(int64), args.Error(1)
}

type stubProjectRepo struct {
	project *models.Project
	err     error
}

func (s *stubProjectRepo) FindByID(ctx context.Context, id uint) (*models.Project, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.project, nil
}

func (s *stubProjectRepo) FindByGUID(ctx context.Context, guid string) (*models.Project, error) {
	return s.project, s.err
}

func (s *stubProjectRepo) FindPublishedByGUID(ctx context.Context, guid string) (*models.Project, error) {
	return s.project, s.err
}

func (s *stubProjectRepo) ListPublished(ctx context.Context) ([]models.Project, error) {
	return nil, s.err
}

func (s *stubProjectRepo) Create(ctx context.Context, project *models.Project) error { return s.err }

func (s *stubProjectRepo) Update(ctx context.Context, project *models.Project) error { return s.err }

func (s *stubProjectRepo) UpdateProjectAndLots(ctx context.Context, project *models.Project, lots []models.Lot) error {
	return s.err
}

func (s *stubProjectRepo) Delete(ctx context.Context, id uint) error { return s.err }

func (s *stubProjectRepo) List(ctx context.Context, query *repository.ListQuery) ([]models.Project, int64, error) {
	return nil, 0, s.err
}

func (s *stubProjectRepo) FindAll(ctx context.Context) ([]models.Project, error) {
	return nil, s.err
}

func newInstructionHandler(instructions repository.ProjectPaymentInstructionRepository) *PaymentInstructionHandler {
	service := services.NewProjectPaymentInstructionService(
		instructions,
		&stubProjectRepo{project: &models.Project{ID: 1}},
		nil,
	)
	return NewPaymentInstructionHandler(service)
}

func instructionContext(method, body string, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	return c, w
}

func TestPaymentInstructionHandlerIndex(t *testing.T) {
	repo := new(mockInstructionRepo)
	repo.On("ListByProject", mock.Anything, uint(1)).Return([]models.ProjectPaymentInstruction{
		{ID: 1, ProjectID: 1, BankName: "BAC Credomatic", BeneficiaryName: "Fintera", AccountNumber: "111", AccountType: "checking"},
		{ID: 2, ProjectID: 1, BankName: "Banco Atlántida", BeneficiaryName: "Fintera", AccountNumber: "222", AccountType: "savings"},
	}, nil)

	handler := newInstructionHandler(repo)
	c, w := instructionContext(http.MethodGet, "", gin.Params{{Key: "project_id", Value: "1"}})
	handler.Index(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var payload struct {
		PaymentInstructions []models.PaymentInstructionResponse `json:"payment_instructions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.PaymentInstructions) != 2 {
		t.Fatalf("instructions = %d, want 2", len(payload.PaymentInstructions))
	}
	repo.AssertExpectations(t)
}

func TestPaymentInstructionHandlerCreateValidation(t *testing.T) {
	cases := map[string]string{
		"missing account number": `{"bank_name":"BAC","beneficiary_name":"Fintera","account_type":"savings"}`,
		"invalid account type":   `{"bank_name":"BAC","beneficiary_name":"Fintera","account_number":"111","account_type":"credit"}`,
		"bank name too long":     `{"bank_name":"` + strings.Repeat("a", 121) + `","beneficiary_name":"Fintera","account_number":"111"}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			repo := new(mockInstructionRepo)
			handler := newInstructionHandler(repo)
			c, w := instructionContext(http.MethodPost, body, gin.Params{{Key: "project_id", Value: "1"}})
			handler.Create(c)

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
			}
			repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

func TestPaymentInstructionHandlerUpdateCrossProject(t *testing.T) {
	repo := new(mockInstructionRepo)
	repo.On("FindByID", mock.Anything, uint(9)).Return(&models.ProjectPaymentInstruction{
		ID: 9, ProjectID: 2, BankName: "BAC", BeneficiaryName: "Fintera", AccountNumber: "111", AccountType: "savings",
	}, nil)

	handler := newInstructionHandler(repo)
	c, w := instructionContext(http.MethodPatch, `{"bank_name":"Otro"}`, gin.Params{
		{Key: "project_id", Value: "1"},
		{Key: "instruction_id", Value: "9"},
	})
	handler.Update(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
}

func TestPaymentInstructionHandlerDeleteCrossProject(t *testing.T) {
	repo := new(mockInstructionRepo)
	repo.On("FindByID", mock.Anything, uint(9)).Return(&models.ProjectPaymentInstruction{
		ID: 9, ProjectID: 2, BankName: "BAC", BeneficiaryName: "Fintera", AccountNumber: "111", AccountType: "savings",
	}, nil)

	handler := newInstructionHandler(repo)
	c, w := instructionContext(http.MethodDelete, "", gin.Params{
		{Key: "project_id", Value: "1"},
		{Key: "instruction_id", Value: "9"},
	})
	handler.Delete(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}
