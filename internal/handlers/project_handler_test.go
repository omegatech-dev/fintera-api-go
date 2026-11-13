package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

// projectRepoForHandlerTests is a configurable ProjectRepository used by the
// list/create/update/delete handler tests.
type projectRepoForHandlerTests struct {
	project *models.Project
	list    []models.Project
	total   int64
	err     error
	created []*models.Project
	updated []*models.Project
	deleted []uint
}

func (s *projectRepoForHandlerTests) FindByID(context.Context, uint) (*models.Project, error) {
	return s.project, s.err
}

func (s *projectRepoForHandlerTests) FindByGUID(context.Context, string) (*models.Project, error) {
	return s.project, s.err
}

func (s *projectRepoForHandlerTests) FindPublishedByGUID(context.Context, string) (*models.Project, error) {
	return s.project, s.err
}

func (s *projectRepoForHandlerTests) ListPublished(context.Context) ([]models.Project, error) {
	return s.list, s.err
}

func (s *projectRepoForHandlerTests) Create(_ context.Context, project *models.Project) error {
	if project.ID == 0 {
		project.ID = 1
	}
	s.created = append(s.created, project)
	return s.err
}

func (s *projectRepoForHandlerTests) Update(_ context.Context, project *models.Project) error {
	s.updated = append(s.updated, project)
	return s.err
}

func (s *projectRepoForHandlerTests) UpdateProjectAndLots(_ context.Context, project *models.Project, _ []models.Lot) error {
	s.updated = append(s.updated, project)
	return s.err
}

func (s *projectRepoForHandlerTests) Delete(_ context.Context, id uint) error {
	s.deleted = append(s.deleted, id)
	return s.err
}

func (s *projectRepoForHandlerTests) List(context.Context, *repository.ListQuery) ([]models.Project, int64, error) {
	return s.list, s.total, s.err
}

func (s *projectRepoForHandlerTests) FindAll(context.Context) ([]models.Project, error) {
	return s.list, s.err
}

func newProjectHandlerWithRepo(t *testing.T, repo repository.ProjectRepository, lotRepo repository.LotRepository, photos ...models.ProjectPhoto) (*ProjectHandler, sqlmock.Sqlmock) {
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

	audit := services.NewAuditService(gdb)
	projectService := services.NewProjectService(repo, lotRepo, audit)
	mediaService := services.NewMediaService(nil, &stubProjectPhotoRepo{photos: photos}, nil, nil, nil, nil, nil)
	return NewProjectHandler(projectService, mediaService), mockDB
}

func projectRequestContext(method, body string, params gin.Params, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = params
	context.Set("userID", userID)
	return context, recorder
}

func TestProjectHandler_Index(t *testing.T) {
	repo := &projectRepoForHandlerTests{
		list:  []models.Project{{ID: 1, Name: "Altos"}},
		total: 25,
	}
	handler, _ := newProjectHandlerWithRepo(t, repo, &stubLotRepo{})

	c, w := projectRequestContext(http.MethodGet, "", nil, 1)
	handler.Index(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var payload struct {
		Projects   []models.ProjectResponse `json:"projects"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Projects) != 1 {
		t.Fatalf("projects = %d, want 1", len(payload.Projects))
	}
	if payload.Pagination.Total != 25 {
		t.Fatalf("total = %d, want 25", payload.Pagination.Total)
	}
}

func TestProjectHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &projectRepoForHandlerTests{}
		handler, mockDB := newProjectHandlerWithRepo(t, repo, &stubLotRepo{})
		mockDB.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		c, w := projectRequestContext(http.MethodPost,
			`{"project":{"name":"Altos","lot_count":2,"price_per_square_unit":100,"measurement_unit":"m2"}}`, nil, 1)
		handler.Create(c)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
		}
		if len(repo.created) != 1 {
			t.Fatalf("created = %d, want 1", len(repo.created))
		}
		if repo.created[0].GUID == "" {
			t.Fatal("expected a generated GUID")
		}
		if len(repo.created[0].Lots) != 2 {
			t.Fatalf("lots = %d, want 2", len(repo.created[0].Lots))
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		repo := &projectRepoForHandlerTests{}
		handler, _ := newProjectHandlerWithRepo(t, repo, &stubLotRepo{})

		c, w := projectRequestContext(http.MethodPost, `not-json`, nil, 1)
		handler.Create(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
		}
		if len(repo.created) != 0 {
			t.Fatalf("created = %d, want 0", len(repo.created))
		}
	})
}

func TestProjectHandler_Update(t *testing.T) {
	params := gin.Params{{Key: "project_id", Value: "1"}}

	t.Run("success", func(t *testing.T) {
		repo := &projectRepoForHandlerTests{project: &models.Project{
			ID: 1, GUID: "existing-guid", MeasurementUnit: "m2", PricePerSquareUnit: 100,
		}}
		handler, mockDB := newProjectHandlerWithRepo(t, repo, &stubLotRepo{})
		mockDB.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		c, w := projectRequestContext(http.MethodPut,
			`{"project":{"name":"Altos actualizado","measurement_unit":"m2","price_per_square_unit":100}}`, params, 1)
		handler.Update(c)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
		}
		if len(repo.updated) != 1 {
			t.Fatalf("updated = %d, want 1", len(repo.updated))
		}
		if repo.updated[0].GUID != "existing-guid" {
			t.Fatalf("GUID = %q, want existing-guid", repo.updated[0].GUID)
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		repo := &projectRepoForHandlerTests{}
		handler, _ := newProjectHandlerWithRepo(t, repo, &stubLotRepo{})

		c, w := projectRequestContext(http.MethodPut, `not-json`, params, 1)
		handler.Update(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &projectRepoForHandlerTests{err: errors.New("boom")}
		handler, _ := newProjectHandlerWithRepo(t, repo, &stubLotRepo{})

		c, w := projectRequestContext(http.MethodPut, `{"project":{"name":"Altos"}}`, params, 1)
		handler.Update(c)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
		}
	})
}

func TestProjectHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &projectRepoForHandlerTests{}
		handler, mockDB := newProjectHandlerWithRepo(t, repo, &stubLotRepo{})
		mockDB.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		c, w := projectRequestContext(http.MethodDelete, "", gin.Params{{Key: "project_id", Value: "9"}}, 1)
		handler.Delete(c)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
		}
		if len(repo.deleted) != 1 || repo.deleted[0] != 9 {
			t.Fatalf("deleted = %v, want [9]", repo.deleted)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &projectRepoForHandlerTests{err: errors.New("boom")}
		handler, _ := newProjectHandlerWithRepo(t, repo, &stubLotRepo{})

		c, w := projectRequestContext(http.MethodDelete, "", gin.Params{{Key: "project_id", Value: "9"}}, 1)
		handler.Delete(c)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
		}
	})
}
