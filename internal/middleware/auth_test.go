package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func performRequest(middleware gin.HandlerFunc, userID uint, role, targetID string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/users/:user_id/contracts", func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", role)
	}, middleware, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/"+targetID+"/contracts", nil)
	router.ServeHTTP(w, req)
	return w
}

func TestRequireAdminSellerOrOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		userID     uint
		role       string
		targetID   string
		wantStatus int
	}{
		{name: "owner can read own data", userID: 4, role: "user", targetID: "4", wantStatus: http.StatusOK},
		{name: "other customer is forbidden", userID: 4, role: "user", targetID: "9", wantStatus: http.StatusForbidden},
		{name: "admin can read anyone", userID: 1, role: "admin", targetID: "9", wantStatus: http.StatusOK},
		{name: "seller keeps existing access", userID: 2, role: "seller", targetID: "9", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := performRequest(RequireAdminSellerOrOwner(), tt.userID, tt.role, tt.targetID).Code; got != tt.wantStatus {
				t.Fatalf("status = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RateLimit(2, time.Minute))
	router.GET("/public/projects", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	request := func() int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/public/projects", nil)
		req.RemoteAddr = "203.0.113.7:1234"
		router.ServeHTTP(w, req)
		return w.Code
	}

	if status := request(); status != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", status)
	}
	if status := request(); status != http.StatusOK {
		t.Fatalf("second request status = %d, want 200", status)
	}
	// gin's test client IP resolution uses RemoteAddr; if it is not honored the
	// limiter still sees a stable fallback IP, so the third request is limited.
	if status := request(); status != http.StatusTooManyRequests {
		t.Fatalf("third request status = %d, want 429", status)
	}
}
