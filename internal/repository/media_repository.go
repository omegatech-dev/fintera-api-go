package repository

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// ProjectDrawingRepository defines persistence for project drawings.
type ProjectDrawingRepository interface {
	Create(ctx context.Context, drawing *models.ProjectDrawing) error
	FindByID(ctx context.Context, id uint) (*models.ProjectDrawing, error)
	ListByProject(ctx context.Context, projectID uint) ([]models.ProjectDrawing, error)
	Update(ctx context.Context, drawing *models.ProjectDrawing) error
	Delete(ctx context.Context, id uint) error
}

type projectDrawingRepository struct{ db *gorm.DB }

func NewProjectDrawingRepository(db *gorm.DB) ProjectDrawingRepository {
	return &projectDrawingRepository{db: db}
}

func (r *projectDrawingRepository) Create(ctx context.Context, drawing *models.ProjectDrawing) error {
	return r.db.WithContext(ctx).Create(drawing).Error
}

func (r *projectDrawingRepository) FindByID(ctx context.Context, id uint) (*models.ProjectDrawing, error) {
	var drawing models.ProjectDrawing
	if err := r.db.WithContext(ctx).First(&drawing, id).Error; err != nil {
		return nil, err
	}
	return &drawing, nil
}

func (r *projectDrawingRepository) ListByProject(ctx context.Context, projectID uint) ([]models.ProjectDrawing, error) {
	var drawings []models.ProjectDrawing
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at ASC").
		Find(&drawings).Error
	return drawings, err
}

func (r *projectDrawingRepository) Update(ctx context.Context, drawing *models.ProjectDrawing) error {
	return r.db.WithContext(ctx).Save(drawing).Error
}

func (r *projectDrawingRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ProjectDrawing{}, id).Error
}

// ProjectPhotoRepository defines persistence for the project gallery.
type ProjectPhotoRepository interface {
	Create(ctx context.Context, photo *models.ProjectPhoto) error
	FindByID(ctx context.Context, id uint) (*models.ProjectPhoto, error)
	ListByProject(ctx context.Context, projectID uint) ([]models.ProjectPhoto, error)
	Update(ctx context.Context, photo *models.ProjectPhoto) error
	Delete(ctx context.Context, id uint) error
	NextPosition(ctx context.Context, projectID uint) (int, error)
}

type projectPhotoRepository struct{ db *gorm.DB }

func NewProjectPhotoRepository(db *gorm.DB) ProjectPhotoRepository {
	return &projectPhotoRepository{db: db}
}

func (r *projectPhotoRepository) Create(ctx context.Context, photo *models.ProjectPhoto) error {
	return r.db.WithContext(ctx).Create(photo).Error
}

func (r *projectPhotoRepository) FindByID(ctx context.Context, id uint) (*models.ProjectPhoto, error) {
	var photo models.ProjectPhoto
	if err := r.db.WithContext(ctx).First(&photo, id).Error; err != nil {
		return nil, err
	}
	return &photo, nil
}

func (r *projectPhotoRepository) ListByProject(ctx context.Context, projectID uint) ([]models.ProjectPhoto, error) {
	var photos []models.ProjectPhoto
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("position ASC, id ASC").
		Find(&photos).Error
	return photos, err
}

func (r *projectPhotoRepository) Update(ctx context.Context, photo *models.ProjectPhoto) error {
	return r.db.WithContext(ctx).Save(photo).Error
}

func (r *projectPhotoRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ProjectPhoto{}, id).Error
}

func (r *projectPhotoRepository) NextPosition(ctx context.Context, projectID uint) (int, error) {
	var max *int
	err := r.db.WithContext(ctx).
		Model(&models.ProjectPhoto{}).
		Where("project_id = ?", projectID).
		Select("MAX(position)").
		Scan(&max).Error
	if err != nil || max == nil {
		return 0, err
	}
	return *max + 1, nil
}

// LotPhotoRepository defines persistence for lot galleries.
type LotPhotoRepository interface {
	Create(ctx context.Context, photo *models.LotPhoto) error
	FindByID(ctx context.Context, id uint) (*models.LotPhoto, error)
	ListByLot(ctx context.Context, lotID uint) ([]models.LotPhoto, error)
	Update(ctx context.Context, photo *models.LotPhoto) error
	Delete(ctx context.Context, id uint) error
	NextPosition(ctx context.Context, lotID uint) (int, error)
}

type lotPhotoRepository struct{ db *gorm.DB }

func NewLotPhotoRepository(db *gorm.DB) LotPhotoRepository {
	return &lotPhotoRepository{db: db}
}

func (r *lotPhotoRepository) Create(ctx context.Context, photo *models.LotPhoto) error {
	return r.db.WithContext(ctx).Create(photo).Error
}

func (r *lotPhotoRepository) FindByID(ctx context.Context, id uint) (*models.LotPhoto, error) {
	var photo models.LotPhoto
	if err := r.db.WithContext(ctx).First(&photo, id).Error; err != nil {
		return nil, err
	}
	return &photo, nil
}

func (r *lotPhotoRepository) ListByLot(ctx context.Context, lotID uint) ([]models.LotPhoto, error) {
	var photos []models.LotPhoto
	err := r.db.WithContext(ctx).
		Where("lot_id = ?", lotID).
		Order("position ASC, id ASC").
		Find(&photos).Error
	return photos, err
}

func (r *lotPhotoRepository) Update(ctx context.Context, photo *models.LotPhoto) error {
	return r.db.WithContext(ctx).Save(photo).Error
}

func (r *lotPhotoRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LotPhoto{}, id).Error
}

func (r *lotPhotoRepository) NextPosition(ctx context.Context, lotID uint) (int, error) {
	var max *int
	err := r.db.WithContext(ctx).
		Model(&models.LotPhoto{}).
		Where("lot_id = ?", lotID).
		Select("MAX(position)").
		Scan(&max).Error
	if err != nil || max == nil {
		return 0, err
	}
	return *max + 1, nil
}

// ProjectPaymentInstructionRepository defines persistence for payment
// instructions.
type ProjectPaymentInstructionRepository interface {
	Create(ctx context.Context, instruction *models.ProjectPaymentInstruction) error
	FindByID(ctx context.Context, id uint) (*models.ProjectPaymentInstruction, error)
	ListByProject(ctx context.Context, projectID uint) ([]models.ProjectPaymentInstruction, error)
	Update(ctx context.Context, instruction *models.ProjectPaymentInstruction) error
	Delete(ctx context.Context, id uint) error
	NextPosition(ctx context.Context, projectID uint) (int, error)
	ClearDefault(ctx context.Context, projectID, exceptID uint) error
	CountByProject(ctx context.Context, projectID uint) (int64, error)
}

type projectPaymentInstructionRepository struct{ db *gorm.DB }

func NewProjectPaymentInstructionRepository(db *gorm.DB) ProjectPaymentInstructionRepository {
	return &projectPaymentInstructionRepository{db: db}
}

func (r *projectPaymentInstructionRepository) Create(ctx context.Context, instruction *models.ProjectPaymentInstruction) error {
	return r.db.WithContext(ctx).Create(instruction).Error
}

func (r *projectPaymentInstructionRepository) FindByID(ctx context.Context, id uint) (*models.ProjectPaymentInstruction, error) {
	var instruction models.ProjectPaymentInstruction
	if err := r.db.WithContext(ctx).First(&instruction, id).Error; err != nil {
		return nil, err
	}
	return &instruction, nil
}

func (r *projectPaymentInstructionRepository) ListByProject(ctx context.Context, projectID uint) ([]models.ProjectPaymentInstruction, error) {
	var instructions []models.ProjectPaymentInstruction
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("position ASC, id ASC").
		Find(&instructions).Error
	return instructions, err
}

func (r *projectPaymentInstructionRepository) Update(ctx context.Context, instruction *models.ProjectPaymentInstruction) error {
	return r.db.WithContext(ctx).Save(instruction).Error
}

func (r *projectPaymentInstructionRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ProjectPaymentInstruction{}, id).Error
}

func (r *projectPaymentInstructionRepository) ClearDefault(ctx context.Context, projectID, exceptID uint) error {
	query := r.db.WithContext(ctx).
		Model(&models.ProjectPaymentInstruction{}).
		Where("project_id = ? AND is_default = ?", projectID, true)
	if exceptID > 0 {
		query = query.Where("id <> ?", exceptID)
	}
	return query.Update("is_default", false).Error
}

func (r *projectPaymentInstructionRepository) CountByProject(ctx context.Context, projectID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ProjectPaymentInstruction{}).
		Where("project_id = ?", projectID).
		Count(&count).Error
	return count, err
}

func (r *projectPaymentInstructionRepository) NextPosition(ctx context.Context, projectID uint) (int, error) {
	var max *int
	err := r.db.WithContext(ctx).
		Model(&models.ProjectPaymentInstruction{}).
		Where("project_id = ?", projectID).
		Select("MAX(position)").
		Scan(&max).Error
	if err != nil || max == nil {
		return 0, err
	}
	return *max + 1, nil
}
