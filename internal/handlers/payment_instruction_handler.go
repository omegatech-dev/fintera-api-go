package handlers

import (
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

type paymentInstructionRequest struct {
	BankName        string  `json:"bank_name"`
	BeneficiaryName string  `json:"beneficiary_name"`
	AccountNumber   string  `json:"account_number"`
	AccountType     string  `json:"account_type"`
	Notes           *string `json:"notes"`
	Position        *int    `json:"position"`
}

// Index lists a project's instructions for the admin view.
func (h *PaymentInstructionHandler) Index(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	instructions, err := h.service.List(c.Request.Context(), uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	responses := make([]interface{}, 0, len(instructions))
	for i := range instructions {
		responses = append(responses, instructions[i].ToResponse())
	}
	c.JSON(http.StatusOK, gin.H{"payment_instructions": responses})
}

// Create adds an instruction to a project.
func (h *PaymentInstructionHandler) Create(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	var req paymentInstructionRequest
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

// Update edits an instruction.
func (h *PaymentInstructionHandler) Update(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	id, _ := strconv.ParseUint(c.Param("instruction_id"), 10, 32)
	var req paymentInstructionRequest
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

// Delete removes an instruction.
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
	switch err {
	case services.ErrNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "Instrucción no encontrada"})
	case services.ErrInstructionInvalidType, services.ErrInstructionInvalidData:
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
