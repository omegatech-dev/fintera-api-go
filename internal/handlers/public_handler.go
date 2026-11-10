package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/services"
)

// PublicHandler serves the unauthenticated customer portal catalog.
type PublicHandler struct {
	catalog *services.PublicCatalogService
	media   *services.MediaService
}

// NewPublicHandler creates the public catalog handler.
func NewPublicHandler(catalog *services.PublicCatalogService, media *services.MediaService) *PublicHandler {
	return &PublicHandler{catalog: catalog, media: media}
}

// @Summary List public projects
// @Description Published, active projects with public stats (no auth).
// @Tags Public Catalog
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /public/projects [get]
func (h *PublicHandler) ListProjects(c *gin.Context) {
	projects, err := h.catalog.ListProjects(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los proyectos"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// @Summary Get public project
// @Description Published project by GUID (no auth).
// @Tags Public Catalog
// @Produce json
// @Param guid path string true "Project GUID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /public/projects/{guid} [get]
func (h *PublicHandler) ShowProject(c *gin.Context) {
	project, err := h.catalog.GetProject(c.Request.Context(), c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proyecto no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"project": project})
}

// @Summary List public lots
// @Description Public lots of a published project (no auth).
// @Tags Public Catalog
// @Produce json
// @Param guid path string true "Project GUID"
// @Param available query bool false "Only available lots"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /public/projects/{guid}/lots [get]
func (h *PublicHandler) ListLots(c *gin.Context) {
	onlyAvailable := c.Query("available") == "true"
	lots, err := h.catalog.ListLots(c.Request.Context(), c.Param("guid"), onlyAvailable)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proyecto no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"lots": lots})
}

// @Summary Get public lot
// @Description Public lot by project/lot GUID (no auth).
// @Tags Public Catalog
// @Produce json
// @Param guid path string true "Project GUID"
// @Param lot_guid path string true "Lot GUID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /public/projects/{guid}/lots/{lot_guid} [get]
func (h *PublicHandler) ShowLot(c *gin.Context) {
	lot, err := h.catalog.GetLot(c.Request.Context(), c.Param("guid"), c.Param("lot_guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lote no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"lot": lot})
}

// @Summary Public payment instructions
// @Description Bank instructions of a published project by GUID (no auth).
// @Tags Public Catalog
// @Produce json
// @Param guid path string true "Project GUID"
// @Success 200 {object} map[string]interface{}
// @Router /public/projects/{guid}/payment-instructions [get]
func (h *PublicHandler) PaymentInstructions(c *gin.Context) {
	instructions, err := h.catalog.ListPaymentInstructions(c.Request.Context(), c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proyecto no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payment_instructions": instructions})
}

// @Summary Public project drawings
// @Description Published drawings of a project by GUID (no auth).
// @Tags Public Catalog
// @Produce json
// @Param guid path string true "Project GUID"
// @Success 200 {object} map[string]interface{}
// @Router /public/projects/{guid}/drawings [get]
func (h *PublicHandler) ListDrawings(c *gin.Context) {
	drawings, err := h.media.ListPublicDrawings(c.Request.Context(), c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proyecto no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"drawings": drawings})
}

// @Summary Public project photos
// @Description Project gallery by GUID (no auth).
// @Tags Public Catalog
// @Produce json
// @Param guid path string true "Project GUID"
// @Success 200 {object} map[string]interface{}
// @Router /public/projects/{guid}/photos [get]
func (h *PublicHandler) ListProjectPhotos(c *gin.Context) {
	photos, err := h.media.ListPublicProjectPhotos(c.Request.Context(), c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proyecto no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"photos": photos})
}

// @Summary Public lot photos
// @Description Lot gallery by project/lot GUID (no auth).
// @Tags Public Catalog
// @Produce json
// @Param guid path string true "Project GUID"
// @Param lot_guid path string true "Lot GUID"
// @Success 200 {object} map[string]interface{}
// @Router /public/projects/{guid}/lots/{lot_guid}/photos [get]
func (h *PublicHandler) ListLotPhotos(c *gin.Context) {
	photos, err := h.media.ListPublicLotPhotos(c.Request.Context(), c.Param("guid"), c.Param("lot_guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lote no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"photos": photos})
}

// mapMediaError translates media service errors into HTTP responses.
func mapMediaError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Recurso no encontrado"})
	case errors.Is(err, services.ErrUnsupportedMedia):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Tipo de archivo no permitido"})
	case errors.Is(err, services.ErrMediaTooLarge):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "El archivo excede el tamaño permitido"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
