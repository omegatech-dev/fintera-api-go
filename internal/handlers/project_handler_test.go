package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type stubLotRepo struct{}

func (s *stubLotRepo) FindByID(ctx context.Context, id uint) (*models.Lot, error) { return nil, nil }

func (s *stubLotRepo) FindByGUID(ctx context.Context, guid string) (*models.Lot, error) {
	return nil, nil
}

func (s *stubLotRepo) FindByProjectAndGUID(ctx context.Context, projectID uint, guid string) (*models.Lot, error) {
	return nil, nil
}

func (s *stubLotRepo) FindByProject(ctx context.Context, projectID uint) ([]models.Lot, error) {
	return nil, nil
}

func (s *stubLotRepo) ReserveIfAvailable(ctx context.Context, id uint) (bool, error) {
	return false, nil
}

func (s *stubLotRepo) Create(ctx context.Context, lot *models.Lot) error { return nil }

func (s *stubLotRepo) Update(ctx context.Context, lot *models.Lot) error { return nil }

func (s *stubLotRepo) Delete(ctx context.Context, id uint) error { return nil }

func (s *stubLotRepo) List(ctx context.Context, projectID uint, query *repository.ListQuery) ([]models.Lot, int64, error) {
	return nil, 0, nil
}

type stubProjectPhotoRepo struct {
	photos []models.ProjectPhoto
}

func (s *stubProjectPhotoRepo) Create(ctx context.Context, photo *models.ProjectPhoto) error {
	return nil
}

func (s *stubProjectPhotoRepo) FindByID(ctx context.Context, id uint) (*models.ProjectPhoto, error) {
	return nil, nil
}

func (s *stubProjectPhotoRepo) ListByProject(ctx context.Context, projectID uint) ([]models.ProjectPhoto, error) {
	return s.photos, nil
}

func (s *stubProjectPhotoRepo) Update(ctx context.Context, photo *models.ProjectPhoto) error {
	return nil
}

func (s *stubProjectPhotoRepo) Delete(ctx context.Context, id uint) error { return nil }

func (s *stubProjectPhotoRepo) NextPosition(ctx context.Context, projectID uint) (int, error) {
	return 0, nil
}

func newProjectHandler(t *testing.T, project *models.Project, photos ...models.ProjectPhoto) *ProjectHandler {
	t.Helper()

	db, mockDB, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	mockDB.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	audit := services.NewAuditService(gdb)
	projectService := services.NewProjectService(&stubProjectRepo{project: project}, &stubLotRepo{}, audit)
	mediaService := services.NewMediaService(nil, &stubProjectPhotoRepo{photos: photos}, nil, nil, nil, nil, nil)
	return NewProjectHandler(projectService, mediaService)
}

func projectContext(method string, id string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/", nil)
	c.Params = gin.Params{{Key: "project_id", Value: id}}
	return c, w
}

func TestProjectHandlerPublish(t *testing.T) {
	project := &models.Project{ID: 1, Name: "Altos", IsPublished: false, IsActive: true}
	handler := newProjectHandler(t, project)

	c, w := projectContext(http.MethodPost, "1")
	handler.Publish(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var payload struct {
		Project models.ProjectResponse `json:"project"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.Project.IsPublished || !payload.Project.IsActive {
		t.Fatalf("published=%v active=%v, want both true", payload.Project.IsPublished, payload.Project.IsActive)
	}
}

func TestProjectHandlerShowIncludesOrderedGallery(t *testing.T) {
	project := &models.Project{ID: 1, Name: "Altos", IsPublished: true, IsActive: true}
	handler := newProjectHandler(t, project,
		models.ProjectPhoto{ID: 1, Position: 0, FilePath: "projects/1/cover.jpg"},
		models.ProjectPhoto{ID: 2, Position: 1, FilePath: "projects/1/living.jpg"},
	)

	c, w := projectContext(http.MethodGet, "1")
	handler.Show(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var payload struct {
		Project models.ProjectResponse `json:"project"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Project.Photos) != 2 {
		t.Fatalf("photos = %d, want 2", len(payload.Project.Photos))
	}
	if payload.Project.Photos[0].URL != "/projects/1/cover.jpg" {
		t.Fatalf("first photo = %q, want /projects/1/cover.jpg", payload.Project.Photos[0].URL)
	}
	if payload.Project.CoverPhotoURL == nil || *payload.Project.CoverPhotoURL != payload.Project.Photos[0].URL {
		t.Fatalf("cover = %v, want first photo URL", payload.Project.CoverPhotoURL)
	}
}

func TestProjectHandlerShowWithoutPhotosReturnsEmptyGallery(t *testing.T) {
	project := &models.Project{ID: 1, Name: "Altos", IsPublished: true, IsActive: true}
	handler := newProjectHandler(t, project)

	c, w := projectContext(http.MethodGet, "1")
	handler.Show(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var payload struct {
		Project models.ProjectResponse `json:"project"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Project.Photos == nil || len(payload.Project.Photos) != 0 {
		t.Fatalf("photos = %v, want empty array", payload.Project.Photos)
	}
	if payload.Project.CoverPhotoURL != nil {
		t.Fatalf("cover = %v, want nil", payload.Project.CoverPhotoURL)
	}
}

func TestProjectHandlerUnpublish(t *testing.T) {
	project := &models.Project{ID: 1, Name: "Altos", IsPublished: true, IsActive: true}
	handler := newProjectHandler(t, project)

	c, w := projectContext(http.MethodPost, "1")
	handler.Unpublish(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var payload struct {
		Project models.ProjectResponse `json:"project"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Project.IsPublished || !payload.Project.IsActive {
		t.Fatalf("published=%v active=%v, want published false and active true", payload.Project.IsPublished, payload.Project.IsActive)
	}
}
