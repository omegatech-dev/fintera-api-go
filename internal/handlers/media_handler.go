package handlers

import (
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/services"
)

// MediaHandler manages project drawings, project photos and lot photos.
type MediaHandler struct {
	media *services.MediaService
}

// NewMediaHandler creates the admin media handler.
func NewMediaHandler(media *services.MediaService) *MediaHandler {
	return &MediaHandler{media: media}
}

// @Summary List project drawings
// @Description Admin list of a project's drawings.
// @Tags Project Media
// @Produce json
// @Param project_id path int true "Project ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/drawings [get]
func (h *MediaHandler) ListDrawings(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	drawings, err := h.media.ListDrawings(c.Request.Context(), uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"drawings": drawings})
}

// @Summary List project photos
// @Description Admin list of the project gallery.
// @Tags Project Media
// @Produce json
// @Param project_id path int true "Project ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/photos [get]
func (h *MediaHandler) ListProjectPhotos(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	photos, err := h.media.ListProjectPhotos(c.Request.Context(), uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"photos": photos})
}

// @Summary List lot photos
// @Description Admin list of a lot gallery.
// @Tags Project Media
// @Produce json
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/photos [get]
func (h *MediaHandler) ListLotPhotos(c *gin.Context) {
	lotID, _ := strconv.ParseUint(c.Param("lot_id"), 10, 32)
	photos, err := h.media.ListLotPhotos(c.Request.Context(), uint(lotID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"photos": photos})
}

// @Summary Upload project drawings
// @Description Upload one or more plans (dwg, dxf, pdf, svg; max 50 MB each).
// @Tags Project Media
// @Accept multipart/form-data
// @Produce json
// @Param project_id path int true "Project ID"
// @Param files formData file true "Drawing files"
// @Success 201 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/drawings [post]
func (h *MediaHandler) UploadDrawings(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 500<<20)
	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudieron leer los archivos"})
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudieron leer los archivos"})
		return
	}

	keys := sortedFileKeys(form.File, "files")
	if len(keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se enviaron planos"})
		return
	}

	actorID := middleware.GetUserID(c)
	drawings := make([]interface{}, 0)
	failures := make([]gin.H, 0)

	for _, key := range keys {
		index, _ := fileIndex(key, "files")
		for _, header := range form.File[key] {
			file, err := header.Open()
			if err != nil {
				failures = append(failures, gin.H{"file": header.Filename, "error": "No se pudo abrir el archivo"})
				continue
			}
			drawing, err := h.media.UploadDrawing(
				c.Request.Context(), uint(projectID), actorID, file, header,
				formValue(form.Value, "names", index),
				formValue(form.Value, "versions", index),
				formValue(form.Value, "notes", index),
			)
			_ = file.Close()
			if err != nil {
				failures = append(failures, gin.H{"file": header.Filename, "error": err.Error()})
				continue
			}
			drawings = append(drawings, drawing.ToResponse())
		}
	}

	if len(drawings) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "No se pudo subir ningún plano", "details": failures})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"drawings": drawings, "errors": failures})
}

// @Summary Update project drawing
// @Description Update name, version and note of a drawing.
// @Tags Project Media
// @Accept json
// @Produce json
// @Param drawing_id path int true "Drawing ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /drawings/{drawing_id} [patch]
func (h *MediaHandler) UpdateDrawing(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("drawing_id"), 10, 32)
	var req struct {
		Name    *string `json:"name"`
		Version *string `json:"version"`
		Note    *string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cuerpo de la solicitud inválido"})
		return
	}
	drawing, err := h.media.UpdateDrawing(c.Request.Context(), uint(id), middleware.GetUserID(c), req.Name, req.Version, req.Note)
	if err != nil {
		mapMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"drawing": drawing.ToResponse()})
}

// @Summary Delete project drawing
// @Description Delete a drawing and its stored file.
// @Tags Project Media
// @Produce json
// @Param drawing_id path int true "Drawing ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /drawings/{drawing_id} [delete]
func (h *MediaHandler) DeleteDrawing(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("drawing_id"), 10, 32)
	if err := h.media.DeleteDrawing(c.Request.Context(), uint(id), middleware.GetUserID(c)); err != nil {
		mapMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plano eliminado"})
}

// @Summary Upload project photos
// @Description Upload project gallery photos (jpeg, png, webp; max 10 MB each).
// @Tags Project Media
// @Accept multipart/form-data
// @Produce json
// @Param project_id path int true "Project ID"
// @Param photos formData file true "Photo files"
// @Param thumbs formData file false "Optional thumbnails"
// @Success 201 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/photos [post]
func (h *MediaHandler) UploadProjectPhotos(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 200<<20)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudieron leer las fotos"})
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudieron leer las fotos"})
		return
	}

	keys := sortedFileKeys(form.File, "photos")
	if len(keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se enviaron fotos"})
		return
	}

	actorID := middleware.GetUserID(c)
	photos := make([]interface{}, 0)
	failures := make([]gin.H, 0)

	for _, key := range keys {
		index, _ := fileIndex(key, "photos")
		thumbFile, thumbHeader := firstFile(form.File, "thumbs", index)
		for _, header := range form.File[key] {
			file, err := header.Open()
			if err != nil {
				failures = append(failures, gin.H{"file": header.Filename, "error": "No se pudo abrir la foto"})
				continue
			}
			photo, err := h.media.UploadProjectPhoto(
				c.Request.Context(), uint(projectID), actorID, file, header,
				thumbFile, thumbHeader, formValue(form.Value, "alts", index),
			)
			_ = file.Close()
			if err != nil {
				failures = append(failures, gin.H{"file": header.Filename, "error": err.Error()})
				continue
			}
			photos = append(photos, photo.ToResponse())
		}
		if thumbFile != nil {
			_ = thumbFile.Close()
		}
	}

	if len(photos) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "No se pudo subir ninguna foto", "details": failures})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"photos": photos, "errors": failures})
}

// @Summary Upload lot photos
// @Description Upload lot gallery photos (jpeg, png, webp; max 10 MB each).
// @Tags Project Media
// @Accept multipart/form-data
// @Produce json
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param photos formData file true "Photo files"
// @Param thumbs formData file false "Optional thumbnails"
// @Success 201 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/photos [post]
func (h *MediaHandler) UploadLotPhotos(c *gin.Context) {
	lotID, _ := strconv.ParseUint(c.Param("lot_id"), 10, 32)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 200<<20)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudieron leer las fotos"})
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudieron leer las fotos"})
		return
	}

	keys := sortedFileKeys(form.File, "photos")
	if len(keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se enviaron fotos"})
		return
	}

	actorID := middleware.GetUserID(c)
	photos := make([]interface{}, 0)
	failures := make([]gin.H, 0)

	for _, key := range keys {
		index, _ := fileIndex(key, "photos")
		thumbFile, thumbHeader := firstFile(form.File, "thumbs", index)
		for _, header := range form.File[key] {
			file, err := header.Open()
			if err != nil {
				failures = append(failures, gin.H{"file": header.Filename, "error": "No se pudo abrir la foto"})
				continue
			}
			photo, err := h.media.UploadLotPhoto(
				c.Request.Context(), uint(lotID), actorID, file, header,
				thumbFile, thumbHeader, formValue(form.Value, "alts", index),
			)
			_ = file.Close()
			if err != nil {
				failures = append(failures, gin.H{"file": header.Filename, "error": err.Error()})
				continue
			}
			photos = append(photos, photo.ToResponse())
		}
		if thumbFile != nil {
			_ = thumbFile.Close()
		}
	}

	if len(photos) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "No se pudo subir ninguna foto", "details": failures})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"photos": photos, "errors": failures})
}

type photoUpdateRequest struct {
	Position *int    `json:"position"`
	Alt      *string `json:"alt"`
}

// @Summary Update project photo
// @Description Reorder or edit the alt text of a project photo.
// @Tags Project Media
// @Accept json
// @Produce json
// @Param photo_id path int true "Photo ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /project-photos/{photo_id} [patch]
func (h *MediaHandler) UpdateProjectPhoto(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("photo_id"), 10, 32)
	var req photoUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cuerpo de la solicitud inválido"})
		return
	}
	photo, err := h.media.UpdateProjectPhoto(c.Request.Context(), 0, uint(id), middleware.GetUserID(c), req.Position, req.Alt)
	if err != nil {
		mapMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"photo": photo.ToResponse()})
}

// @Summary Delete project photo
// @Description Delete a project photo and its files.
// @Tags Project Media
// @Produce json
// @Param photo_id path int true "Photo ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /project-photos/{photo_id} [delete]
func (h *MediaHandler) DeleteProjectPhoto(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("photo_id"), 10, 32)
	if err := h.media.DeleteProjectPhoto(c.Request.Context(), 0, uint(id), middleware.GetUserID(c)); err != nil {
		mapMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Foto eliminada"})
}

// @Summary Update lot photo
// @Description Reorder or edit the alt text of a lot photo.
// @Tags Project Media
// @Accept json
// @Produce json
// @Param photo_id path int true "Photo ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /photos/{photo_id} [patch]
func (h *MediaHandler) UpdateLotPhoto(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("photo_id"), 10, 32)
	var req photoUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cuerpo de la solicitud inválido"})
		return
	}
	photo, err := h.media.UpdateLotPhoto(c.Request.Context(), 0, uint(id), middleware.GetUserID(c), req.Position, req.Alt)
	if err != nil {
		mapMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"photo": photo.ToResponse()})
}

// @Summary Delete lot photo
// @Description Delete a lot photo and its files.
// @Tags Project Media
// @Produce json
// @Param photo_id path int true "Photo ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /photos/{photo_id} [delete]
func (h *MediaHandler) DeleteLotPhoto(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("photo_id"), 10, 32)
	if err := h.media.DeleteLotPhoto(c.Request.Context(), 0, uint(id), middleware.GetUserID(c)); err != nil {
		mapMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Foto eliminada"})
}

func sortedFileKeys(files map[string][]*multipart.FileHeader, prefix string) []string {
	keys := make([]string, 0)
	for key := range files {
		if key == prefix || strings.HasPrefix(key, prefix+"[") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func fileIndex(key, prefix string) (int, bool) {
	if key == prefix {
		return 0, true
	}
	start := strings.Index(key, "[")
	end := strings.Index(key, "]")
	if start < 0 || end <= start {
		return 0, false
	}
	value, err := strconv.Atoi(key[start+1 : end])
	if err != nil {
		return 0, false
	}
	return value, true
}

func formValue(values map[string][]string, prefix string, index int) string {
	key := prefix + "[" + strconv.Itoa(index) + "]"
	if items := values[key]; len(items) > 0 {
		return items[0]
	}
	if items := values[prefix]; len(items) > 0 {
		return items[0]
	}
	return ""
}

func firstFile(files map[string][]*multipart.FileHeader, prefix string, index int) (multipart.File, *multipart.FileHeader) {
	keys := []string{prefix + "[" + strconv.Itoa(index) + "]", prefix}
	for _, key := range keys {
		headers := files[key]
		if len(headers) == 0 {
			continue
		}
		file, err := headers[0].Open()
		if err == nil {
			return file, headers[0]
		}
	}
	return nil, nil
}
