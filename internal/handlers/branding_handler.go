package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
)

type BrandingHandler struct {
	brandingService *services.BrandingService
}

func NewBrandingHandler(brandingService *services.BrandingService) *BrandingHandler {
	return &BrandingHandler{brandingService: brandingService}
}

// @Summary Get Branding
// @Description Get the global application branding (public)
// @Tags Branding
// @Produce json
// @Success 200 {object} models.BrandingResponse
// @Router /branding [get]
func (h *BrandingHandler) Show(c *gin.Context) {
	branding, err := h.brandingService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener la configuración de marca"})
		return
	}

	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, gin.H{"branding": branding.ToResponse()})
}

// @Summary Update Branding
// @Description Update the global application branding (Admin)
// @Tags Branding
// @Accept json
// @Produce json
// @Param request body brandingUpdateRequest true "Branding Data"
// @Success 200 {object} models.BrandingResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /branding [put]
func (h *BrandingHandler) Update(c *gin.Context) {
	var payload brandingUpdateRequest
	if err := BindNestedOrFlat(c, "branding", &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato JSON inválido: " + err.Error()})
		return
	}

	branding, err := h.brandingService.UpdateName(c.Request.Context(), payload.AppName, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branding": branding.ToResponse(), "message": "Configuración de marca actualizada"})
}

type brandingUpdateRequest struct {
	AppName string `json:"app_name"`
}

// @Summary Upload Branding Logo
// @Description Upload or replace the light/dark application logo (Admin)
// @Tags Branding
// @Accept multipart/form-data
// @Produce json
// @Param logo formData file true "Logo file (PNG, JPEG or WebP, max 2MB)"
// @Param variant formData string false "Logo variant: light or dark" default(light)
// @Success 200 {object} models.BrandingResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /branding/logo [post]
func (h *BrandingHandler) UploadLogo(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, services.MaxBrandingLogoBytes)

	file, header, err := c.Request.FormFile("logo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se ha subido ningún logo o el archivo es demasiado grande"})
		return
	}
	defer func() { _ = file.Close() }()

	variant := c.DefaultPostForm("variant", models.BrandingLogoVariantLight)

	branding, err := h.brandingService.UpdateLogo(c.Request.Context(), variant, file, header, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branding": branding.ToResponse(), "message": "Logo actualizado"})
}

// @Summary Delete Branding Logo
// @Description Remove the light/dark application logo (Admin)
// @Tags Branding
// @Produce json
// @Param variant query string false "Logo variant: light or dark" default(light)
// @Success 200 {object} models.BrandingResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /branding/logo [delete]
func (h *BrandingHandler) DeleteLogo(c *gin.Context) {
	variant := c.DefaultQuery("variant", models.BrandingLogoVariantLight)

	branding, err := h.brandingService.RemoveLogo(c.Request.Context(), variant, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branding": branding.ToResponse(), "message": "Logo eliminado"})
}
