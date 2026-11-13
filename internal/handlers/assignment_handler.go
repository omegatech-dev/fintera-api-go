package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
)

// AssignmentHandler serves the seller assignment inbox and responses.
type AssignmentHandler struct {
	assignments *services.AssignmentService
}

// NewAssignmentHandler creates the assignment handler.
func NewAssignmentHandler(assignments *services.AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{assignments: assignments}
}

// @Summary List pending assignments
// @Description Pending reservations assigned to the authenticated seller. Admins can list all or filter by seller_id.
// @Tags Assignments
// @Produce json
// @Param seller_id query int false "Seller ID (admin only)"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /assignments/pending [get]
func (h *AssignmentHandler) Pending(c *gin.Context) {
	role := strings.ToLower(middleware.GetUserRole(c))
	var (
		assignments []models.Assignment
		err         error
	)

	if role == "admin" {
		if raw := strings.TrimSpace(c.Query("seller_id")); raw != "" {
			id, parseErr := strconv.ParseUint(raw, 10, 32)
			if parseErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "seller_id inválido"})
				return
			}
			assignments, err = h.assignments.PendingForSeller(c.Request.Context(), uint(id))
		} else {
			assignments, err = h.assignments.PendingAll(c.Request.Context())
		}
	} else {
		sellerID := middleware.GetUserID(c)
		if sellerID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesión inválida"})
			return
		}
		assignments, err = h.assignments.PendingForSeller(c.Request.Context(), sellerID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar las asignaciones"})
		return
	}

	responses := make([]models.AssignmentResponse, 0, len(assignments))
	for i := range assignments {
		responses = append(responses, assignments[i].ToResponse())
	}
	c.JSON(http.StatusOK, gin.H{"assignments": responses})
}

// @Summary Accept assignment
// @Description Accept a pending reservation assignment. Idempotent for an already accepted assignment.
// @Tags Assignments
// @Produce json
// @Param id path int true "Assignment ID"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /assignments/{id}/accept [post]
func (h *AssignmentHandler) Accept(c *gin.Context) {
	assignment, err := h.respond(c, true)
	if assignment == nil {
		return
	}
	if err != nil {
		writeAssignmentError(c, err, assignment)
		return
	}
	c.JSON(http.StatusOK, gin.H{"assignment": assignment.ToResponse(), "message": "Asignación aceptada"})
}

// @Summary Reject assignment
// @Description Reject a pending reservation assignment; the reservation is offered to the next seller. Idempotent for an already expired assignment.
// @Tags Assignments
// @Produce json
// @Param id path int true "Assignment ID"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /assignments/{id}/reject [post]
func (h *AssignmentHandler) Reject(c *gin.Context) {
	assignment, err := h.respond(c, false)
	if assignment == nil {
		return
	}
	if err != nil {
		writeAssignmentError(c, err, assignment)
		return
	}
	c.JSON(http.StatusOK, gin.H{"assignment": assignment.ToResponse(), "message": "Asignación rechazada"})
}

func (h *AssignmentHandler) respond(c *gin.Context, accept bool) (*models.Assignment, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de asignación inválido"})
		return nil, nil
	}
	actorID := middleware.GetUserID(c)
	if actorID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesión inválida"})
		return nil, nil
	}
	isAdmin := middleware.IsAdmin(c)
	if accept {
		return h.assignments.Accept(c.Request.Context(), uint(id), actorID, isAdmin)
	}
	return h.assignments.Reject(c.Request.Context(), uint(id), actorID, isAdmin)
}

func writeAssignmentError(c *gin.Context, err error, assignment *models.Assignment) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, services.ErrAssignmentNotFound):
		status = http.StatusNotFound
	case errors.Is(err, services.ErrAssignmentNotOwned):
		status = http.StatusForbidden
	case errors.Is(err, services.ErrAssignmentExpired), errors.Is(err, services.ErrAssignmentNotPending):
		status = http.StatusConflict
	}
	payload := gin.H{"error": err.Error()}
	if assignment != nil {
		payload["assignment"] = assignment.ToResponse()
	}
	c.JSON(status, payload)
}
