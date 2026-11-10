package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
	return []models.Project{
		{
			ID:                 1,
			GUID:               "project-1",
			Name:               "Residencial Los Pinos",
			Description:        "internal description",
			ProjectType:        "residential",
			Address:            "Tegucigalpa",
			InterestRate:       12.5,
			CommissionRate:     3,
			PricePerSquareUnit: 150,
			MeasurementUnit:    "m2",
			IsPublished:        true,
			IsActive:           true,
			Lots: []models.Lot{
				{ID: 1, GUID: "lot-1", Name: "Lote 1", Status: models.LotStatusAvailable, Length: 10, Width: 20, Price: 30000},
				{ID: 2, GUID: "lot-2", Name: "Lote 2", Status: models.LotStatusReserved, Length: 10, Width: 20, Price: 30000},
				{ID: 3, GUID: "lot-3", Name: "Lote 3", Status: models.LotStatusFinanced, Length: 10, Width: 20, Price: 30000},
				{ID: 4, GUID: "lot-4", Name: "Lote 4", Status: models.LotStatusActive, Length: 10, Width: 20, Price: 30000},
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
	if project.Stats.TotalLots != 3 {
		t.Errorf("total_lots = %d, want 3 (inactive exclusion)", project.Stats.TotalLots)
	}
	if project.Stats.AvailableLots != 1 {
		t.Errorf("available_lots = %d, want 1", project.Stats.AvailableLots)
	}
	if project.Stats.ReservedLots != 1 {
		t.Errorf("reserved_lots = %d, want 1", project.Stats.ReservedLots)
	}
	if project.Stats.SoldLots != 1 {
		t.Errorf("sold_lots = %d, want 1", project.Stats.SoldLots)
	}

	// Public payloads must never leak admin-only fields.
	encoded, err := json.Marshal(project)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"interest_rate", "commission_rate", "description"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("public payload leaked %q: %s", forbidden, encoded)
		}
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
	if len(lots) != 3 {
		t.Fatalf("lots = %d, want 3 (inactive excluded)", len(lots))
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
