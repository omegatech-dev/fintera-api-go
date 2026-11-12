package services

import (
	"context"
	"math"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Repositories

type mockProjectRepo struct {
	mock.Mock
}

func (m *mockProjectRepo) FindByID(ctx context.Context, id uint) (*models.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *mockProjectRepo) FindByGUID(ctx context.Context, guid string) (*models.Project, error) {
	return nil, nil
}

func (m *mockProjectRepo) FindPublishedByGUID(ctx context.Context, guid string) (*models.Project, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *mockProjectRepo) ListPublished(ctx context.Context) ([]models.Project, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Project), args.Error(1)
}

func (m *mockProjectRepo) List(ctx context.Context, query *repository.ListQuery) ([]models.Project, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]models.Project), args.Get(1).(int64), args.Error(2)
}

func (m *mockProjectRepo) FindAll(ctx context.Context) ([]models.Project, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Project), args.Error(1)
}

func (m *mockProjectRepo) Create(ctx context.Context, project *models.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *mockProjectRepo) Update(ctx context.Context, project *models.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *mockProjectRepo) UpdateProjectAndLots(ctx context.Context, project *models.Project, lots []models.Lot) error {
	args := m.Called(ctx, project, lots)
	return args.Error(0)
}

func (m *mockProjectRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockLotRepo struct {
	mock.Mock
}

func (m *mockLotRepo) FindByID(ctx context.Context, id uint) (*models.Lot, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Lot), args.Error(1)
}

func (m *mockLotRepo) ReserveIfAvailable(ctx context.Context, id uint) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *mockLotRepo) FindByGUID(ctx context.Context, guid string) (*models.Lot, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Lot), args.Error(1)
}

func (m *mockLotRepo) FindByProjectAndGUID(ctx context.Context, projectID uint, guid string) (*models.Lot, error) {
	args := m.Called(ctx, projectID, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Lot), args.Error(1)
}

func (m *mockLotRepo) FindByProject(ctx context.Context, projectID uint) ([]models.Lot, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]models.Lot), args.Error(1)
}

func (m *mockLotRepo) List(ctx context.Context, projectID uint, query *repository.ListQuery) ([]models.Lot, int64, error) {
	args := m.Called(ctx, projectID, query)
	return args.Get(0).([]models.Lot), args.Get(1).(int64), args.Error(2)
}

func (m *mockLotRepo) Create(ctx context.Context, lot *models.Lot) error {
	args := m.Called(ctx, lot)
	return args.Error(0)
}

func (m *mockLotRepo) Update(ctx context.Context, lot *models.Lot) error {
	args := m.Called(ctx, lot)
	return args.Error(0)
}

func (m *mockLotRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockLotRepo) BulkUpdate(ctx context.Context, lots []models.Lot) error {
	args := m.Called(ctx, lots)
	return args.Error(0)
}

func TestProjectService_Create(t *testing.T) {
	ctx := context.Background()
	mockProjRepo := new(mockProjectRepo)
	mockLotRepo := new(mockLotRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewProjectService(mockProjRepo, mockLotRepo, auditSvc)

	project := &models.Project{
		Name:               "Test Project",
		LotCount:           2,
		PricePerSquareUnit: 100,
		MeasurementUnit:    "m2",
	}

	mockProjRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Project")).Return(nil)

	err := service.Create(ctx, project, 1)

	assert.NoError(t, err)
	assert.NotEmpty(t, project.GUID)
	assert.Len(t, project.Lots, 2)
	assert.Equal(t, "Lote 001", project.Lots[0].Name)
	assert.Equal(t, models.LotStatusAvailable, project.Lots[0].Status)
	assert.Equal(t, 20000.0, project.Lots[0].Price) // 20 * 10 * 100
	assert.Equal(t, "m2", *project.Lots[0].MeasurementUnit)

	mockProjRepo.AssertExpectations(t)
}

func TestProjectService_SetProjectPublished(t *testing.T) {
	ctx := context.Background()
	mockProjRepo := new(mockProjectRepo)
	mockLotRepo := new(mockLotRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	service := NewProjectService(mockProjRepo, mockLotRepo, NewAuditService(gdb))

	project := &models.Project{
		ID:          1,
		Name:        "Altos",
		IsPublished: false,
		IsActive:    true,
	}
	mockProjRepo.On("FindByID", mock.Anything, uint(1)).Return(project, nil)
	mockProjRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Project")).Return(nil)

	published, err := service.SetProjectPublished(ctx, 1, 9, true)

	assert.NoError(t, err)
	assert.True(t, published.IsPublished)
	assert.True(t, published.IsActive)
	mockProjRepo.AssertExpectations(t)
}

func TestProjectService_SetProjectPublishedNoop(t *testing.T) {
	ctx := context.Background()
	mockProjRepo := new(mockProjectRepo)
	mockLotRepo := new(mockLotRepo)

	service := NewProjectService(mockProjRepo, mockLotRepo, nil)

	project := &models.Project{ID: 1, IsPublished: true, IsActive: true}
	mockProjRepo.On("FindByID", mock.Anything, uint(1)).Return(project, nil)

	result, err := service.SetProjectPublished(ctx, 1, 9, true)

	assert.NoError(t, err)
	assert.True(t, result.IsPublished)
	mockProjRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestProjectService_Update(t *testing.T) {
	ctx := context.Background()
	mockProjRepo := new(mockProjectRepo)
	mockLotRepo := new(mockLotRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewProjectService(mockProjRepo, mockLotRepo, auditSvc)

	existingProject := &models.Project{
		ID:                 1,
		GUID:               "existing-guid",
		MeasurementUnit:    "m2",
		PricePerSquareUnit: 100,
	}

	updatedProject := &models.Project{
		ID:                 1,
		MeasurementUnit:    "ft2",
		PricePerSquareUnit: 100, // unchanged: the service converts it
	}

	mu := "m2"
	overrideArea := 200.0
	overridePrice := 50000.0
	lots := []models.Lot{
		{
			ID:              1,
			Status:          models.LotStatusAvailable,
			Length:          20,
			Width:           10,
			Price:           20000,
			MeasurementUnit: &mu,
		},
		{
			ID:              2,
			Status:          models.LotStatusReserved,
			Length:          20,
			Width:           10,
			Price:           20000,
			MeasurementUnit: &mu,
		},
		{
			ID:              3,
			Status:          models.LotStatusAvailable,
			Length:          20,
			Width:           10,
			Price:           50000,
			OverridePrice:   &overridePrice,
			OverrideArea:    &overrideArea,
			MeasurementUnit: &mu,
		},
	}

	mockProjRepo.On("FindByID", ctx, uint(1)).Return(existingProject, nil)
	mockLotRepo.On("FindByProject", ctx, uint(1)).Return(lots, nil)
	mockProjRepo.On("UpdateProjectAndLots", ctx,
		mock.MatchedBy(func(p *models.Project) bool {
			return p.MeasurementUnit == "ft2" && math.Abs(p.PricePerSquareUnit-9.29) < 0.001
		}),
		mock.MatchedBy(func(updated []models.Lot) bool {
			if len(updated) != 3 {
				return false
			}
			first := updated[0]
			second := updated[1]
			third := updated[2]
			return *first.MeasurementUnit == "ft2" &&
				math.Abs(first.Length-65.62) < 0.01 &&
				math.Abs(first.Width-32.81) < 0.01 &&
				*second.MeasurementUnit == "ft2" &&
				second.Price == 20000 &&
				third.OverrideArea != nil &&
				math.Abs(*third.OverrideArea-2152.78) < 0.01 &&
				third.Price == 50000
		}),
	).Return(nil)

	err := service.Update(ctx, updatedProject, 1)

	assert.NoError(t, err)
	assert.Equal(t, "existing-guid", updatedProject.GUID)
	assert.Equal(t, "ft2", updatedProject.MeasurementUnit)

	mockProjRepo.AssertExpectations(t)
	mockLotRepo.AssertExpectations(t)
}

func TestProjectService_UpdateGeneratesGUIDForLegacyProject(t *testing.T) {
	ctx := context.Background()
	mockProjRepo := new(mockProjectRepo)
	mockLotRepo := new(mockLotRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	service := NewProjectService(mockProjRepo, mockLotRepo, NewAuditService(gdb))

	existingProject := &models.Project{
		ID:                 1,
		GUID:               "",
		MeasurementUnit:    "m2",
		PricePerSquareUnit: 100,
	}
	updatedProject := &models.Project{
		ID:                 1,
		MeasurementUnit:    "m2",
		PricePerSquareUnit: 100,
	}

	mockProjRepo.On("FindByID", ctx, uint(1)).Return(existingProject, nil)
	mockProjRepo.On("Update", ctx, mock.AnythingOfType("*models.Project")).Return(nil)

	if err := service.Update(ctx, updatedProject, 1); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assert.NotEmpty(t, updatedProject.GUID)

	mockProjRepo.AssertExpectations(t)
}

func TestProjectService_UpdatePricePerUnit(t *testing.T) {
	ctx := context.Background()
	mockProjRepo := new(mockProjectRepo)
	mockLotRepo := new(mockLotRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewProjectService(mockProjRepo, mockLotRepo, auditSvc)

	existingProject := &models.Project{
		ID:                 1,
		GUID:               "existing-guid",
		MeasurementUnit:    "m2",
		PricePerSquareUnit: 100,
	}
	updatedProject := &models.Project{
		ID:                 1,
		MeasurementUnit:    "m2",
		PricePerSquareUnit: 150,
	}
	mu := "m2"
	lots := []models.Lot{
		{ID: 1, Status: models.LotStatusAvailable, Length: 20, Width: 10, Price: 20000, MeasurementUnit: &mu},
		{ID: 2, Status: models.LotStatusReserved, Length: 20, Width: 10, Price: 20000, MeasurementUnit: &mu},
	}

	mockProjRepo.On("FindByID", ctx, uint(1)).Return(existingProject, nil)
	mockLotRepo.On("FindByProject", ctx, uint(1)).Return(lots, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.ID == 1 && l.Price == 30000
	})).Return(nil)
	mockProjRepo.On("Update", ctx, updatedProject).Return(nil)

	err := service.Update(ctx, updatedProject, 1)

	assert.NoError(t, err)
	mockProjRepo.AssertExpectations(t)
	mockLotRepo.AssertExpectations(t)
}

func TestLotService_Create(t *testing.T) {
	ctx := context.Background()
	mockLotRepo := new(mockLotRepo)
	mockProjRepo := new(mockProjectRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewLotService(mockLotRepo, mockProjRepo, auditSvc)

	lot := &models.Lot{
		Name:      "New Lot",
		ProjectID: 1,
	}

	project := &models.Project{
		ID:              1,
		LotCount:        5,
		Name:            "Project 1",
		MeasurementUnit: "ft2",
	}

	mockLotRepo.On("Create", ctx, lot).Return(nil)
	mockProjRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	// Expect lot count to be incremented
	mockProjRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.LotCount == 6
	})).Return(nil)

	err := service.Create(ctx, lot, 1)

	assert.NoError(t, err)
	assert.NotNil(t, lot.MeasurementUnit)
	assert.Equal(t, "ft2", *lot.MeasurementUnit)

	mockLotRepo.AssertExpectations(t)
	mockProjRepo.AssertExpectations(t)
}

func TestLotService_UpdatePreservesGUID(t *testing.T) {
	ctx := context.Background()
	mockLotRepo := new(mockLotRepo)
	mockProjRepo := new(mockProjectRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	service := NewLotService(mockLotRepo, mockProjRepo, NewAuditService(gdb))

	unit := "m2"
	existingLot := &models.Lot{
		ID:              1,
		GUID:            "existing-lot-guid",
		ProjectID:       2,
		Status:          models.LotStatusAvailable,
		MeasurementUnit: &unit,
		Project:         models.Project{ID: 2, MeasurementUnit: unit},
	}
	updatedLot := &models.Lot{
		ID:     1,
		Name:   "Updated Lot",
		Length: 10,
		Width:  20,
		Price:  2000,
	}

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(existingLot, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(lot *models.Lot) bool {
		return lot.GUID == "existing-lot-guid"
	})).Return(nil)

	err := service.Update(ctx, updatedLot, 7)

	assert.NoError(t, err)
	assert.Equal(t, "existing-lot-guid", updatedLot.GUID)

	mockLotRepo.AssertExpectations(t)
}

func TestLotService_UpdateHealsEmptyGUID(t *testing.T) {
	ctx := context.Background()
	mockLotRepo := new(mockLotRepo)
	mockProjRepo := new(mockProjectRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	service := NewLotService(mockLotRepo, mockProjRepo, NewAuditService(gdb))

	unit := "m2"
	existingLot := &models.Lot{
		ID:              1,
		GUID:            "",
		ProjectID:       2,
		Status:          models.LotStatusAvailable,
		MeasurementUnit: &unit,
		Project:         models.Project{ID: 2, MeasurementUnit: unit},
	}
	updatedLot := &models.Lot{
		ID:     1,
		Name:   "Updated Lot",
		Length: 10,
		Width:  20,
		Price:  2000,
	}

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(existingLot, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(lot *models.Lot) bool {
		return lot.GUID != ""
	})).Return(nil)

	err := service.Update(ctx, updatedLot, 7)

	assert.NoError(t, err)
	assert.NotEmpty(t, updatedLot.GUID)

	mockLotRepo.AssertExpectations(t)
}

func TestLotService_Delete(t *testing.T) {
	ctx := context.Background()
	mockLotRepo := new(mockLotRepo)
	mockProjRepo := new(mockProjectRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewLotService(mockLotRepo, mockProjRepo, auditSvc)

	lot := &models.Lot{
		ID:        1,
		ProjectID: 1,
	}

	project := &models.Project{
		ID:       1,
		LotCount: 5,
	}

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	mockLotRepo.On("Delete", ctx, uint(1)).Return(nil)
	mockProjRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	// Expect lot count to be decremented
	mockProjRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.LotCount == 4
	})).Return(nil)

	err := service.Delete(ctx, 1, 1)

	assert.NoError(t, err)

	mockLotRepo.AssertExpectations(t)
	mockProjRepo.AssertExpectations(t)
}
