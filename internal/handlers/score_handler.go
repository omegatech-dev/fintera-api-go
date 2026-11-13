package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/services"
)

// ScoreHandler serves the audited admin score override.
type ScoreHandler struct {
	scores *services.ScoreService
}

// NewScoreHandler creates the score handler.
func NewScoreHandler(scores *services.ScoreService) *ScoreHandler {
	return &ScoreHandler{scores: scores}
}

type overrideScoreRequest struct {
	Score  *int   `json:"score"`
	Reason string `json:"reason"`
	Clear  bool   `json:"clear"`
}

// @Summary Override seller score
// @Description Admin override of a seller acceptance score. The change is audited and the nightly recompute skips active overrides.
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "Seller ID"
// @Param request body overrideScoreRequest true "Score override"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id}/score [patch]
func (h *ScoreHandler) Override(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de usuario inválido"})
		return
	}

	var req overrideScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}

	actorID := middleware.GetUserID(c)
	ip := c.ClientIP()
	agent := c.Request.UserAgent()

	if req.Clear {
		seller, err := h.scores.ClearOverride(c.Request.Context(), uint(id), actorID, ip, agent)
		if err != nil {
			writeScoreError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": seller.ToResponse(), "message": "Sobrescritura eliminada"})
		return
	}

	if req.Score == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "El puntaje es requerido"})
		return
	}
	seller, err := h.scores.Override(c.Request.Context(), uint(id), *req.Score, req.Reason, actorID, ip, agent)
	if err != nil {
		writeScoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": seller.ToResponse(), "message": "Puntaje actualizado"})
}

func writeScoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendedor no encontrado"})
	case errors.Is(err, services.ErrScoreOverrideInvalid):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "El puntaje debe estar entre 0 y 100 y el motivo es obligatorio"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el puntaje"})
	}
}
