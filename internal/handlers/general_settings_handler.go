package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
)

// GeneralSettingsHandler exposes the deployment-wide customer settings.
type GeneralSettingsHandler struct {
	service *services.GeneralSettingsService
}

// NewGeneralSettingsHandler creates the handler.
func NewGeneralSettingsHandler(service *services.GeneralSettingsService) *GeneralSettingsHandler {
	return &GeneralSettingsHandler{service: service}
}

// @Summary Get general settings
// @Description Public customer settings: support contact, reservation hold, legal data and terms content.
// @Tags Settings
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /settings [get]
func (h *GeneralSettingsHandler) Show(c *gin.Context) {
	settings, err := h.service.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener la configuración"})
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, gin.H{"settings": settings.ToResponse()})
}

type generalSettingsUpdateRequest struct {
	SupportWhatsApp      string `json:"support_whatsapp"`
	SupportEmail         string `json:"support_email"`
	ReservationHoldHours int    `json:"reservation_hold_hours"`
	LegalName            string `json:"legal_name"`
	LegalTaxID           string `json:"legal_tax_id"`
	LegalAddress         string `json:"legal_address"`
	Website              string `json:"website"`
	TermsEffectiveDate   string `json:"terms_effective_date"`
	TermsContentES       string `json:"terms_content_es"`
	TermsContentEN       string `json:"terms_content_en"`
}

func optionalTrimmedString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isSettingsValidationError(err error) bool {
	return errors.Is(err, services.ErrInvalidSupportWhatsApp) ||
		errors.Is(err, services.ErrInvalidSupportEmail) ||
		errors.Is(err, services.ErrInvalidHoldHours) ||
		errors.Is(err, services.ErrInvalidWebsite) ||
		errors.Is(err, services.ErrInvalidTermsDate) ||
		errors.Is(err, services.ErrLegalNameTooLong) ||
		errors.Is(err, services.ErrLegalTaxIDTooLong) ||
		errors.Is(err, services.ErrLegalAddressTooLong) ||
		errors.Is(err, services.ErrWebsiteTooLong) ||
		errors.Is(err, services.ErrTermsContentTooLong)
}

// @Summary Update general settings
// @Description Update support contact, reservation hold, legal data and terms content (Admin).
// @Tags Settings
// @Accept json
// @Produce json
// @Param request body generalSettingsUpdateRequest true "Settings data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Security BearerAuth
// @Router /settings [put]
func (h *GeneralSettingsHandler) Update(c *gin.Context) {
	var req generalSettingsUpdateRequest
	if err := BindNestedOrFlat(c, "settings", &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato JSON inválido: " + err.Error()})
		return
	}
	termsDate, err := services.ParseTermsEffectiveDate(req.TermsEffectiveDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settings, err := h.service.Update(c.Request.Context(), middleware.GetUserID(c), models.GeneralSettings{
		SupportWhatsApp:      req.SupportWhatsApp,
		SupportEmail:         req.SupportEmail,
		ReservationHoldHours: req.ReservationHoldHours,
		LegalName:            req.LegalName,
		LegalTaxID:           req.LegalTaxID,
		LegalAddress:         optionalTrimmedString(req.LegalAddress),
		Website:              req.Website,
		TermsEffectiveDate:   termsDate,
		TermsContentES:       optionalTrimmedString(req.TermsContentES),
		TermsContentEN:       optionalTrimmedString(req.TermsContentEN),
	})
	if err != nil {
		if isSettingsValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"settings": settings.ToResponse(),
		"message":  "Configuración actualizada",
	})
}
