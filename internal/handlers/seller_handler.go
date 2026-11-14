package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/services"
)

// SellerHandler serves the seller picker data for admins and sellers.
type SellerHandler struct {
	sellers *services.SellerService
}

// NewSellerHandler creates the seller handler.
func NewSellerHandler(sellers *services.SellerService) *SellerHandler {
	return &SellerHandler{sellers: sellers}
}

// @Summary List active sellers
// @Description Active sellers with sales count and acceptance score. Available to admins and sellers.
// @Tags Sellers
// @Produce json
// @Param search query string false "Search by name, email, phone or identity"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /sellers [get]
func (h *SellerHandler) Index(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	sellers, err := h.sellers.List(c.Request.Context(), search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los vendedores"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sellers": sellers})
}
