package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/services"
)

// CommissionHandler serves the global commission rules and the per-seller
// exceptions.
type CommissionHandler struct {
	commissions *services.CommissionService
}

// NewCommissionHandler creates the commission handler.
func NewCommissionHandler(commissions *services.CommissionService) *CommissionHandler {
	return &CommissionHandler{commissions: commissions}
}

// ShowRules returns the global commission configuration.
// @Summary Commission rules
// @Description Global default rate, unassigned rate, payment rule and acceptance window.
// @Tags Commissions
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /commissions/rules [get]
func (h *CommissionHandler) ShowRules(c *gin.Context) {
	rules, err := h.commissions.Rules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// UpdateRules persists the global commission rates.
// @Summary Update commission rules
// @Description Update the global default and unassigned rates. Admin only.
// @Tags Commissions
// @Accept json
// @Produce json
// @Param request body services.CommissionRulesInput true "Rules"
// @Success 200 {object} map[string]interface{}
// @Failure 422 {object} map[string]string
// @Security BearerAuth
// @Router /commissions/rules [put]
func (h *CommissionHandler) UpdateRules(c *gin.Context) {
	var input services.CommissionRulesInput
	if err := BindNestedOrFlat(c, "rules", &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}
	rules, err := h.commissions.UpdateRules(c.Request.Context(), middleware.GetUserID(c), input)
	if err != nil {
		writeCommissionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules, "message": "Reglas de comisión actualizadas"})
}

// ListExceptions returns the per-seller rate exceptions.
// @Summary List commission exceptions
// @Description Per-seller commission-rate exceptions. Admin only.
// @Tags Commissions
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /commissions/exceptions [get]
func (h *CommissionHandler) ListExceptions(c *gin.Context) {
	exceptions, err := h.commissions.ListExceptions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"exceptions": exceptions})
}

// UpsertException creates or updates a seller rate exception.
// @Summary Upsert commission exception
// @Description Create or replace the commission rate for one seller. Admin only.
// @Tags Commissions
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "seller_id and rate"
// @Success 200 {object} map[string]interface{}
// @Failure 422 {object} map[string]string
// @Security BearerAuth
// @Router /commissions/exceptions [post]
func (h *CommissionHandler) UpsertException(c *gin.Context) {
	var input struct {
		SellerID uint    `json:"seller_id"`
		Rate     float64 `json:"rate"`
	}
	if err := BindNestedOrFlat(c, "exception", &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}
	exception, err := h.commissions.UpsertException(c.Request.Context(), middleware.GetUserID(c), input.SellerID, input.Rate)
	if err != nil {
		writeCommissionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"exception": exception, "message": "Excepción guardada"})
}

// DeleteException removes a seller rate exception.
// @Summary Delete commission exception
// @Description Delete a seller rate exception and return the seller to the project/global rate. Admin only.
// @Tags Commissions
// @Produce json
// @Param id path int true "Exception ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /commissions/exceptions/{id} [delete]
func (h *CommissionHandler) DeleteException(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de excepción inválido"})
		return
	}
	if err := h.commissions.DeleteException(c.Request.Context(), middleware.GetUserID(c), uint(id)); err != nil {
		writeCommissionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Excepción eliminada"})
}

func writeCommissionError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, services.ErrCommissionExceptionMissing):
		status = http.StatusNotFound
	case errors.Is(err, services.ErrInvalidCommissionRate),
		errors.Is(err, services.ErrInvalidCommissionRules),
		errors.Is(err, services.ErrAssignmentSellerInactive):
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
