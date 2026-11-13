package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
)

// ReferralHandler serves the public referral link and the seller metrics.
type ReferralHandler struct {
	referral *services.ReferralService
}

// NewReferralHandler creates the referral handler.
func NewReferralHandler(referral *services.ReferralService) *ReferralHandler {
	return &ReferralHandler{referral: referral}
}

// @Summary Resolve referral link
// @Description Resolve an active seller referral GUID, record the visit and redirect to the portal with a 30-day attribution.
// @Tags Referrals
// @Produce json
// @Param guid path string true "Referral GUID"
// @Success 302 "Redirect to the customer portal"
// @Failure 404 {object} map[string]string
// @Failure 429 {object} map[string]string
// @Router /r/{guid} [get]
func (h *ReferralHandler) Redirect(c *gin.Context) {
	now := time.Now()
	_, _, err := h.referral.ResolveVisit(
		c.Request.Context(),
		c.Param("guid"),
		c.Request.URL.Path,
		c.ClientIP(),
		c.Request.UserAgent(),
		now,
	)
	if err != nil {
		writeReferralResolveError(c, err)
		return
	}

	http.SetCookie(c.Writer, h.referral.AttributionCookie(c.Param("guid"), now))
	c.Redirect(http.StatusFound, h.referral.PortalRedirectURL(c.Param("guid")))
}

// @Summary Record a referral visit
// @Description Record a visit for an active seller referral GUID without redirecting. The portal uses this when it already knows the GUID.
// @Tags Referrals
// @Produce json
// @Param guid path string true "Referral GUID"
// @Success 204 "Visit recorded"
// @Failure 404 {object} map[string]string
// @Failure 429 {object} map[string]string
// @Router /public/referrals/{guid}/visit [post]
func (h *ReferralHandler) Visit(c *gin.Context) {
	_, _, err := h.referral.ResolveVisit(
		c.Request.Context(),
		c.Param("guid"),
		c.Request.URL.Path,
		c.ClientIP(),
		c.Request.UserAgent(),
		time.Now(),
	)
	if err != nil {
		writeReferralResolveError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeReferralResolveError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrReferralNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Enlace de referido no encontrado"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo procesar el enlace de referido"})
}

// @Summary Seller referral metrics
// @Description Visits, attributed reservations, conversion and generated commission for the authenticated seller.
// @Tags Referrals
// @Produce json
// @Param days query int false "Period in days" default(30)
// @Param seller_id query int false "Seller ID (admin only)"
// @Success 200 {object} models.ReferralMetrics
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /vendedor/referrals [get]
func (h *ReferralHandler) Metrics(c *gin.Context) {
	sellerID, ok := referralSellerID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes acceso a las métricas de referidos"})
		return
	}
	days, _ := strconv.Atoi(c.Query("days"))
	metrics, err := h.referral.Metrics(c.Request.Context(), sellerID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron calcular las métricas"})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

// @Summary Seller referral reservations
// @Description Reservations attributed to the authenticated seller.
// @Tags Referrals
// @Produce json
// @Param limit query int false "Maximum rows" default(50)
// @Param seller_id query int false "Seller ID (admin only)"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /vendedor/referrals/reservations [get]
func (h *ReferralHandler) Reservations(c *gin.Context) {
	sellerID, ok := referralSellerID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes acceso a los referidos"})
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	contracts, err := h.referral.Reservations(c.Request.Context(), sellerID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los referidos"})
		return
	}
	responses := make([]models.ReferralReservation, 0, len(contracts))
	for i := range contracts {
		responses = append(responses, contracts[i].ToReferralReservation())
	}
	c.JSON(http.StatusOK, gin.H{"reservations": responses})
}

func referralSellerID(c *gin.Context) (uint, bool) {
	role := strings.ToLower(middleware.GetUserRole(c))
	if role == "admin" {
		if raw := strings.TrimSpace(c.Query("seller_id")); raw != "" {
			id, err := strconv.ParseUint(raw, 10, 32)
			if err != nil {
				return 0, false
			}
			return uint(id), true
		}
	}
	id := middleware.GetUserID(c)
	return id, id > 0
}
