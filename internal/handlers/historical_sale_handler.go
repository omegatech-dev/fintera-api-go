package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
)

// HistoricalSaleHandler serves migrated sales. Historical records never create
// reservations or touch lot availability.
type HistoricalSaleHandler struct {
	historical *services.HistoricalSaleService
	storage    *storage.LocalStorage
}

// NewHistoricalSaleHandler creates the historical-sale handler.
func NewHistoricalSaleHandler(historical *services.HistoricalSaleService, store *storage.LocalStorage) *HistoricalSaleHandler {
	return &HistoricalSaleHandler{historical: historical, storage: store}
}

// Index lists historical sales. Sellers only see the records they created;
// admins see every record and can filter with seller_id.
// @Summary List historical sales
// @Description Historical sales visible to the caller. Sellers see their own; admins see all.
// @Tags HistoricalSales
// @Produce json
// @Param page query int false "Page" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param status query string false "Validation status: pending, validated, rejected, all"
// @Param seller_id query int false "Creator ID (admin only)"
// @Param search_term query string false "Client name, identity, phone or project"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /historical_sales [get]
func (h *HistoricalSaleHandler) Index(c *gin.Context) {
	query := &repository.HistoricalSaleQuery{ListQuery: ParseListQuery(c)}
	if !middleware.IsAdmin(c) {
		query.CreatorID = middleware.GetUserID(c)
	} else if raw := strings.TrimSpace(c.Query("seller_id")); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "seller_id inválido"})
			return
		}
		query.CreatorID = uint(id)
	}
	query.ValidationStatus = c.Query("status")
	if from := c.Query("sale_from"); from != "" {
		query.Filters["sale_from"] = from
	}
	if to := c.Query("sale_to"); to != "" {
		query.Filters["sale_to"] = to
	}

	sales, total, err := h.historical.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar las ventas históricas"})
		return
	}
	responses := make([]interface{}, 0, len(sales))
	for i := range sales {
		responses = append(responses, sales[i].ToResponse())
	}
	totalPages := int64(1)
	if query.PerPage > 0 {
		totalPages = (total + int64(query.PerPage) - 1) / int64(query.PerPage)
		if totalPages < 1 {
			totalPages = 1
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"historical_sales": responses,
		"pagination": gin.H{
			"page":        query.Page,
			"per_page":    query.PerPage,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// Show returns one historical sale. Sellers can only read their own records;
// admins can read any record.
// @Summary Get historical sale
// @Description Historical sale visible to the caller. Sellers see their own; admins see any.
// @Tags HistoricalSales
// @Produce json
// @Param id path int true "Historical sale ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /historical_sales/{id} [get]
func (h *HistoricalSaleHandler) Show(c *gin.Context) {
	id, ok := parseHistoricalSaleID(c)
	if !ok {
		return
	}
	sale, err := h.historical.Get(c.Request.Context(), id, middleware.GetUserID(c), middleware.IsAdmin(c))
	if err != nil {
		writeHistoricalSaleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"historical_sale": sale.ToResponse()})
}

// Create records one historical sale for the authenticated seller.
// @Summary Create historical sale
// @Description Record a migrated sale. It never creates a reservation or blocks a lot.
// @Tags HistoricalSales
// @Accept json
// @Produce json
// @Param request body services.HistoricalSaleInput true "Historical sale"
// @Success 201 {object} map[string]interface{}
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Security BearerAuth
// @Router /historical_sales [post]
func (h *HistoricalSaleHandler) Create(c *gin.Context) {
	var input services.HistoricalSaleInput
	if err := BindNestedOrFlat(c, "historical_sale", &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}
	actorID := middleware.GetUserID(c)
	sale, err := h.historical.Create(c.Request.Context(), actorID, input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeHistoricalSaleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"historical_sale": sale.ToResponse(), "message": "Venta histórica registrada"})
}

// Update edits a historical sale.
// @Summary Update historical sale
// @Description Update a historical sale. Sellers can only edit their own pending records.
// @Tags HistoricalSales
// @Accept json
// @Produce json
// @Param id path int true "Historical sale ID"
// @Param request body services.HistoricalSaleInput true "Historical sale"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /historical_sales/{id} [put]
func (h *HistoricalSaleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}
	var input services.HistoricalSaleInput
	if err := BindNestedOrFlat(c, "historical_sale", &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}
	sale, err := h.historical.Update(c.Request.Context(), uint(id), middleware.GetUserID(c), input, middleware.IsAdmin(c), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeHistoricalSaleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"historical_sale": sale.ToResponse(), "message": "Venta histórica actualizada"})
}

// Import uploads a CSV of historical sales.
// @Summary Import historical sales
// @Description Import a CSV with toggles and an optional column mapping. Invalid rows are reported and valid rows are imported.
// @Tags HistoricalSales
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "CSV file"
// @Param mark_historical formData bool false "Mark rows as historical (default true)"
// @Param skip_duplicates formData bool false "Skip rows whose identity and sale date already exist (default true)"
// @Param commission_with_sale_date formData bool false "Create the commission using the sale date (default true)"
// @Param mapping formData string false "JSON object mapping field names to column indexes/headers"
// @Success 200 {object} services.HistoricalImportResult
// @Success 207 {object} services.HistoricalImportResult "Import finished with row errors"
// @Failure 400 {object} map[string]string
// @Security BearerAuth
// @Router /historical_sales/import [post]
func (h *HistoricalSaleHandler) Import(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El archivo CSV es obligatorio (campo file)"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudo leer el archivo CSV"})
		return
	}
	defer func() { _ = file.Close() }()

	options := services.HistoricalImportOptions{
		MarkHistorical:         importBoolOptionDefault(c, "mark_historical", true),
		SkipDuplicates:         importBoolOptionDefault(c, "skip_duplicates", true),
		CommissionWithSaleDate: importBoolOptionDefault(c, "commission_with_sale_date", importBoolOptionDefault(c, "create_commission", true)),
		Mapping:                parseHistoricalMapping(c.PostForm("mapping")),
	}
	result, err := h.historical.ImportCSV(c.Request.Context(), middleware.GetUserID(c), file, options, c.ClientIP(), c.Request.UserAgent())
	if result == nil {
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudo procesar el archivo"})
		return
	}
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "errors": result.Errors, "result": result})
		return
	}
	status := http.StatusOK
	if len(result.Errors) > 0 {
		status = http.StatusMultiStatus
	}
	c.JSON(status, result)
}

// ImportTemplate downloads the canonical historical-sales CSV template.
// @Summary Download historical sales template
// @Description Download the CSV template accepted by the import endpoint.
// @Tags HistoricalSales
// @Produce text/csv
// @Success 200 {file} file
// @Security BearerAuth
// @Router /historical_sales/import/template [get]
func (h *HistoricalSaleHandler) ImportTemplate(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="historical-sales-template.csv"`)
	c.String(http.StatusOK, services.HistoricalSalesCSVTemplate())
}

// Validate marks a historical sale as validated. Admin only.
// @Summary Validate historical sale
// @Description Validate a historical sale without creating a reservation. Admin only.
// @Tags HistoricalSales
// @Produce json
// @Param id path int true "Historical sale ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /historical_sales/{id}/validate [post]
func (h *HistoricalSaleHandler) Validate(c *gin.Context) {
	id, ok := parseHistoricalSaleID(c)
	if !ok {
		return
	}
	sale, err := h.historical.Validate(c.Request.Context(), id, middleware.GetUserID(c), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeHistoricalSaleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"historical_sale": sale.ToResponse(), "message": "Venta histórica validada"})
}

// Reject marks a historical sale as rejected. Admin only.
// @Summary Reject historical sale
// @Description Reject a historical sale without creating a reservation. Admin only.
// @Tags HistoricalSales
// @Accept json
// @Produce json
// @Param id path int true "Historical sale ID"
// @Param request body map[string]string false "Optional reason"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /historical_sales/{id}/reject [post]
func (h *HistoricalSaleHandler) Reject(c *gin.Context) {
	id, ok := parseHistoricalSaleID(c)
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&input)
	sale, err := h.historical.Reject(c.Request.Context(), id, middleware.GetUserID(c), input.Reason, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeHistoricalSaleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"historical_sale": sale.ToResponse(), "message": "Venta histórica rechazada"})
}

func parseHistoricalSaleID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return 0, false
	}
	return uint(id), true
}

// isExternalEvidenceURL reports whether the stored evidence value is an
// external link (legacy data) rather than a local storage path.
func isExternalEvidenceURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// UploadEvidence stores the seller's evidence ("respaldo") file for a historical
// sale. PDF, JPEG and PNG up to 10 MB are accepted; the file lands under
// STORAGE_PATH/evidence/YYYY/MM and its relative path is saved on the record.
// @Summary Upload historical sale evidence
// @Description Upload the evidence (respaldo) file for a historical sale. PDF/JPEG/PNG up to 10 MB. Sellers may upload to their own records; admins to any.
// @Tags HistoricalSales
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Historical sale ID"
// @Param file formData file true "Evidence file (PDF, JPEG or PNG)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /historical_sales/{id}/evidence [post]
func (h *HistoricalSaleHandler) UploadEvidence(c *gin.Context) {
	id, ok := parseHistoricalSaleID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, storage.MaxFileSize())

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se ha subido ningún archivo o el archivo es demasiado grande (campo file, máx. 10 MB)"})
		return
	}
	defer func() { _ = file.Close() }()

	if !storage.IsValidContentType(header.Header.Get("Content-Type")) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Solo se aceptan archivos PDF, JPEG o PNG"})
		return
	}
	if header.Size > storage.MaxFileSize() {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "El archivo supera el límite de 10 MB"})
		return
	}

	path, err := h.storage.Upload(file, header, "evidence")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar el archivo de evidencia"})
		return
	}

	sale, err := h.historical.SetEvidence(c.Request.Context(), id, middleware.GetUserID(c), middleware.IsAdmin(c), path, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		// The file is already on disk; the orphan is acceptable and the retry
		// simply stores another copy.
		writeHistoricalSaleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"historical_sale": sale.ToResponse(), "message": "Evidencia subida exitosamente"})
}

// DownloadEvidence streams the evidence file of a historical sale. The default
// response renders inline (the admin can view the image in the browser);
// `download=1` forces an attachment download. Legacy records whose evidence is
// an external URL get that URL back instead of a file stream.
// @Summary View or download historical sale evidence
// @Description Stream the evidence (respaldo) file of a historical sale. Inline by default; pass download=1 for an attachment. Admins access any record, sellers only their own.
// @Tags HistoricalSales
// @Produce application/octet-stream
// @Param id path int true "Historical sale ID"
// @Param download query bool false "Set to 1 to force a file download"
// @Success 200 {file} file "evidence file"
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /historical_sales/{id}/evidence [get]
func (h *HistoricalSaleHandler) DownloadEvidence(c *gin.Context) {
	id, ok := parseHistoricalSaleID(c)
	if !ok {
		return
	}
	sale, err := h.historical.Get(c.Request.Context(), id, middleware.GetUserID(c), middleware.IsAdmin(c))
	if err != nil {
		writeHistoricalSaleError(c, err)
		return
	}
	if sale.EvidenceFile == nil || strings.TrimSpace(*sale.EvidenceFile) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Esta venta no tiene evidencia subida"})
		return
	}

	evidence := strings.TrimSpace(*sale.EvidenceFile)
	if isExternalEvidenceURL(evidence) {
		c.JSON(http.StatusOK, gin.H{"evidence_url": evidence})
		return
	}

	fullPath, err := h.storage.SafeFullPath(evidence)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Evidencia no encontrada"})
		return
	}
	if !h.storage.Exists(evidence) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Evidencia no encontrada"})
		return
	}

	disposition := "inline"
	if c.Query("download") == "1" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", disposition+`; filename="`+filepath.Base(fullPath)+`"`)
	c.File(fullPath)
}

func writeHistoricalSaleError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, services.ErrHistoricalSaleNotFound):
		status = http.StatusNotFound
	case errors.Is(err, services.ErrHistoricalSaleForbidden):
		status = http.StatusForbidden
	case errors.Is(err, services.ErrHistoricalSaleDuplicate):
		status = http.StatusConflict
	case errors.Is(err, services.ErrHistoricalClientStaffIdentity):
		status = http.StatusConflict
	case errors.Is(err, services.ErrHistoricalSaleInvalid),
		errors.Is(err, services.ErrInvalidCommissionRate):
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func importBoolOptionDefault(c *gin.Context, name string, fallback bool) bool {
	value, exists := c.GetPostForm(name)
	if !exists || value == "" {
		value = c.PostForm("options[" + name + "]")
		if value == "" {
			return fallback
		}
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return fallback
	}
}

func parseHistoricalMapping(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	mapping := make(map[string]string, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case string:
			mapping[key] = typed
		case float64:
			mapping[key] = strconv.Itoa(int(typed))
		}
	}
	return mapping
}
