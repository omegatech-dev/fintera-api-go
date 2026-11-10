package services

import (
	"mime/multipart"
	"net/textproto"
	"testing"
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
