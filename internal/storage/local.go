package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage handles file storage on the local filesystem
type LocalStorage struct {
	basePath string
	root     *os.Root
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	// Ensure the base directory exists
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	root, err := os.OpenRoot(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage root: %w", err)
	}
	return &LocalStorage{basePath: basePath, root: root}, nil
}

// Upload saves a file and returns its relative path
func (s *LocalStorage) Upload(file multipart.File, header *multipart.FileHeader, subDir string) (string, error) {
	// Create subdirectory organized by year/month (e.g., "contracts/2026/01")
	relDir := filepath.Join(subDir, time.Now().Format("2006/01"))
	if err := s.root.MkdirAll(relDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%s%s", generateID(), ext)
	relPath := filepath.Join(relDir, filename)

	// Create destination file
	dst, err := s.root.OpenFile(relPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	// Copy content
	if _, err := io.Copy(dst, file); err != nil {
		// Clean up on failure
		_ = s.root.Remove(relPath)
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Return relative path for database storage
	return relPath, nil
}

// UploadFromBytes saves bytes to a file and returns its relative path
func (s *LocalStorage) UploadFromBytes(data []byte, filename string, subDir string) (string, error) {
	relDir := filepath.Join(subDir, time.Now().Format("2006/01"))
	if err := s.root.MkdirAll(relDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	ext := filepath.Ext(filename)
	uniqueFilename := fmt.Sprintf("%s%s", generateID(), ext)
	relPath := filepath.Join(relDir, uniqueFilename)

	if err := s.root.WriteFile(relPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return relPath, nil
}

// Download returns a file for reading
func (s *LocalStorage) Download(relativePath string) (*os.File, error) {
	return s.root.Open(relativePath)
}

// Delete removes a file
func (s *LocalStorage) Delete(relativePath string) error {
	return s.root.Remove(relativePath)
}

// Exists checks if a file exists
func (s *LocalStorage) Exists(relativePath string) bool {
	_, err := s.root.Stat(relativePath)
	return err == nil
}

// GetFullPath returns the absolute path for serving files (no traversal check)
func (s *LocalStorage) GetFullPath(relativePath string) string {
	return filepath.Join(s.basePath, relativePath)
}

// SafeFullPath returns the full path for serving files only if it is under basePath (prevents path traversal)
func (s *LocalStorage) SafeFullPath(relativePath string) (string, error) {
	cleanRel := filepath.Clean(relativePath)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path")
	}
	full := filepath.Join(s.basePath, cleanRel)
	baseAbs, err := filepath.Abs(s.basePath)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if fullAbs != baseAbs && !strings.HasPrefix(fullAbs, baseAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes storage base")
	}
	return full, nil
}

// GetSize returns the size of a file in bytes
func (s *LocalStorage) GetSize(relativePath string) (int64, error) {
	info, err := s.root.Stat(relativePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// generateID creates a unique identifier for filenames
func generateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate random id: %v", err))
	}
	return hex.EncodeToString(bytes)
}

// ValidContentTypes returns allowed MIME types for uploads
func ValidContentTypes() map[string]bool {
	return map[string]bool{
		"application/pdf": true,
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
	}
}

// MaxFileSize returns the maximum allowed file size (10MB)
func MaxFileSize() int64 {
	return 10 * 1024 * 1024 // 10 MB
}

// IsValidContentType checks if the content type is allowed
func IsValidContentType(contentType string) bool {
	return ValidContentTypes()[contentType]
}
