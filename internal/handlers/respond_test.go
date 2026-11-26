package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runHandler serves a request whose handler ends with AbortIfCanceled(err), the
// way a real handler does, so the status is flushed by the engine.
func runHandler(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	router := gin.New()
	router.GET("/probe", func(c *gin.Context) {
		if AbortIfCanceled(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "boom"})
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestAbortIfCanceledEndsRequestAsClientClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// The repository error arrives wrapped, so cancellation must still be found.
	recorder := runHandler(t, fmt.Errorf("query failed: %w", context.Canceled))

	require.Equal(t, statusClientClosedRequest, recorder.Code, "an aborted client is not a server failure")
	assert.Empty(t, recorder.Body.String(), "an absent client gets no body")
}

func TestAbortIfCanceledLeavesRealErrorsToTheHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for name, err := range map[string]error{
		"no error":       nil,
		"database error": errors.New("connection refused"),
		"timeout":        fmt.Errorf("boom: %w", context.DeadlineExceeded),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := runHandler(t, err)

			assert.Equal(t, http.StatusInternalServerError, recorder.Code,
				"only cancellation is handled by AbortIfCanceled")
			assert.Contains(t, recorder.Body.String(), "boom")
		})
	}
}
