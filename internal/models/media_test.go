package models

import "testing"

func TestPhotoResponseIncludesIDAndPosition(t *testing.T) {
	alt := "Front view"

	projectPhoto := ProjectPhoto{ID: 7, Position: 3, FilePath: "projects/1/cover.jpg", Alt: &alt}
	projectResp := projectPhoto.ToResponse()
	if projectResp.ID != 7 {
		t.Errorf("project photo response ID = %d, want 7", projectResp.ID)
	}
	if projectResp.Position != 3 {
		t.Errorf("project photo response position = %d, want 3", projectResp.Position)
	}
	if projectResp.Alt == nil || *projectResp.Alt != alt {
		t.Errorf("project photo response alt = %v, want %q", projectResp.Alt, alt)
	}

	lotPhoto := LotPhoto{ID: 9, Position: 1, FilePath: "lots/2/front.jpg"}
	lotResp := lotPhoto.ToResponse()
	if lotResp.ID != 9 {
		t.Errorf("lot photo response ID = %d, want 9", lotResp.ID)
	}
	if lotResp.Position != 1 {
		t.Errorf("lot photo response position = %d, want 1", lotResp.Position)
	}
}
