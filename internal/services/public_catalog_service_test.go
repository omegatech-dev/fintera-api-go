package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/mock"
)

type stubInstructionRepo struct {
	list []models.ProjectPaymentInstruction
}

func (s *stubInstructionRepo) Create(context.Context, *models.ProjectPaymentInstruction) error {
	return nil
}
func (s *stubInstructionRepo) FindByID(context.Context, uint) (*models.ProjectPaymentInstruction, error) {
	return nil, errors.New("not found")
}
func (s *stubInstructionRepo) ListByProject(context.Context, uint) ([]models.ProjectPaymentInstruction, error) {
	return s.list, nil
}
func (s *stubInstructionRepo) Update(context.Context, *models.ProjectPaymentInstruction) error {
	return nil
}
func (s *stubInstructionRepo) Delete(context.Context, uint) error { return nil }
func (s *stubInstructionRepo) NextPosition(context.Context, uint) (int, error) {
	return 0, nil
}

func publicTestProjects() []models.Project {
	city := "Tegucigalpa"
	code := "RTP-2025"
	createdAt := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	overridePrice := 45000.0
	photoAlt := "Vista"
	return []models.Project{
		{
			ID:                 1,
			GUID:               "project-1",
			Name:               "Residencial Los Pinos",
			Description:        "Proyecto residencial de prueba",
			ProjectType:        "residential",
			Address:            "Tegucigalpa",
			City:               &city,
			Code:               &code,
			CreatedAt:          createdAt,
			InterestRate:       12.5,
			CommissionRate:     3,
			PricePerSquareUnit: 150,
			MeasurementUnit:    "m2",
			IsPublished:        true,
			IsActive:           true,
			Lots: []models.Lot{
				{ID: 1, GUID: "lot-1", Name: "Lote 1", Status: models.LotStatusAvailable, Length: 10, Width: 20, Price: 30000},
				{ID: 2, GUID: "lot-2", Name: "Lote 2", Status: models.LotStatusReserved, Length: 10, Width: 20, Price: 40000, OverridePrice: &overridePrice},
				{ID: 3, GUID: "lot-3", Name: "Lote 3", Status: models.LotStatusFinanced, Length: 10, Width: 20, Price: 50000},
				{ID: 4, GUID: "lot-4", Name: "Lote 4", Status: models.LotStatusFullyPaid, Length: 10, Width: 20, Price: 60000},
				{ID: 5, GUID: "lot-5", Name: "Lote 5", Status: models.LotStatusActive, Length: 10, Width: 20, Price: 99999},
			},
			Photos: []models.ProjectPhoto{
				{FilePath: "uploads/projects/a.jpg", Alt: &photoAlt},
			},
			Drawings: []models.ProjectDrawing{
				{ID: 1, Name: "Plano general"},
				{ID: 2, Name: "Manzana A"},
			},
		},
	}
}

func TestPublicCatalogService_ListProjects(t *testing.T) {
	projectRepo := new(mockProjectRepo)
	projectRepo.On("ListPublished", mock.Anything).Return(publicTestProjects(), nil)
	service := NewPublicCatalogService(projectRepo, new(mockLotRepo), &stubInstructionRepo{})

	projects, err := service.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects = %d, want 1", len(projects))
	}

	project := projects[0]
	if project.Stats.TotalLots != 4 {
		t.Errorf("total_lots = %d, want 4 (inactive exclusion)", project.Stats.TotalLots)
	}
	if project.Stats.AvailableLots != 1 {
		t.Errorf("available_lots = %d, want 1", project.Stats.AvailableLots)
	}
	if project.Stats.ReservedLots != 1 {
		t.Errorf("reserved_lots = %d, want 1", project.Stats.ReservedLots)
	}
	if project.Stats.FinancedLots != 1 {
		t.Errorf("financed_lots = %d, want 1", project.Stats.FinancedLots)
	}
	if project.Stats.PaidLots != 1 {
		t.Errorf("paid_lots = %d, want 1", project.Stats.PaidLots)
	}
	if project.Stats.SoldLots != 2 {
		t.Errorf("sold_lots = %d, want 2", project.Stats.SoldLots)
	}
	if project.PriceFrom == nil || *project.PriceFrom != 30000 {
		t.Errorf("price_from = %v, want 30000", project.PriceFrom)
	}
	if project.PriceTo == nil || *project.PriceTo != 60000 {
		t.Errorf("price_to = %v, want 60000", project.PriceTo)
	}
	if project.DrawingCount != 2 {
		t.Errorf("drawing_count = %d, want 2", project.DrawingCount)
	}
	if project.City == nil || *project.City != "Tegucigalpa" {
		t.Errorf("city = %v, want Tegucigalpa", project.City)
	}
	if project.Code == nil || *project.Code != "RTP-2025" {
		t.Errorf("code = %v, want RTP-2025", project.Code)
	}
	if project.Description != "Proyecto residencial de prueba" {
		t.Errorf("description = %q", project.Description)
	}
	if project.CreatedAt.IsZero() {
		t.Error("created_at should be set")
	}
	if project.PhotoCount != 1 || len(project.Photos) != 1 {
		t.Errorf("photo_count = %d, photos = %d, want 1", project.PhotoCount, len(project.Photos))
	}

	// Public payloads must never leak admin-only fields.
	encoded, err := json.Marshal(project)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"interest_rate", "commission_rate", "uploaded_by", "reserved_by"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("public payload leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestPublicProjectResponseLegacyProject(t *testing.T) {
	project := &models.Project{
		GUID:        "legacy-project",
		Name:        "Proyecto legado",
		Description: "Sin ciudad ni código",
		ProjectType: "residential",
		Address:     "San Pedro Sula",
		IsPublished: true,
		IsActive:    true,
	}

	response := publicProjectResponse(project)

	if response.City != nil {
		t.Errorf("city = %v, want nil", response.City)
	}
	if response.Code != nil {
		t.Errorf("code = %v, want nil", response.Code)
	}
	if response.PriceFrom != nil || response.PriceTo != nil {
		t.Errorf("price range = %v-%v, want nil-nil", response.PriceFrom, response.PriceTo)
	}
	if response.DrawingCount != 0 {
		t.Errorf("drawing_count = %d, want 0", response.DrawingCount)
	}
	if response.Stats.TotalLots != 0 || response.Stats.AvailableLots != 0 {
		t.Errorf("stats = %+v, want zero", response.Stats)
	}
}

func TestPublicCatalogService_GetProjectNotFound(t *testing.T) {
	projectRepo := new(mockProjectRepo)
	projectRepo.On("FindPublishedByGUID", mock.Anything, "missing").Return(nil, errors.New("not found"))
	service := NewPublicCatalogService(projectRepo, new(mockLotRepo), &stubInstructionRepo{})

	if _, err := service.GetProject(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestPublicCatalogService_ListLotsAndPaymentInstructions(t *testing.T) {
	project := &publicTestProjects()[0]
	projectRepo := new(mockProjectRepo)
	projectRepo.On("FindPublishedByGUID", mock.Anything, "project-1").Return(project, nil)
	instructions := []models.ProjectPaymentInstruction{
		{ID: 1, ProjectID: 1, BankName: "Banco Atlántida", BeneficiaryName: "Fintera", AccountNumber: "123", AccountType: models.AccountTypeSavings},
	}
	service := NewPublicCatalogService(projectRepo, new(mockLotRepo), &stubInstructionRepo{list: instructions})

	lots, err := service.ListLots(context.Background(), "project-1", false)
	if err != nil {
		t.Fatalf("ListLots: %v", err)
	}
	if len(lots) != 4 {
		t.Fatalf("lots = %d, want 4 (inactive excluded)", len(lots))
	}

	publicInstructions, err := service.ListPaymentInstructions(context.Background(), "project-1")
	if err != nil {
		t.Fatalf("ListPaymentInstructions: %v", err)
	}
	if len(publicInstructions) != 1 || publicInstructions[0].BankName != "Banco Atlántida" {
		t.Fatalf("instructions = %#v", publicInstructions)
	}
	encoded, _ := json.Marshal(publicInstructions[0])
	for _, forbidden := range []string{"project_id", "position"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("public payment instruction leaked %q: %s", forbidden, encoded)
		}
	}
}

var _ repository.ProjectPaymentInstructionRepository = (*stubInstructionRepo)(nil)
