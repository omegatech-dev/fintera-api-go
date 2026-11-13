package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
)

// PaymentInstructionHandler manages the bank instructions of a project.
type PaymentInstructionHandler struct {
	service *services.ProjectPaymentInstructionService
}

// NewPaymentInstructionHandler creates the handler.
func NewPaymentInstructionHandler(service *services.ProjectPaymentInstructionService) *PaymentInstructionHandler {
	return &PaymentInstructionHandler{service: service}
}

type PaymentInstructionRequest struct {
	BankName        string  `json:"bank_name" example:"BAC Credomatic"`
	BeneficiaryName string  `json:"beneficiary_name" example:"Fintera Inversiones Inmobiliarias S.A."`
	AccountNumber   string  `json:"account_number" example:"741-209-4321"`
	AccountType     string  `json:"account_type" example:"checking" enums:"savings,checking"`
	Notes           *string `json:"notes" example:"Indica el número de lote en la referencia"`
	Position        *int    `json:"position" example:"0"`
	Currency        string  `json:"currency" example:"HNL"`
	IsDefault       bool    `json:"is_default" example:"true"`
}

// @Summary List project payment instructions
// @Description Admin list of the bank accounts configured for a project.
// @Tags Project Payment Instructions
// @Produce json
// @Param project_id path int true "Project ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/payment-instructions [get]
func (h *PaymentInstructionHandler) Index(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	instructions, err := h.service.List(c.Request.Context(), uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	responses := make([]models.PaymentInstructionResponse, 0, len(instructions))
	for i := range instructions {
		responses = append(responses, instructions[i].ToResponse())
	}
	c.JSON(http.StatusOK, gin.H{"payment_instructions": responses})
}

// @Summary Create project payment instruction
// @Description Add a bank account where customers can pay a project reservation.
// @Tags Project Payment Instructions
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Param request body PaymentInstructionRequest true "Bank account data"
// @Success 201 {object} map[string]interface{}
// @Failure 422 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/payment-instructions [post]
func (h *PaymentInstructionHandler) Create(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	var req PaymentInstructionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cuerpo de la solicitud inválido"})
		return
	}
	instruction := &models.ProjectPaymentInstruction{
		BankName:        req.BankName,
		BeneficiaryName: req.BeneficiaryName,
		AccountNumber:   req.AccountNumber,
		AccountType:     req.AccountType,
		Notes:           req.Notes,
		Currency:        req.Currency,
		IsDefault:       req.IsDefault,
	}
	if req.Position != nil {
		instruction.Position = *req.Position
	}
	if err := h.service.Create(c.Request.Context(), uint(projectID), middleware.GetUserID(c), instruction); err != nil {
		mapInstructionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"payment_instruction": instruction.ToResponse()})
}

// @Summary Update project payment instruction
// @Description Edit an existing bank account of a project.
// @Tags Project Payment Instructions
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Param instruction_id path int true "Payment instruction ID"
// @Param request body PaymentInstructionRequest true "Bank account data"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/payment-instructions/{instruction_id} [patch]
func (h *PaymentInstructionHandler) Update(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	id, _ := strconv.ParseUint(c.Param("instruction_id"), 10, 32)
	var req PaymentInstructionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cuerpo de la solicitud inválido"})
		return
	}
	instruction := &models.ProjectPaymentInstruction{
		BankName:        req.BankName,
		BeneficiaryName: req.BeneficiaryName,
		AccountNumber:   req.AccountNumber,
		AccountType:     req.AccountType,
		Notes:           req.Notes,
		Position:        -1,
		Currency:        req.Currency,
		IsDefault:       req.IsDefault,
	}
	if req.Position != nil {
		instruction.Position = *req.Position
	}
	updated, err := h.service.Update(c.Request.Context(), uint(projectID), uint(id), middleware.GetUserID(c), instruction)
	if err != nil {
		mapInstructionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"payment_instruction": updated.ToResponse()})
}

// @Summary Delete project payment instruction
// @Description Remove a bank account from a project.
// @Tags Project Payment Instructions
// @Produce json
// @Param project_id path int true "Project ID"
// @Param instruction_id path int true "Payment instruction ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/payment-instructions/{instruction_id} [delete]
func (h *PaymentInstructionHandler) Delete(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	id, _ := strconv.ParseUint(c.Param("instruction_id"), 10, 32)
	if err := h.service.Delete(c.Request.Context(), uint(projectID), uint(id), middleware.GetUserID(c)); err != nil {
		mapInstructionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Instrucción eliminada"})
}

func mapInstructionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Instrucción no encontrada"})
	case errors.Is(err, services.ErrInstructionInvalidType),
		errors.Is(err, services.ErrInstructionInvalidData),
		errors.Is(err, services.ErrInstructionInvalidLength),
		errors.Is(err, services.ErrInstructionInvalidCurrency):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInstructionDefaultDelete):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
