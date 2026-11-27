package handlers

import (
	"errors"
	"net/http"
	"time"

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
// @Description Public customer settings: support contact, reservation and notification policy, regional defaults, legal data and terms content. Includes `supports_partial_updates` so clients can detect the merge behaviour.
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
	SupportWhatsApp                   *string  `json:"support_whatsapp"`
	SupportEmail                      *string  `json:"support_email"`
	ReservationHoldHours              *int     `json:"reservation_hold_hours"`
	ReservationMinAmount              *float64 `json:"reservation_min_amount"`
	ReservationDueDays                *int     `json:"reservation_due_days"`
	DownPaymentDueDays                *int     `json:"down_payment_due_days"`
	DownPaymentDueDaysWithReservation *int     `json:"down_payment_due_days_with_reservation"`
	EmailNotificationsEnabled         *bool    `json:"email_notifications_enabled"`
	OverdueRemindersEnabled           *bool    `json:"overdue_reminders_enabled"`
	UpcomingReminderDays              *int     `json:"upcoming_reminder_days"`
	DefaultCurrency                   *string  `json:"default_currency"`
	DefaultLocale                     *string  `json:"default_locale"`
	Timezone                          *string  `json:"timezone"`
	LegalName                         *string  `json:"legal_name"`
	LegalTaxID                        *string  `json:"legal_tax_id"`
	LegalAddress                      *string  `json:"legal_address"`
	Website                           *string  `json:"website"`
	TermsEffectiveDate                *string  `json:"terms_effective_date"`
	TermsContentES                    *string  `json:"terms_content_es"`
	TermsContentEN                    *string  `json:"terms_content_en"`
}

func isSettingsValidationError(err error) bool {
	return errors.Is(err, services.ErrInvalidSupportWhatsApp) ||
		errors.Is(err, services.ErrInvalidSupportEmail) ||
		errors.Is(err, services.ErrInvalidHoldHours) ||
		errors.Is(err, services.ErrInvalidMinAmount) ||
		errors.Is(err, services.ErrInvalidPaymentWindow) ||
		errors.Is(err, services.ErrInvalidReminderDays) ||
		errors.Is(err, services.ErrInvalidDefaultCurrency) ||
		errors.Is(err, services.ErrInvalidDefaultLocale) ||
		errors.Is(err, services.ErrInvalidTimezone) ||
		errors.Is(err, services.ErrInvalidWebsite) ||
		errors.Is(err, services.ErrInvalidTermsDate) ||
		errors.Is(err, services.ErrLegalNameTooLong) ||
		errors.Is(err, services.ErrLegalTaxIDTooLong) ||
		errors.Is(err, services.ErrLegalAddressTooLong) ||
		errors.Is(err, services.ErrWebsiteTooLong) ||
		errors.Is(err, services.ErrTermsContentTooLong)
}

// @Summary Update general settings
// @Description Update support contact, reservation, notification and regional policy, legal data and terms content (Admin). Sent fields are merged; omitted fields keep their stored value.
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
		WriteBindError(c, err, "Formato JSON inválido: ")
		return
	}
	// A nil pointer means "not sent". An empty string means "clear it", which
	// ParseTermsEffectiveDate already accepts as "no effective date".
	var termsDate *time.Time
	if req.TermsEffectiveDate != nil {
		parsed, err := services.ParseTermsEffectiveDate(*req.TermsEffectiveDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		termsDate = parsed
	}
	patch := &models.GeneralSettingsPatch{
		SupportWhatsApp:                   req.SupportWhatsApp,
		SupportEmail:                      req.SupportEmail,
		ReservationHoldHours:              req.ReservationHoldHours,
		ReservationMinAmount:              req.ReservationMinAmount,
		ReservationDueDays:                req.ReservationDueDays,
		DownPaymentDueDays:                req.DownPaymentDueDays,
		DownPaymentDueDaysWithReservation: req.DownPaymentDueDaysWithReservation,
		EmailNotificationsEnabled:         req.EmailNotificationsEnabled,
		OverdueRemindersEnabled:           req.OverdueRemindersEnabled,
		UpcomingReminderDays:              req.UpcomingReminderDays,
		DefaultCurrency:                   req.DefaultCurrency,
		DefaultLocale:                     req.DefaultLocale,
		Timezone:                          req.Timezone,
		LegalName:                         req.LegalName,
		LegalTaxID:                        req.LegalTaxID,
		LegalAddress:                      req.LegalAddress,
		Website:                           req.Website,
		TermsEffectiveDate:                termsDate,
		TermsContentES:                    req.TermsContentES,
		TermsContentEN:                    req.TermsContentEN,
	}
	settings, err := h.service.Update(c.Request.Context(), middleware.GetUserID(c), patch)
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
