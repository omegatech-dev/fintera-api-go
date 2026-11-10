package services

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// PublicCatalogService builds the privacy-safe payloads consumed by the
// customer portal.
type PublicCatalogService struct {
	projects     repository.ProjectRepository
	lots         repository.LotRepository
	instructions repository.ProjectPaymentInstructionRepository
}

// NewPublicCatalogService creates the public catalog service.
func NewPublicCatalogService(
	projects repository.ProjectRepository,
	lots repository.LotRepository,
	instructions repository.ProjectPaymentInstructionRepository,
) *PublicCatalogService {
	return &PublicCatalogService{
		projects:     projects,
		lots:         lots,
		instructions: instructions,
	}
}

// ListProjects returns published, active projects with public stats.
func (s *PublicCatalogService) ListProjects(ctx context.Context) ([]models.PublicProjectResponse, error) {
	projects, err := s.projects.ListPublished(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]models.PublicProjectResponse, 0, len(projects))
	for i := range projects {
		responses = append(responses, publicProjectResponse(&projects[i]))
	}
	return responses, nil
}

// GetProject returns a published project by GUID.
func (s *PublicCatalogService) GetProject(ctx context.Context, guid string) (*models.PublicProjectResponse, error) {
	project, err := s.projects.FindPublishedByGUID(ctx, guid)
	if err != nil {
		return nil, ErrNotFound
	}
	resp := publicProjectResponse(project)
	return &resp, nil
}

// ListLots returns the public lot payloads of a published project.
func (s *PublicCatalogService) ListLots(ctx context.Context, projectGUID string, onlyAvailable bool) ([]models.PublicLotResponse, error) {
	project, err := s.projects.FindPublishedByGUID(ctx, projectGUID)
	if err != nil {
		return nil, ErrNotFound
	}

	responses := make([]models.PublicLotResponse, 0, len(project.Lots))
	for i := range project.Lots {
		lot := &project.Lots[i]
		if lot.Status == models.LotStatusActive {
			continue
		}
		if onlyAvailable && !lot.IsAvailable() {
			continue
		}
		responses = append(responses, publicLotResponse(lot))
	}
	return responses, nil
}

// GetLot returns one public lot of a published project.
func (s *PublicCatalogService) GetLot(ctx context.Context, projectGUID, lotGUID string) (*models.PublicLotResponse, error) {
	project, err := s.projects.FindPublishedByGUID(ctx, projectGUID)
	if err != nil {
		return nil, ErrNotFound
	}
	lot, err := s.lots.FindByProjectAndGUID(ctx, project.ID, lotGUID)
	if err != nil || lot.Status == models.LotStatusActive {
		return nil, ErrNotFound
	}
	resp := publicLotResponse(lot)
	return &resp, nil
}

// ListPaymentInstructions returns the public instructions of a project.
func (s *PublicCatalogService) ListPaymentInstructions(ctx context.Context, projectGUID string) ([]models.PublicPaymentInstructionResponse, error) {
	project, err := s.projects.FindPublishedByGUID(ctx, projectGUID)
	if err != nil {
		return nil, ErrNotFound
	}
	instructions, err := s.instructions.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	responses := make([]models.PublicPaymentInstructionResponse, 0, len(instructions))
	for i := range instructions {
		responses = append(responses, instructions[i].ToPublicResponse())
	}
	return responses, nil
}

func publicProjectResponse(project *models.Project) models.PublicProjectResponse {
	stats := models.PublicProjectStats{}
	var priceFrom, priceTo *float64
	for i := range project.Lots {
		lot := &project.Lots[i]
		if lot.Status == models.LotStatusActive {
			continue
		}
		stats.TotalLots++
		switch lot.Status {
		case models.LotStatusAvailable:
			stats.AvailableLots++
		case models.LotStatusReserved:
			stats.ReservedLots++
		case models.LotStatusFinanced:
			stats.FinancedLots++
			stats.SoldLots++
		case models.LotStatusFullyPaid:
			stats.PaidLots++
			stats.SoldLots++
		}
		if price := lot.EffectivePrice(); price > 0 {
			if priceFrom == nil || price < *priceFrom {
				value := price
				priceFrom = &value
			}
			if priceTo == nil || price > *priceTo {
				value := price
				priceTo = &value
			}
		}
	}

	photos := make([]models.PhotoResponse, 0, len(project.Photos))
	for i := range project.Photos {
		photos = append(photos, project.Photos[i].ToResponse())
	}

	return models.PublicProjectResponse{
		GUID:               project.GUID,
		Name:               project.Name,
		Description:        project.Description,
		ProjectType:        project.ProjectType,
		Address:            project.Address,
		City:               project.City,
		Code:               project.Code,
		DeliveryDate:       project.DeliveryDate,
		CreatedAt:          project.CreatedAt,
		PricePerSquareUnit: project.PricePerSquareUnit,
		MeasurementUnit:    project.MeasurementUnit,
		PriceFrom:          priceFrom,
		PriceTo:            priceTo,
		DrawingCount:       len(project.Drawings),
		Stats:              stats,
		Photos:             photos,
		PhotoCount:         len(photos),
	}
}

func publicLotResponse(lot *models.Lot) models.PublicLotResponse {
	photos := make([]models.PhotoResponse, 0, len(lot.Photos))
	for i := range lot.Photos {
		photos = append(photos, lot.Photos[i].ToResponse())
	}

	return models.PublicLotResponse{
		GUID:            lot.GUID,
		Name:            lot.Name,
		Status:          lot.Status,
		Length:          lot.Length,
		Width:           lot.Width,
		Area:            lot.Area(),
		OverrideArea:    lot.OverrideArea,
		EffectivePrice:  lot.EffectivePrice(),
		HasSpecialPrice: lot.OverridePrice != nil && *lot.OverridePrice > 0,
		Boundaries: models.PublicLotBoundaries{
			North: lot.North,
			South: lot.South,
			East:  lot.East,
			West:  lot.West,
		},
		MeasurementUnit: lot.MeasurementUnit,
		Photos:          photos,
	}
}
