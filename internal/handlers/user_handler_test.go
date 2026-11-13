package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUserRequestBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
		expectedName   string
		expectError    bool
	}{
		{
			name: "Support full_name (snake_case)",
			payload: map[string]interface{}{
				"email":     "test@example.com",
				"password":  "password123",
				"full_name": "Snake Case User",
			},
			expectedStatus: http.StatusOK, // We'll mock the rest so it doesn't fail on service call
			expectedName:   "Snake Case User",
		},
		{
			name: "Support FullName (PascalCase)",
			payload: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
				"FullName": "Pascal Case User",
			},
			expectedStatus: http.StatusOK,
			expectedName:   "Pascal Case User",
		},
		{
			name: "Error when both missing",
			payload: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			jsonBytes, _ := json.Marshal(tt.payload)
			c.Request, _ = http.NewRequest("POST", "/users", bytes.NewBuffer(jsonBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			var req CreateUserRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				if !tt.expectError {
					t.Errorf("Unexpected error binding: %v", err)
				}
				return
			}

			if req.FullName == "" && req.FullNamePascal != "" {
				req.FullName = req.FullNamePascal
			}

			if tt.expectError {
				if req.FullName != "" {
					t.Errorf("Expected validation error but got FullName: %s", req.FullName)
				}
			} else {
				assert.Equal(t, tt.expectedName, req.FullName)
			}
		})
	}
}

type mockUserRepo struct {
	repository.UserRepository
	mockList        func(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error)
	mockFindByID    func(ctx context.Context, id uint) (*models.User, error)
	mockUpdate      func(ctx context.Context, user *models.User) error
	mockClientCount func(ctx context.Context, creatorIDs []uint) (map[uint]int64, error)
}

func (m *mockUserRepo) List(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error) {
	if m.mockList != nil {
		return m.mockList(ctx, query)
	}
	return []models.User{}, 0, nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uint) (*models.User, error) {
	if m.mockFindByID != nil {
		return m.mockFindByID(ctx, id)
	}
	return nil, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *models.User) error {
	if m.mockUpdate != nil {
		return m.mockUpdate(ctx, user)
	}
	return nil
}

func (m *mockUserRepo) CountActiveClientsByCreator(ctx context.Context, creatorIDs []uint) (map[uint]int64, error) {
	if m.mockClientCount != nil {
		return m.mockClientCount(ctx, creatorIDs)
	}
	return map[uint]int64{}, nil
}

func TestUserHandler_Index_DefaultStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockUserRepo{}
	userService := services.NewUserService(mockRepo, nil, nil, nil, nil, nil) // Updated with nil ContractRepo and ImageService
	handler := NewUserHandler(userService, nil, nil, nil)

	var capturedStatus string
	mockRepo.mockList = func(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error) {
		capturedStatus = query.Filters["status"]
		return []models.User{}, 0, nil
	}

	// Test Case 1: No status provided -> should default to "active"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users", nil)
	handler.Index(c)
	assert.Equal(t, models.StatusActive, capturedStatus)

	// Test Case 2: Status "all" provided -> should be empty string (no filter)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users?status=all", nil)
	handler.Index(c)
	assert.Equal(t, "", capturedStatus)

	// Test Case 3: Specific status provided -> should use it
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users?status=inactive", nil)
	handler.Index(c)
	assert.Equal(t, "inactive", capturedStatus)
}

func TestUserHandler_Index_IncludesClientsCount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockUserRepo{}
	var countedIDs []uint
	mockRepo.mockList = func(context.Context, *repository.ListQuery) ([]models.User, int64, error) {
		return []models.User{
			{ID: 10, FullName: "Seller One", Role: models.RoleSeller},
			{ID: 11, FullName: "Seller Two", Role: models.RoleSeller},
		}, 2, nil
	}
	mockRepo.mockClientCount = func(_ context.Context, creatorIDs []uint) (map[uint]int64, error) {
		countedIDs = creatorIDs
		return map[uint]int64{10: 3}, nil
	}

	userService := services.NewUserService(mockRepo, nil, nil, nil, nil, nil)
	handler := NewUserHandler(userService, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users", nil)
	handler.Index(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.ElementsMatch(t, []uint{10, 11}, countedIDs)

	var payload struct {
		Users []struct {
			ID           uint  `json:"id"`
			ClientsCount int64 `json:"clients_count"`
		} `json:"users"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.Len(t, payload.Users, 2)
	assert.Equal(t, int64(3), payload.Users[0].ClientsCount)
	assert.Equal(t, int64(0), payload.Users[1].ClientsCount)
}

func TestUserHandler_Index_ClientsCountNullOnFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockUserRepo{}
	mockRepo.mockList = func(context.Context, *repository.ListQuery) ([]models.User, int64, error) {
		return []models.User{{ID: 20, Role: models.RoleSeller}}, 1, nil
	}
	mockRepo.mockClientCount = func(context.Context, []uint) (map[uint]int64, error) {
		return nil, errors.New("count failed")
	}

	userService := services.NewUserService(mockRepo, nil, nil, nil, nil, nil)
	handler := NewUserHandler(userService, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users", nil)
	handler.Index(c)

	var payload struct {
		Users []struct {
			ClientsCount *int64 `json:"clients_count"`
		} `json:"users"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Len(t, payload.Users, 1)
	assert.Nil(t, payload.Users[0].ClientsCount)
}

func TestUserHandler_Show_IncludesClientsCount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockUserRepo{}
	mockRepo.mockFindByID = func(context.Context, uint) (*models.User, error) {
		return &models.User{ID: 4, FullName: "Seller", Role: models.RoleSeller}, nil
	}
	mockRepo.mockClientCount = func(context.Context, []uint) (map[uint]int64, error) {
		return map[uint]int64{4: 7}, nil
	}

	userService := services.NewUserService(mockRepo, nil, nil, nil, nil, nil)
	handler := NewUserHandler(userService, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/users/4", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "4"}}
	handler.Show(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		User struct {
			ClientsCount int64 `json:"clients_count"`
		} `json:"user"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.Equal(t, int64(7), payload.User.ClientsCount)
}

func TestUploadProfilePicture(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create temp dir for uploads
	tempDir := t.TempDir()
	imageService := services.NewImageService(tempDir)

	mockRepo := &mockUserRepo{}
	userService := services.NewUserService(mockRepo, nil, nil, nil, nil, imageService)
	handler := NewUserHandler(userService, nil, nil, nil)

	userID := uint(1)
	mockUser := &models.User{ID: userID, Email: "test@example.com"}

	mockRepo.mockFindByID = func(ctx context.Context, id uint) (*models.User, error) {
		if id == userID {
			return mockUser, nil
		}
		return nil, nil
	}

	mockRepo.mockUpdate = func(ctx context.Context, user *models.User) error {
		assert.NotNil(t, user.ProfilePicture)
		assert.NotNil(t, user.ProfilePictureThumb)
		return nil
	}

	// Mock file upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Minimal valid 1x1 PNG
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // Magic
		0x00, 0x00, 0x00, 0x0d, // IHDR length
		0x49, 0x48, 0x44, 0x52, // IHDR chunk type
		0x00, 0x00, 0x00, 0x01, // Width: 1
		0x00, 0x00, 0x00, 0x01, // Height: 1
		0x08, 0x06, 0x00, 0x00, 0x00, // Bit depth, Color type, Compression, Filter, Interlace
		0x1f, 0x15, 0xc4, 0x89, // CRC
		0x00, 0x00, 0x00, 0x0a, // IDAT length
		0x49, 0x44, 0x41, 0x54, // IDAT chunk type
		0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, // Compressed data
		0x0d, 0x0a, 0x2d, 0xb4, // CRC
		0x00, 0x00, 0x00, 0x00, // IEND length
		0x49, 0x45, 0x4e, 0x44, // IEND chunk type
		0xae, 0x42, 0x60, 0x82, // CRC
	}
	part, err := writer.CreateFormFile("image", "test.png")
	assert.NoError(t, err)
	_, err = part.Write(minimalPNG)
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/users/1/picture", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}
	c.Set("userID", uint(1)) // Mock authenticated user (self update)

	handler.UploadProfilePicture(c)

	assert.Equal(t, http.StatusOK, w.Code)
}
