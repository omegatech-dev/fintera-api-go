package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/storage"
)

// Media validation errors.
var (
	ErrUnsupportedMedia = errors.New("tipo de archivo no permitido")
	ErrMediaTooLarge    = errors.New("el archivo excede el tamaño permitido")
)

const (
	// DrawingMaxBytes is the API limit for project plans.
	DrawingMaxBytes int64 = 50 * 1024 * 1024
	// PhotoMaxBytes is the API limit for project and lot photos.
	PhotoMaxBytes int64 = 10 * 1024 * 1024
)

var drawingExtensions = map[string]string{
	".dwg": models.DrawingTypeDWG,
	".dxf": models.DrawingTypeDXF,
	".pdf": models.DrawingTypePDF,
	".svg": models.DrawingTypeSVG,
}

var photoExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// MediaService stores and publishes project drawings and photo galleries.
type MediaService struct {
	drawings     repository.ProjectDrawingRepository
	projectPhoto repository.ProjectPhotoRepository
	lotPhoto     repository.LotPhotoRepository
	projects     repository.ProjectRepository
	lots         repository.LotRepository
	storage      *storage.LocalStorage
	audit        *AuditService
}

// NewMediaService creates the media service.
func NewMediaService(
	drawings repository.ProjectDrawingRepository,
	projectPhoto repository.ProjectPhotoRepository,
	lotPhoto repository.LotPhotoRepository,
	projects repository.ProjectRepository,
	lots repository.LotRepository,
	storage *storage.LocalStorage,
	audit *AuditService,
) *MediaService {
	return &MediaService{
		drawings:     drawings,
		projectPhoto: projectPhoto,
		lotPhoto:     lotPhoto,
		projects:     projects,
		lots:         lots,
		storage:      storage,
		audit:        audit,
	}
}

// ListDrawings returns the drawings of a project for the admin view.
func (s *MediaService) ListDrawings(ctx context.Context, projectID uint) ([]models.ProjectDrawingResponse, error) {
	drawings, err := s.drawings.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	responses := make([]models.ProjectDrawingResponse, 0, len(drawings))
	for i := range drawings {
		responses = append(responses, drawings[i].ToResponse())
	}
	return responses, nil
}

// ListProjectPhotos returns the project gallery for the admin view.
func (s *MediaService) ListProjectPhotos(ctx context.Context, projectID uint) ([]models.PhotoResponse, error) {
	photos, err := s.projectPhoto.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	responses := make([]models.PhotoResponse, 0, len(photos))
	for i := range photos {
		responses = append(responses, photos[i].ToResponse())
	}
	return responses, nil
}

// ListLotPhotos returns a lot gallery for the admin view.
func (s *MediaService) ListLotPhotos(ctx context.Context, lotID uint) ([]models.PhotoResponse, error) {
	photos, err := s.lotPhoto.ListByLot(ctx, lotID)
	if err != nil {
		return nil, err
	}
	responses := make([]models.PhotoResponse, 0, len(photos))
	for i := range photos {
		responses = append(responses, photos[i].ToResponse())
	}
	return responses, nil
}

// ListPublicDrawings returns the published drawings of a project by GUID.
func (s *MediaService) ListPublicDrawings(ctx context.Context, projectGUID string) ([]models.ProjectDrawingResponse, error) {
	project, err := s.projects.FindPublishedByGUID(ctx, projectGUID)
	if err != nil {
		return nil, ErrNotFound
	}
	drawings, err := s.drawings.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	responses := make([]models.ProjectDrawingResponse, 0, len(drawings))
	for i := range drawings {
		responses = append(responses, drawings[i].ToResponse())
	}
	return responses, nil
}

// ListPublicProjectPhotos returns the project gallery by GUID.
func (s *MediaService) ListPublicProjectPhotos(ctx context.Context, projectGUID string) ([]models.PhotoResponse, error) {
	project, err := s.projects.FindPublishedByGUID(ctx, projectGUID)
	if err != nil {
		return nil, ErrNotFound
	}
	photos, err := s.projectPhoto.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	responses := make([]models.PhotoResponse, 0, len(photos))
	for i := range photos {
		responses = append(responses, photos[i].ToResponse())
	}
	return responses, nil
}

// ListPublicLotPhotos returns a lot's gallery by project/lot GUID.
func (s *MediaService) ListPublicLotPhotos(ctx context.Context, projectGUID, lotGUID string) ([]models.PhotoResponse, error) {
	project, err := s.projects.FindPublishedByGUID(ctx, projectGUID)
	if err != nil {
		return nil, ErrNotFound
	}
	lot, err := s.lots.FindByProjectAndGUID(ctx, project.ID, lotGUID)
	if err != nil {
		return nil, ErrNotFound
	}
	photos, err := s.lotPhoto.ListByLot(ctx, lot.ID)
	if err != nil {
		return nil, err
	}
	responses := make([]models.PhotoResponse, 0, len(photos))
	for i := range photos {
		responses = append(responses, photos[i].ToResponse())
	}
	return responses, nil
}

// UploadDrawing stores a plan and creates its record.
func (s *MediaService) UploadDrawing(
	ctx context.Context,
	projectID, actorID uint,
	file multipart.File,
	header *multipart.FileHeader,
	name, version, note string,
) (*models.ProjectDrawing, error) {
	if _, err := s.projects.FindByID(ctx, projectID); err != nil {
		return nil, ErrNotFound
	}
	fileType, err := drawingType(header.Filename)
	if err != nil {
		return nil, err
	}
	if header.Size > DrawingMaxBytes {
		return nil, ErrMediaTooLarge
	}

	path, err := s.storage.Upload(file, header, "uploads/projects")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(name) == "" {
		name = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}
	drawing := &models.ProjectDrawing{
		ProjectID:  projectID,
		Name:       name,
		FilePath:   path,
		FileType:   fileType,
		SizeBytes:  header.Size,
		UploadedBy: &actorID,
	}
	if version != "" {
		drawing.Version = &version
	}
	if note != "" {
		drawing.Note = &note
	}

	if err := s.drawings.Create(ctx, drawing); err != nil {
		_ = s.storage.Delete(path)
		return nil, err
	}
	logIgnoredError("audit drawing upload", s.audit.Log(ctx, actorID, "CREATE", "ProjectDrawing", drawing.ID,
		fmt.Sprintf("Plano %s subido al proyecto %d", drawing.Name, projectID), "", ""))
	return drawing, nil
}

// UpdateDrawing updates the public metadata of a drawing.
func (s *MediaService) UpdateDrawing(ctx context.Context, id, actorID uint, name, version, note *string) (*models.ProjectDrawing, error) {
	drawing, err := s.drawings.FindByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if name != nil {
		drawing.Name = strings.TrimSpace(*name)
	}
	if version != nil {
		if strings.TrimSpace(*version) == "" {
			drawing.Version = nil
		} else {
			drawing.Version = version
		}
	}
	if note != nil {
		if strings.TrimSpace(*note) == "" {
			drawing.Note = nil
		} else {
			drawing.Note = note
		}
	}
	if err := s.drawings.Update(ctx, drawing); err != nil {
		return nil, err
	}
	logIgnoredError("audit drawing update", s.audit.Log(ctx, actorID, "UPDATE", "ProjectDrawing", drawing.ID,
		fmt.Sprintf("Plano %s actualizado", drawing.Name), "", ""))
	return drawing, nil
}

// DeleteDrawing removes a drawing record and its file.
func (s *MediaService) DeleteDrawing(ctx context.Context, id, actorID uint) error {
	drawing, err := s.drawings.FindByID(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if err := s.drawings.Delete(ctx, id); err != nil {
		return err
	}
	logIgnoredError("delete drawing file", s.storage.Delete(drawing.FilePath))
	logIgnoredError("audit drawing delete", s.audit.Log(ctx, actorID, "DELETE", "ProjectDrawing", id,
		fmt.Sprintf("Plano %s eliminado", drawing.Name), "", ""))
	return nil
}

// UploadProjectPhoto stores one photo in the project gallery.
func (s *MediaService) UploadProjectPhoto(
	ctx context.Context,
	projectID, actorID uint,
	file multipart.File,
	header *multipart.FileHeader,
	thumb multipart.File,
	thumbHeader *multipart.FileHeader,
	alt string,
) (*models.ProjectPhoto, error) {
	if _, err := s.projects.FindByID(ctx, projectID); err != nil {
		return nil, ErrNotFound
	}
	if err := validatePhoto(header); err != nil {
		return nil, err
	}
	position, err := s.projectPhoto.NextPosition(ctx, projectID)
	if err != nil {
		return nil, err
	}

	path, err := s.storage.Upload(file, header, "uploads/projects")
	if err != nil {
		return nil, err
	}
	thumbPath, err := s.storeThumb(thumb, thumbHeader, "uploads/projects/thumbs")
	if err != nil {
		_ = s.storage.Delete(path)
		return nil, err
	}

	photo := &models.ProjectPhoto{
		ProjectID:  projectID,
		FilePath:   path,
		Position:   position,
		UploadedBy: &actorID,
	}
	if thumbPath != "" {
		photo.ThumbPath = &thumbPath
	}
	if strings.TrimSpace(alt) != "" {
		photo.Alt = &alt
	}
	if err := s.projectPhoto.Create(ctx, photo); err != nil {
		_ = s.storage.Delete(path)
		if thumbPath != "" {
			_ = s.storage.Delete(thumbPath)
		}
		return nil, err
	}
	logIgnoredError("audit project photo upload", s.audit.Log(ctx, actorID, "CREATE", "ProjectPhoto", photo.ID,
		fmt.Sprintf("Foto subida al proyecto %d", projectID), "", ""))
	return photo, nil
}

// UploadLotPhoto stores one photo in a lot gallery.
func (s *MediaService) UploadLotPhoto(
	ctx context.Context,
	lotID, actorID uint,
	file multipart.File,
	header *multipart.FileHeader,
	thumb multipart.File,
	thumbHeader *multipart.FileHeader,
	alt string,
) (*models.LotPhoto, error) {
	if _, err := s.lots.FindByID(ctx, lotID); err != nil {
		return nil, ErrNotFound
	}
	if err := validatePhoto(header); err != nil {
		return nil, err
	}
	position, err := s.lotPhoto.NextPosition(ctx, lotID)
	if err != nil {
		return nil, err
	}

	path, err := s.storage.Upload(file, header, "uploads/lots")
	if err != nil {
		return nil, err
	}
	thumbPath, err := s.storeThumb(thumb, thumbHeader, "uploads/lots/thumbs")
	if err != nil {
		_ = s.storage.Delete(path)
		return nil, err
	}

	photo := &models.LotPhoto{
		LotID:      lotID,
		FilePath:   path,
		Position:   position,
		UploadedBy: &actorID,
	}
	if thumbPath != "" {
		photo.ThumbPath = &thumbPath
	}
	if strings.TrimSpace(alt) != "" {
		photo.Alt = &alt
	}
	if err := s.lotPhoto.Create(ctx, photo); err != nil {
		_ = s.storage.Delete(path)
		if thumbPath != "" {
			_ = s.storage.Delete(thumbPath)
		}
		return nil, err
	}
	logIgnoredError("audit lot photo upload", s.audit.Log(ctx, actorID, "CREATE", "LotPhoto", photo.ID,
		fmt.Sprintf("Foto subida al lote %d", lotID), "", ""))
	return photo, nil
}

// UpdateProjectPhoto updates position and alt text, normalizing the gallery.
func (s *MediaService) UpdateProjectPhoto(ctx context.Context, projectID, id, actorID uint, position *int, alt *string) (*models.ProjectPhoto, error) {
	photo, err := s.projectPhoto.FindByID(ctx, id)
	if err != nil || (projectID != 0 && photo.ProjectID != projectID) {
		return nil, ErrNotFound
	}
	applyPhotoChanges(photo, position, alt)
	if err := s.projectPhoto.Update(ctx, photo); err != nil {
		return nil, err
	}
	if position != nil {
		s.normalizeProjectPhotos(ctx, photo.ProjectID)
	}
	logIgnoredError("audit project photo update", s.audit.Log(ctx, actorID, "UPDATE", "ProjectPhoto", photo.ID,
		fmt.Sprintf("Foto del proyecto %d actualizada", photo.ProjectID), "", ""))
	return photo, nil
}

// UpdateLotPhoto updates position and alt text, normalizing the gallery.
func (s *MediaService) UpdateLotPhoto(ctx context.Context, lotID, id, actorID uint, position *int, alt *string) (*models.LotPhoto, error) {
	photo, err := s.lotPhoto.FindByID(ctx, id)
	if err != nil || (lotID != 0 && photo.LotID != lotID) {
		return nil, ErrNotFound
	}
	if position != nil {
		photo.Position = *position
	}
	if alt != nil {
		photo.Alt = alt
	}
	if err := s.lotPhoto.Update(ctx, photo); err != nil {
		return nil, err
	}
	if position != nil {
		s.normalizeLotPhotos(ctx, photo.LotID)
	}
	logIgnoredError("audit lot photo update", s.audit.Log(ctx, actorID, "UPDATE", "LotPhoto", photo.ID,
		fmt.Sprintf("Foto del lote %d actualizada", photo.LotID), "", ""))
	return photo, nil
}

// DeleteProjectPhoto removes a project photo and its files.
func (s *MediaService) DeleteProjectPhoto(ctx context.Context, projectID, id, actorID uint) error {
	photo, err := s.projectPhoto.FindByID(ctx, id)
	if err != nil || (projectID != 0 && photo.ProjectID != projectID) {
		return ErrNotFound
	}
	if err := s.projectPhoto.Delete(ctx, id); err != nil {
		return err
	}
	logIgnoredError("delete project photo", s.storage.Delete(photo.FilePath))
	if photo.ThumbPath != nil {
		logIgnoredError("delete project thumb", s.storage.Delete(*photo.ThumbPath))
	}
	s.normalizeProjectPhotos(ctx, photo.ProjectID)
	logIgnoredError("audit project photo delete", s.audit.Log(ctx, actorID, "DELETE", "ProjectPhoto", id,
		fmt.Sprintf("Foto del proyecto %d eliminada", photo.ProjectID), "", ""))
	return nil
}

// DeleteLotPhoto removes a lot photo and its files.
func (s *MediaService) DeleteLotPhoto(ctx context.Context, lotID, id, actorID uint) error {
	photo, err := s.lotPhoto.FindByID(ctx, id)
	if err != nil || (lotID != 0 && photo.LotID != lotID) {
		return ErrNotFound
	}
	if err := s.lotPhoto.Delete(ctx, id); err != nil {
		return err
	}
	logIgnoredError("delete lot photo", s.storage.Delete(photo.FilePath))
	if photo.ThumbPath != nil {
		logIgnoredError("delete lot thumb", s.storage.Delete(*photo.ThumbPath))
	}
	s.normalizeLotPhotos(ctx, photo.LotID)
	logIgnoredError("audit lot photo delete", s.audit.Log(ctx, actorID, "DELETE", "LotPhoto", id,
		fmt.Sprintf("Foto del lote %d eliminada", photo.LotID), "", ""))
	return nil
}

func (s *MediaService) storeThumb(file multipart.File, header *multipart.FileHeader, subDir string) (string, error) {
	if file == nil || header == nil {
		return "", nil
	}
	if err := validatePhoto(header); err != nil {
		return "", err
	}
	return s.storage.Upload(file, header, subDir)
}

func (s *MediaService) normalizeProjectPhotos(ctx context.Context, projectID uint) {
	photos, err := s.projectPhoto.ListByProject(ctx, projectID)
	if err != nil {
		return
	}
	sort.SliceStable(photos, func(i, j int) bool {
		if photos[i].Position == photos[j].Position {
			return photos[i].ID < photos[j].ID
		}
		return photos[i].Position < photos[j].Position
	})
	for index := range photos {
		if photos[index].Position == index {
			continue
		}
		photos[index].Position = index
		logIgnoredError("normalize project photo", s.projectPhoto.Update(ctx, &photos[index]))
	}
}

func (s *MediaService) normalizeLotPhotos(ctx context.Context, lotID uint) {
	photos, err := s.lotPhoto.ListByLot(ctx, lotID)
	if err != nil {
		return
	}
	sort.SliceStable(photos, func(i, j int) bool {
		if photos[i].Position == photos[j].Position {
			return photos[i].ID < photos[j].ID
		}
		return photos[i].Position < photos[j].Position
	})
	for index := range photos {
		if photos[index].Position == index {
			continue
		}
		photos[index].Position = index
		logIgnoredError("normalize lot photo", s.lotPhoto.Update(ctx, &photos[index]))
	}
}

func applyPhotoChanges(photo *models.ProjectPhoto, position *int, alt *string) {
	if position != nil {
		photo.Position = *position
	}
	if alt != nil {
		photo.Alt = alt
	}
}

func drawingType(filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	fileType, ok := drawingExtensions[ext]
	if !ok {
		return "", ErrUnsupportedMedia
	}
	return fileType, nil
}

func validatePhoto(header *multipart.FileHeader) error {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !photoExtensions[ext] {
		return ErrUnsupportedMedia
	}
	if header.Size > PhotoMaxBytes {
		return ErrMediaTooLarge
	}
	contentType := strings.ToLower(header.Header.Get("Content-Type"))
	switch contentType {
	case "", "image/jpeg", "image/jpg", "image/png", "image/webp", "application/octet-stream":
		return nil
	default:
		return ErrUnsupportedMedia
	}
}
