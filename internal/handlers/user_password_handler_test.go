package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func changeOwnPasswordContext(body string, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change_password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID > 0 {
		c.Set("userID", userID)
	}
	return c, w
}

func TestChangeOwnPasswordRequiresSession(t *testing.T) {
	handler := NewUserHandler(nil, nil, nil, nil)
	c, w := changeOwnPasswordContext(`{"current_password":"old","new_password":"newpass123"}`, 0)

	handler.ChangeOwnPassword(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestChangeOwnPasswordRequiresCurrentPassword(t *testing.T) {
	handler := NewUserHandler(nil, nil, nil, nil)
	c, w := changeOwnPasswordContext(`{"new_password":"newpass123"}`, 5)

	handler.ChangeOwnPassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "contraseña actual")
}

func TestChangeOwnPasswordValidatesNewPassword(t *testing.T) {
	handler := NewUserHandler(nil, nil, nil, nil)
	c, w := changeOwnPasswordContext(`{"current_password":"old","new_password":"123"}`, 5)

	handler.ChangeOwnPassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChangeOwnPasswordValidatesJSON(t *testing.T) {
	handler := NewUserHandler(nil, nil, nil, nil)
	c, w := changeOwnPasswordContext(`not-json`, 5)

	handler.ChangeOwnPassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
