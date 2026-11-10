package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

type ProjectService struct {
	repo     repository.ProjectRepository
	lotRepo  repository.LotRepository
	auditSvc *AuditService
}

func NewProjectService(repo repository.ProjectRepository, lotRepo repository.LotRepository, auditSvc *AuditService) *ProjectService {
	return &ProjectService{repo: repo, lotRepo: lotRepo, auditSvc: auditSvc}
}

func (s *ProjectService) FindByID(ctx context.Context, id uint) (*models.Project, error) {
	return s.repo.FindByID(ctx, id)
}

// SetProjectPublished changes only the project's visibility. The active flag is
// intentionally left untouched so admins can publish and deactivate
// independently.
func (s *ProjectService) SetProjectPublished(ctx context.Context, id, actorID uint, published bool) (*models.Project, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if project.IsPublished == published {
		return project, nil
	}

	project.IsPublished = published
	if err := s.repo.Update(ctx, project); err != nil {
		return nil, err
	}

	action := "PUBLISH"
	message := fmt.Sprintf("Proyecto publicado: %s", project.Name)
	if !published {
		action = "UNPUBLISH"
		message = fmt.Sprintf("Proyecto despublicado: %s", project.Name)
	}
	logIgnoredError("audit project visibility", s.auditSvc.Log(ctx, actorID, action, "Project", project.ID, message, "", ""))
	return project, nil
}

func (s *ProjectService) List(ctx context.Context, query *repository.ListQuery) ([]models.Project, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *ProjectService) Create(ctx context.Context, project *models.Project, actorID uint) error {
	// Auto-generate GUID if not provided
	if project.GUID == "" {
		project.GUID = uuid.New().String()
	}

	project.MeasurementUnit = models.NormalizeMeasurementUnit(project.MeasurementUnit)

	// Auto-generate lots if lot count is specified
	if project.LotCount > 0 {
		project.Lots = make([]models.Lot, 0, project.LotCount)
		for i := 1; i <= project.LotCount; i++ {
			lot := models.Lot{
				ProjectID:       project.ID, // This will be handled by GORM on create
				Name:            "Lote " + fmt.Sprintf("%03d", i),
				Status:          models.LotStatusAvailable,
				Length:          20.0,
				Width:           10.0,
				Price:           project.PricePerSquareUnit * 200.0, // Assuming 20x10=200 area
				MeasurementUnit: &project.MeasurementUnit,
			}
			project.Lots = append(project.Lots, lot)
		}
	}
	if err := s.repo.Create(ctx, project); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "CREATE", "Project", project.ID, fmt.Sprintf("Proyecto creado: %s. %d lotes generados.", project.Name, project.LotCount), "", "")
}

func (s *ProjectService) Update(ctx context.Context, project *models.Project, actorID uint) error {
	existing, err := s.repo.FindByID(ctx, project.ID)
	if err != nil {
		return err
	}

	// Ensure GUID is preserved if not provided in the update
	if project.GUID == "" {
		project.GUID = existing.GUID
	}

	// Visibility is managed separately from the edit form; preserve it so a
	// normal project update never unpublishes the project.
	project.IsPublished = existing.IsPublished
	project.IsActive = existing.IsActive

	// An omitted unit means "keep the current one"; normalize aliases so the
	// stored value is always one of m2, ft2 or vara2.
	if strings.TrimSpace(project.MeasurementUnit) == "" {
		project.MeasurementUnit = existing.MeasurementUnit
	}
	oldUnit := models.NormalizeMeasurementUnit(existing.MeasurementUnit)
	newUnit := models.NormalizeMeasurementUnit(project.MeasurementUnit)
	project.MeasurementUnit = newUnit

	measurementUnitChanged := oldUnit != newUnit
	pricePerUnitChanged := existing.PricePerSquareUnit != project.PricePerSquareUnit

	if measurementUnitChanged {
		lots, err := s.lotRepo.FindByProject(ctx, project.ID)
		if err != nil {
			return err
		}

		// Preserve the economic value when the admin only changed the unit.
		// If they also changed the price per unit, their value wins.
		if !pricePerUnitChanged {
			project.PricePerSquareUnit = models.ConvertPricePerArea(
				existing.PricePerSquareUnit, oldUnit, newUnit,
			)
		}

		for i := range lots {
			lots[i].Length = models.ConvertLength(lots[i].Length, oldUnit, newUnit)
			lots[i].Width = models.ConvertLength(lots[i].Width, oldUnit, newUnit)
			if lots[i].OverrideArea != nil {
				converted := models.ConvertArea(*lots[i].OverrideArea, oldUnit, newUnit)
				lots[i].OverrideArea = &converted
			}
			unit := newUnit
			lots[i].MeasurementUnit = &unit

			if lots[i].Status == models.LotStatusAvailable &&
				(lots[i].OverridePrice == nil || *lots[i].OverridePrice == 0) {
				lots[i].Price = lots[i].Area() * project.PricePerSquareUnit
			}
		}

		if err := s.repo.UpdateProjectAndLots(ctx, project, lots); err != nil {
			return err
		}
		return s.auditSvc.Log(ctx, actorID, "UPDATE", "Project", project.ID, fmt.Sprintf("Proyecto actualizado: %s", project.Name), "", "")
	}

	if pricePerUnitChanged {
		lots, err := s.lotRepo.FindByProject(ctx, project.ID)
		if err != nil {
			return err
		}
		for i := range lots {
			if lots[i].Status != models.LotStatusAvailable {
				continue
			}
			if lots[i].OverridePrice == nil || *lots[i].OverridePrice == 0 {
				lots[i].Price = lots[i].Area() * project.PricePerSquareUnit
			}
			if err := s.lotRepo.Update(ctx, &lots[i]); err != nil {
				return err
			}
		}
	}

	if err := s.repo.Update(ctx, project); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "UPDATE", "Project", project.ID, fmt.Sprintf("Proyecto actualizado: %s", project.Name), "", "")
}

func (s *ProjectService) Delete(ctx context.Context, id uint, actorID uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "DELETE", "Project", id, "Proyecto eliminado", "", "")
}

type LotService struct {
	repo        repository.LotRepository
	projectRepo repository.ProjectRepository
	auditSvc    *AuditService
}

func NewLotService(repo repository.LotRepository, projectRepo repository.ProjectRepository, auditSvc *AuditService) *LotService {
	return &LotService{repo: repo, projectRepo: projectRepo, auditSvc: auditSvc}
}

// resolveLotUnit guarantees a lot carries a real unit for display. The lot
// value wins; otherwise the project value is used, normalized to m2 when both
// are empty.
func resolveLotUnit(lot *models.Lot, projectUnit string) {
	unit := models.ResolveMeasurementUnit(lot.MeasurementUnit, projectUnit)
	lot.MeasurementUnit = &unit
}

func (s *LotService) FindByID(ctx context.Context, id uint) (*models.Lot, error) {
	lot, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resolveLotUnit(lot, lot.Project.MeasurementUnit)
	return lot, nil
}

func (s *LotService) FindByProject(ctx context.Context, projectID uint) ([]models.Lot, error) {
	lots, err := s.repo.FindByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	projectUnit := ""
	if project, err := s.projectRepo.FindByID(ctx, projectID); err == nil {
		projectUnit = project.MeasurementUnit
	}
	for i := range lots {
		resolveLotUnit(&lots[i], projectUnit)
	}
	return lots, nil
}

func (s *LotService) List(ctx context.Context, projectID uint, query *repository.ListQuery) ([]models.Lot, int64, error) {
	lots, total, err := s.repo.List(ctx, projectID, query)
	if err != nil {
		return nil, 0, err
	}
	projectUnit := ""
	if project, err := s.projectRepo.FindByID(ctx, projectID); err == nil {
		projectUnit = project.MeasurementUnit
	}
	for i := range lots {
		resolveLotUnit(&lots[i], projectUnit)
	}
	return lots, total, nil
}

func (s *LotService) Create(ctx context.Context, lot *models.Lot, actorID uint) error {
	project, err := s.projectRepo.FindByID(ctx, lot.ProjectID)
	if err != nil {
		return err
	}
	unit := models.ResolveMeasurementUnit(lot.MeasurementUnit, project.MeasurementUnit)
	lot.MeasurementUnit = &unit

	if err := s.repo.Create(ctx, lot); err != nil {
		return err
	}
	// Keep project lot_count in sync
	project.LotCount++
	if err := s.projectRepo.Update(ctx, project); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "CREATE", "Lot", lot.ID, fmt.Sprintf("Lote creado: %s en Proyecto %s", lot.Name, project.Name), "", "")
}

func (s *LotService) Update(ctx context.Context, lot *models.Lot, actorID uint) error {
	existingLot, err := s.repo.FindByID(ctx, lot.ID)
	if err != nil {
		return err
	}

	// Preserve critical fields if not provided
	if lot.ProjectID == 0 {
		lot.ProjectID = existingLot.ProjectID
	}
	if lot.Status == "" {
		lot.Status = existingLot.Status
	}

	// Preserve metadata fields if not provided (zero/nil)
	if lot.Address == nil {
		lot.Address = existingLot.Address
	}
	if lot.MeasurementUnit == nil {
		lot.MeasurementUnit = existingLot.MeasurementUnit
	}
	unit := models.ResolveMeasurementUnit(lot.MeasurementUnit, existingLot.Project.MeasurementUnit)
	lot.MeasurementUnit = &unit
	if lot.RegistrationNumber == nil {
		lot.RegistrationNumber = existingLot.RegistrationNumber
	}
	if lot.Note == nil {
		lot.Note = existingLot.Note
	}
	if lot.North == nil {
		lot.North = existingLot.North
	}
	if lot.South == nil {
		lot.South = existingLot.South
	}
	if lot.East == nil {
		lot.East = existingLot.East
	}
	if lot.West == nil {
		lot.West = existingLot.West
	}

	// Carry over project association for ToResponse
	lot.Project = existingLot.Project

	// Recalculate base price if 0
	if lot.Price == 0 && lot.Project.PricePerSquareUnit > 0 {
		lot.Price = lot.Area() * lot.Project.PricePerSquareUnit
	}

	// Ideally we should use a PATCH approach, but for now this fixes the critical data loss
	if err := s.repo.Update(ctx, lot); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "UPDATE", "Lot", lot.ID, fmt.Sprintf("Lote actualizado: %s", lot.Name), "", "")
}

func (s *LotService) Delete(ctx context.Context, id uint, actorID uint) error {
	lot, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	projectID := lot.ProjectID
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// Keep project lot_count in sync
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}
	if project.LotCount > 0 {
		project.LotCount--
		if err := s.projectRepo.Update(ctx, project); err != nil {
			return err
		}
	}
	return s.auditSvc.Log(ctx, actorID, "DELETE", "Lot", id, "Lote eliminado", "", "")
}
