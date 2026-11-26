package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
)

// IdentityDocumentHandler serves the DNI and the RTN of a person.
type IdentityDocumentHandler struct {
	service *services.IdentityDocumentService
}

// NewIdentityDocumentHandler creates the identity document handler.
func NewIdentityDocumentHandler(service *services.IdentityDocumentService) *IdentityDocumentHandler {
	return &IdentityDocumentHandler{service: service}
}

// viewer identifies the caller of an identity document operation.
func (h *IdentityDocumentHandler) viewer(c *gin.Context) services.IdentityDocumentViewer {
	return services.IdentityDocumentViewer{
		UserID:   middleware.GetUserID(c),
		IsAdmin:  middleware.IsAdmin(c),
		IsSeller: middleware.IsSeller(c),
	}
}

// ListByUser lists the identity documents of a person.
//
// @Summary List the identity documents of a person
// @Description Return the DNI and the RTN slots of a person. A slot without a document on file is reported as empty.
// @Tags IdentityDocuments
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} models.IdentityDocumentUserResponse
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id}/identity_documents [get]
func (h *IdentityDocumentHandler) ListByUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de usuario inválido"})
		return
	}

	ctx := c.Request.Context()
	allowed, err := h.service.CanReadPerson(ctx, h.viewer(c), uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para ver los documentos de este cliente"})
		return
	}

	result, err := h.service.ListByUser(ctx, uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"identity_documents": result})
}

// UploadForUser stores the DNI or the RTN of a person.
//
// @Summary Upload an identity document for a person
// @Description Store the DNI or the RTN of a person. The upload replaces the previous document of that kind.
// @Tags IdentityDocuments
// @Accept multipart/form-data
// @Produce json
// @Param user_id path int true "User ID"
// @Param kind formData string true "Document kind" Enums(dni, rtn)
// @Param file formData file true "Document file"
// @Success 200 {object} models.IdentityDocumentFile
// @Failure 422 {object} map[string]interface{}
// @Security BearerAuth
// @Router /users/{user_id}/identity_documents [post]
func (h *IdentityDocumentHandler) UploadForUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de usuario inválido"})
		return
	}

	kind := strings.ToLower(strings.TrimSpace(c.PostForm("kind")))
	if kind != models.IdentityDocumentKindDNI && kind != models.IdentityDocumentKindRTN {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "El tipo de documento debe ser dni o rtn",
			"code":  "INVALID_DOCUMENT_KIND",
		})
		return
	}

	_, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Archivo requerido"})
		return
	}

	doc, err := h.service.Upsert(c.Request.Context(), uint(userID), kind, header, h.viewer(c).UserID)
	if err != nil {
		writeIdentityDocumentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"identity_document": doc.ToResponse(),
		"message":           "Documento de identificación actualizado",
	})
}

// ListByContract lists the identity documents of the applicant of a contract.
//
// @Summary List the identity documents of a contract
// @Description Return the DNI and the RTN slots of the applicant of a contract, plus the files that were attached to it before the identity documents existed.
// @Tags IdentityDocuments
// @Accept json
// @Produce json
// @Param guid path string true "Contract GUID"
// @Success 200 {object} models.IdentityDocumentContractResponse
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /contracts/{guid}/identity_documents [get]
func (h *IdentityDocumentHandler) ListByContract(c *gin.Context) {
	contractID, ok := h.resolveContract(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	allowed, err := h.service.CanReadContract(ctx, h.viewer(c), contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para ver los documentos de este contrato"})
		return
	}

	result, err := h.service.ListByContract(ctx, contractID)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"identity_documents": result})
}

// UploadForContract stores the DNI or the RTN of the applicant of a
// contract.
//
// @Summary Upload an identity document for a contract
// @Description Store the DNI or the RTN of the applicant of a contract. The upload replaces the previous document of that kind.
// @Tags IdentityDocuments
// @Accept multipart/form-data
// @Produce json
// @Param guid path string true "Contract GUID"
// @Param kind formData string true "Document kind" Enums(dni, rtn)
// @Param file formData file true "Document file"
// @Success 200 {object} models.IdentityDocumentFile
// @Failure 422 {object} map[string]interface{}
// @Security BearerAuth
// @Router /contracts/{guid}/identity_documents [post]
func (h *IdentityDocumentHandler) UploadForContract(c *gin.Context) {
	contractID, ok := h.resolveContract(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	allowed, err := h.service.CanReadContract(ctx, h.viewer(c), contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para subir documentos a este contrato"})
		return
	}

	kind := strings.ToLower(strings.TrimSpace(c.PostForm("kind")))
	if kind != models.IdentityDocumentKindDNI && kind != models.IdentityDocumentKindRTN {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "El tipo de documento debe ser dni o rtn",
			"code":  "INVALID_DOCUMENT_KIND",
		})
		return
	}

	_, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Archivo requerido"})
		return
	}

	doc, err := h.service.UploadForContract(ctx, contractID, kind, header, h.viewer(c).UserID)
	if err != nil {
		writeIdentityDocumentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"identity_document": doc.ToResponse(),
		"message":           "Documento de identificación actualizado",
	})
}

// resolveContract reads the contract of the route and answers the request
// when it does not exist.
func (h *IdentityDocumentHandler) resolveContract(c *gin.Context) (uint, bool) {
	guid := strings.TrimSpace(c.Param("guid"))
	if guid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contrato inválido"})
		return 0, false
	}
	contract, err := h.service.FindContractByGUID(c.Request.Context(), guid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return 0, false
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return 0, false
	}
	return contract.ID, true
}

// Content serves the stored original of an identity document.
//
// @Summary Download an identity document
// @Description Serve the stored original of an identity document.
// @Tags IdentityDocuments
// @Accept json
// @Produce json
// @Param id path int true "Identity document ID"
// @Success 200 {file} file
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /identity_documents/{id}/content [get]
func (h *IdentityDocumentHandler) Content(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de documento inválido"})
		return
	}

	content, err := h.service.Content(c.Request.Context(), h.viewer(c), uint(id))
	if err != nil {
		writeIdentityDocumentError(c, err)
		return
	}
	defer func() { _ = content.File.Close() }()
	streamIdentityDocument(c, content)
}

// Preview serves the inline preview of an identity document.
//
// @Summary Preview an identity document
// @Description Serve the inline WebP preview of an identity document. Documents the server cannot rasterize are served inline as the original.
// @Tags IdentityDocuments
// @Accept json
// @Produce json
// @Param id path int true "Identity document ID"
// @Success 200 {file} file
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /identity_documents/{id}/preview [get]
func (h *IdentityDocumentHandler) Preview(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de documento inválido"})
		return
	}

	content, err := h.service.Preview(c.Request.Context(), h.viewer(c), uint(id))
	if err != nil {
		writeIdentityDocumentError(c, err)
		return
	}
	defer func() { _ = content.File.Close() }()
	streamIdentityDocument(c, content)
}

// LegacyContent serves a file that was attached to a contract before the
// identity documents existed.
//
// @Summary Download a legacy contract attachment
// @Description Serve a file that was attached to a contract before the identity documents existed. It is read-only.
// @Tags IdentityDocuments
// @Accept json
// @Produce json
// @Param guid path string true "Contract GUID"
// @Param document_id path int true "Legacy document ID"
// @Success 200 {file} file
// @Failure 403 {object} map[string]string
// @Security BearerAuth
// @Router /contracts/{guid}/legacy_documents/{document_id}/content [get]
func (h *IdentityDocumentHandler) LegacyContent(c *gin.Context) {
	contractID, ok := h.resolveContract(c)
	if !ok {
		return
	}
	documentID, err := strconv.ParseUint(c.Param("document_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de documento inválido"})
		return
	}

	content, err := h.service.LegacyContent(c.Request.Context(), h.viewer(c), contractID, uint(documentID))
	if err != nil {
		writeIdentityDocumentError(c, err)
		return
	}
	defer func() { _ = content.File.Close() }()
	streamIdentityDocument(c, content)
}

// streamIdentityDocument writes the file with its MIME type and disposition.
func streamIdentityDocument(c *gin.Context, content *services.IdentityDocumentFileContent) {
	c.Header("Content-Type", content.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename=%q", content.Disposition, sanitizeHeaderFilename(content.Filename)))
	if content.SizeBytes > 0 {
		c.Header("Content-Length", strconv.FormatInt(content.SizeBytes, 10))
	}
	c.Header("X-Content-Type-Options", "nosniff")
	// The documents are private: no shared cache may keep them.
	c.Header("Cache-Control", "private, no-store")
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, content.File); err != nil {
		return
	}
}

// sanitizeHeaderFilename strips the characters that would break the header.
func sanitizeHeaderFilename(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == '"' || r == '\\' {
			return -1
		}
		return r
	}, name)
	if strings.TrimSpace(cleaned) == "" {
		return "documento"
	}
	return cleaned
}

// writeIdentityDocumentError maps a service error to its response.
func writeIdentityDocumentError(c *gin.Context, err error) {
	var typeErr *services.ErrIdentityDocumentType
	if errors.As(err, &typeErr) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": typeErr.Error(),
			"code":  "INVALID_DOCUMENT_TYPE",
			"file":  typeErr.Filename,
		})
		return
	}

	var sizeErr *services.ErrIdentityDocumentSize
	if errors.As(err, &sizeErr) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": sizeErr.Error(),
			"code":  "DOCUMENT_TOO_LARGE",
			"file":  sizeErr.Filename,
		})
		return
	}

	var requiredErr services.ErrIdentityDocumentsRequired
	if errors.As(err, &requiredErr) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   requiredErr.Error(),
			"code":    "IDENTITY_DOCUMENTS_REQUIRED",
			"missing": requiredErr.Missing,
		})
		return
	}

	if errors.Is(err, services.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Documento no encontrado"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
