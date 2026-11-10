package services

import (
	"context"
	"mime/multipart"
	"net/textproto"
	"strconv"
	"strings"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
)

func TestDrawingType(t *testing.T) {
	cases := map[string]string{
		"plan.dwg": "dwg",
		"plan.DXF": "dxf",
		"plan.pdf": "pdf",
		"plan.svg": "svg",
	}
	for filename, expected := range cases {
		got, err := drawingType(filename)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", filename, err)
		}
		if got != expected {
			t.Fatalf("%s: expected %s, got %s", filename, expected, got)
		}
	}

	if _, err := drawingType("plan.exe"); err != ErrUnsupportedMedia {
		t.Fatalf("expected unsupported media error, got %v", err)
	}
}

type reorderItem struct {
	ID       uint
	Position int
}

func reorderItems() []reorderItem {
	return []reorderItem{
		{ID: 1, Position: 0},
		{ID: 2, Position: 1},
		{ID: 3, Position: 2},
	}
}

func reorderIDs(items []reorderItem) []uint {
	ids := make([]uint, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	return ids
}

func TestReorderByID(t *testing.T) {
	idOf := func(item reorderItem) uint { return item.ID }
	positionOf := func(item reorderItem) int { return item.Position }

	moved, position, ok := reorderByID(reorderItems(), 1, 1, idOf, positionOf)
	if !ok || position != 1 {
		t.Fatalf("move down: ok=%v position=%d", ok, position)
	}
	if got := reorderIDs(moved); got[0] != 2 || got[1] != 1 || got[2] != 3 {
		t.Fatalf("move down order = %v, want [2 1 3]", got)
	}

	moved, position, ok = reorderByID(moved, 1, 0, idOf, positionOf)
	if !ok || position != 0 {
		t.Fatalf("move up: ok=%v position=%d", ok, position)
	}
	if got := reorderIDs(moved); got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("move up order = %v, want [1 2 3]", got)
	}

	moved, position, ok = reorderByID(reorderItems(), 3, -1, idOf, positionOf)
	if !ok || position != 0 {
		t.Fatalf("cover: ok=%v position=%d", ok, position)
	}
	if got := reorderIDs(moved); got[0] != 3 {
		t.Fatalf("cover order = %v, want 3 first", got)
	}

	moved, position, ok = reorderByID(reorderItems(), 1, 99, idOf, positionOf)
	if !ok || position != 2 {
		t.Fatalf("clamp: ok=%v position=%d", ok, position)
	}
	if got := reorderIDs(moved); got[2] != 1 {
		t.Fatalf("clamp order = %v, want 1 last", got)
	}

	if _, _, ok := reorderByID(reorderItems(), 99, 0, idOf, positionOf); ok {
		t.Fatal("expected missing id to report not found")
	}
}

type coverPhotoRepo struct {
	photos []models.ProjectPhoto
}

func (r *coverPhotoRepo) Create(ctx context.Context, photo *models.ProjectPhoto) error { return nil }

func (r *coverPhotoRepo) FindByID(ctx context.Context, id uint) (*models.ProjectPhoto, error) {
	return nil, nil
}

func (r *coverPhotoRepo) ListByProject(ctx context.Context, projectID uint) ([]models.ProjectPhoto, error) {
	return r.photos, nil
}

func (r *coverPhotoRepo) Update(ctx context.Context, photo *models.ProjectPhoto) error { return nil }

func (r *coverPhotoRepo) Delete(ctx context.Context, id uint) error { return nil }

func (r *coverPhotoRepo) NextPosition(ctx context.Context, projectID uint) (int, error) {
	return 0, nil
}

func TestProjectCardPhotos(t *testing.T) {
	service := &MediaService{projectPhoto: &coverPhotoRepo{}}

	photos, err := service.ProjectCardPhotos(context.Background(), 1)
	if err != nil {
		t.Fatalf("empty gallery: unexpected error: %v", err)
	}
	if len(photos) != 0 {
		t.Fatalf("empty gallery length = %d, want 0", len(photos))
	}

	gallery := make([]models.ProjectPhoto, 0, 12)
	for i := 1; i <= 12; i++ {
		gallery = append(gallery, models.ProjectPhoto{
			ID:       uint(i),
			Position: i,
			FilePath: "projects/1/photo-" + strconv.Itoa(i) + ".jpg",
		})
	}
	service.projectPhoto = &coverPhotoRepo{photos: gallery}

	photos, err = service.ProjectCardPhotos(context.Background(), 1)
	if err != nil {
		t.Fatalf("gallery: unexpected error: %v", err)
	}
	if len(photos) != ProjectCardPhotoLimit {
		t.Fatalf("gallery length = %d, want %d", len(photos), ProjectCardPhotoLimit)
	}
	if !strings.Contains(photos[0].URL, "photo-1.jpg") {
		t.Fatalf("first photo = %q, want photo-1.jpg", photos[0].URL)
	}
	if photos[0].Position != 1 {
		t.Fatalf("first photo position = %d, want 1", photos[0].Position)
	}
}

func photoHeader(filename, contentType string, size int64) *multipart.FileHeader {
	return &multipart.FileHeader{
		Filename: filename,
		Size:     size,
		Header:   textproto.MIMEHeader{"Content-Type": []string{contentType}},
	}
}

func TestValidatePhoto(t *testing.T) {
	if err := validatePhoto(photoHeader("cover.jpg", "image/jpeg", 1024)); err != nil {
		t.Fatalf("expected valid jpeg, got %v", err)
	}
	if err := validatePhoto(photoHeader("cover.webp", "image/webp", 1024)); err != nil {
		t.Fatalf("expected valid webp, got %v", err)
	}
	if err := validatePhoto(photoHeader("cover.gif", "image/gif", 1024)); err != ErrUnsupportedMedia {
		t.Fatalf("expected unsupported type, got %v", err)
	}
	if err := validatePhoto(photoHeader("cover.jpg", "image/jpeg", PhotoMaxBytes+1)); err != ErrMediaTooLarge {
		t.Fatalf("expected size error, got %v", err)
	}
}
