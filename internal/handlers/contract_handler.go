package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
)

type ContractHandler struct {
	contractService *services.ContractService
	storage         *storage.LocalStorage
}

func NewContractHandler(contractService *services.ContractService, storage *storage.LocalStorage) *ContractHandler {
	return &ContractHandler{contractService: contractService, storage: storage}
}

// @Summary Send Contract Reminder
// @Description Send a manual reservation/payment reminder email for a contract.
// @Tags Contracts
// @Produce json
// @Param guid path string true "Contract GUID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /contracts/{guid}/reminder [post]
func (h *ContractHandler) SendReminder(c *gin.Context) {
	guid := c.Param("guid")
	if err := h.contractService.SendReminder(c.Request.Context(), guid, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Recordatorio enviado"})
}

// @Summary List Contracts
// @Description Get a paginated list of contracts for the current user (or all for admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param search_term query string false "Search term"
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /contracts [get]
func (h *ContractHandler) Index(c *gin.Context) {
	query := &repository.ContractQuery{ListQuery: ParseListQuery(c)}
	query.Status = c.Query("status")
	if guid := c.Query("guid"); guid != "" {
		query.Filters["guid"] = guid
	}
	if startDate := c.Query("start_date"); startDate != "" {
		query.Filters["start_date"] = startDate
	}
	if endDate := c.Query("end_date"); endDate != "" {
		query.Filters["end_date"] = endDate
	}
	query.UserID = middleware.GetUserID(c)
	query.IsAdmin = middleware.IsAdmin(c)

	contracts, total, err := h.contractService.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []interface{}
	for _, contract := range contracts {
		responses = append(responses, contract.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{
		"contracts": responses,
		"pagination": gin.H{
			"page":        query.Page,
			"per_page":    query.PerPage,
			"total":       total,
			"total_pages": (total + int64(query.PerPage) - 1) / int64(query.PerPage),
		},
	})
}

// @Summary Get Contract Stats
// @Description Get contract statistics: status counts plus active outstanding and overdue amounts.
// @Tags Contracts
// @Accept json
// @Produce json
// @Success 200 {object} repository.ContractStats
// @Security BearerAuth
// @Router /contracts/stats [get]
func (h *ContractHandler) GetStats(c *gin.Context) {
	stats, err := h.contractService.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// @Summary Get Contract
// @Description Get a contract by ID
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} models.ContractResponse
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id} [get]
func (h *ContractHandler) Show(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	contract, err := h.contractService.FindByIDWithDetails(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse()})
}

// @Summary Get Contract by GUID
// @Description Owner/admin access to a reservation by its public GUID.
// @Tags Contracts
// @Produce json
// @Param guid path string true "Contract GUID"
// @Success 200 {object} models.ContractResponse
// @Failure 403,404 {object} map[string]string
// @Security BearerAuth
// @Router /contracts/{guid} [get]
func (h *ContractHandler) ShowByGUID(c *gin.Context) {
	contract, err := h.contractService.FindByGUIDWithDetails(c.Request.Context(), c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}
	role := middleware.GetUserRole(c)
	if role != "admin" && role != "seller" && contract.ApplicantUserID != middleware.GetUserID(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes acceso a esta reserva"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse()})
}

type PrepaymentRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

// @Summary Request Prepayment
// @Description Customer request to pay part or all of the outstanding balance
// ahead of schedule. The payment stays submitted until staff validates the
// uploaded receipt.
// @Tags Contracts
// @Accept json
// @Produce json
// @Param guid path string true "Contract GUID"
// @Param request body PrepaymentRequest true "Amount"
// @Success 201 {object} map[string]interface{}
// @Failure 403,404,422 {object} map[string]string
// @Security BearerAuth
// @Router /contracts/{guid}/prepayments [post]
func (h *ContractHandler) RequestPrepayment(c *gin.Context) {
	contract, err := h.contractService.FindByGUIDWithDetails(c.Request.Context(), c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}
	role := middleware.GetUserRole(c)
	if role != "admin" && role != "seller" && contract.ApplicantUserID != middleware.GetUserID(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes acceso a esta reserva"})
		return
	}

	var req PrepaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Monto requerido y debe ser mayor a 0"})
		return
	}

	payment, err := h.contractService.CreatePrepaymentRequest(c.Request.Context(), contract.GUID, req.Amount)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"payment": payment.ToResponse(),
		"message": "Solicitud de pago anticipado registrada",
	})
}

// @Summary Create Contract
// @Description Create a new contract with optional new user and documents
// @Tags Contracts
// @Accept multipart/form-data
// @Produce json
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param contract formData string true "Contract Data (JSON)"
// @Param user formData string false "User Data (JSON)"
// @Param documents formData file false "Documents"
// @Success 201 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts [post]
func (h *ContractHandler) Create(c *gin.Context) {
	lotID, _ := strconv.ParseUint(c.Param("lot_id"), 10, 32)
	h.create(c, uint(lotID), false)
}

// @Summary Create Guest Reservation
// @Description Create a reservation without a session (implicit sign-up).
// @Tags Public Catalog
// @Accept multipart/form-data
// @Produce json
// @Param guid path string true "Project GUID"
// @Param lot_guid path string true "Lot GUID"
// @Success 201 {object} map[string]interface{}
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Router /public/projects/{guid}/lots/{lot_guid}/contracts [post]
func (h *ContractHandler) CreateGuest(c *gin.Context) {
	lot, err := h.contractService.FindLotByProjectAndGUID(c.Request.Context(), c.Param("guid"), c.Param("lot_guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lote no encontrado"})
		return
	}
	h.create(c, lot.ID, true)
}

func (h *ContractHandler) create(c *gin.Context, lotID uint, guest bool) {
	creatorID := middleware.GetUserID(c)

	// 2. Parse Form Data
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing form data: " + err.Error()})
		return
	}

	// 3. Extract Contract Fields
	paymentTerm, _ := strconv.Atoi(c.Request.FormValue("contract[payment_term]"))
	financingType := strings.TrimSpace(strings.ToLower(c.Request.FormValue("contract[financing_type]")))
	reserveAmount, _ := strconv.ParseFloat(c.Request.FormValue("contract[reserve_amount]"), 64)
	downPayment, _ := strconv.ParseFloat(c.Request.FormValue("contract[down_payment]"), 64)
	maxPaymentDateStr := strings.TrimSpace(c.Request.FormValue("contract[max_payment_date]"))
	bankName := strings.TrimSpace(c.Request.FormValue("contract[bank_name]"))
	bankCreditStatusStr := strings.TrimSpace(c.Request.FormValue("contract[bank_credit_status]"))
	note := c.Request.FormValue("contract[note]")
	applicantUserIDStr := c.Request.FormValue("contract[applicant_user_id]")

	// Validate and parse max_payment_date. Cash reservations require it because
	// the buyer controls the transfer; bank reservations do not, because the
	// bank does not commit to a disbursement date at reservation time.
	maxPaymentDate, err := parseMaxPaymentDate(financingType, maxPaymentDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Bank details only apply to bank financing. Empty means "not provided".
	bankNamePtr, bankCreditStatusPtr, err := parseBankCreditDetails(financingType, bankName, bankCreditStatusStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 4. Handle User (Find or Create)
	var applicantID uint
	if applicantUserIDStr != "" {
		// Existing User
		id, _ := strconv.ParseUint(applicantUserIDStr, 10, 32)
		user, err := h.contractService.GetUserByID(c.Request.Context(), uint(id))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario no encontrado"})
			return
		}

		if !user.IsActive() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El usuario no está activo y no puede crear contratos"})
			return
		}
		applicantID = user.ID
	} else if guest && creatorID > 0 {
		applicantID = creatorID
	} else if guest {
		email := strings.TrimSpace(c.Request.FormValue("user[email]"))
		identity := strings.TrimSpace(c.Request.FormValue("user[identity]"))

		if email == "" || identity == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email e Identidad son requeridos para nuevos usuarios"})
			return
		}
		if _, err := mail.ParseAddress(email); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El correo electrónico no tiene un formato válido"})
			return
		}
		if len(identity) < 5 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La identidad debe tener al menos 5 caracteres"})
			return
		}

		// Guests with an existing account must sign in instead of creating a
		// duplicate.
		if existing, err := h.contractService.GetUserByEmail(c.Request.Context(), email); err == nil && existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "EMAIL_TAKEN"})
			return
		}
		if existing, err := h.contractService.GetUserByIdentity(c.Request.Context(), identity); err == nil && existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "EMAIL_TAKEN"})
			return
		}

		newUser := &models.User{
			Email:    email,
			FullName: c.Request.FormValue("user[full_name]"),
			Phone:    c.Request.FormValue("user[phone]"),
			Identity: identity,
			RTN:      c.Request.FormValue("user[rtn]"),
			Role:     models.RoleUser,
			Status:   models.StatusActive,
			Locale:   models.LocaleES,
		}
		tempPassword, err := services.GenerateTempPassword()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generando contraseña temporal"})
			return
		}
		if err := h.contractService.CreateUser(c.Request.Context(), newUser, tempPassword); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creando usuario: " + err.Error()})
			return
		}
		applicantID = newUser.ID
	} else {
		// Create New User
		email := strings.TrimSpace(c.Request.FormValue("user[email]"))
		identity := strings.TrimSpace(c.Request.FormValue("user[identity]"))

		// Required
		if email == "" || identity == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email e Identidad son requeridos para nuevos usuarios"})
			return
		}

		// Validate email format
		if _, err := mail.ParseAddress(email); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El correo electrónico no tiene un formato válido"})
			return
		}

		// Validate identity (non-empty, reasonable length to avoid garbage)
		if len(identity) < 5 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La identidad debe tener al menos 5 caracteres"})
			return
		}

		// Check duplicate by email
		existingUser, err := h.contractService.GetUserByEmail(c.Request.Context(), email)
		if err == nil && existingUser != nil {
			applicantID = existingUser.ID // Use existing user if found
		} else {
			// Check duplicate by identity
			existingUserByIdentity, err := h.contractService.GetUserByIdentity(c.Request.Context(), identity)
			if err == nil && existingUserByIdentity != nil {
				applicantID = existingUserByIdentity.ID
			} else {
				// Create new user
				newUser := &models.User{
					Email:    email,
					FullName: c.Request.FormValue("user[full_name]"),
					Phone:    c.Request.FormValue("user[phone]"),
					Identity: identity,
					RTN:      c.Request.FormValue("user[rtn]"),
					Role:     models.RoleUser,
					Status:   models.StatusActive,
					Locale:   models.LocaleES,
				}
				// Generate 5-char temp password (number, uppercase, symbol) - user must change on first login
				tempPassword, err := services.GenerateTempPassword()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generando contraseña temporal"})
					return
				}
				if err := h.contractService.CreateUser(c.Request.Context(), newUser, tempPassword); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creando usuario: " + err.Error()})
					return
				}
				applicantID = newUser.ID
			}
		}
	}

	// 5. Handle Document Uploads
	var documentPaths []string
	if form, err := c.MultipartForm(); err == nil && form != nil {
		// The frontend sends documents[0], documents[1], etc. Order is not
		// preserved by the map, but it is not critical for the stored paths.
		for key, fileHeaders := range form.File {
			if len(key) < 9 || key[:9] != "documents" {
				continue
			}
			for _, fileHeader := range fileHeaders {
				file, err := fileHeader.Open()
				if err != nil {
					continue
				}

				if !storage.IsValidContentType(fileHeader.Header.Get("Content-Type")) {
					_ = file.Close()
					continue
				}

				path, uploadErr := h.storage.Upload(file, fileHeader, "contracts")
				_ = file.Close()
				if uploadErr != nil {
					continue
				}
				documentPaths = append(documentPaths, path)
			}
		}
	}

	// 6. Create Contract Object
	var creatorRef *uint
	if creatorID > 0 {
		creatorRef = &creatorID
	}
	contract := &models.Contract{
		LotID:            lotID,
		ApplicantUserID:  applicantID,
		CreatorID:        creatorRef,
		PaymentTerm:      paymentTerm,
		FinancingType:    financingType,
		ReserveAmount:    &reserveAmount,
		DownPayment:      &downPayment,
		MaxPaymentDate:   maxPaymentDate,
		BankName:         bankNamePtr,
		BankCreditStatus: bankCreditStatusPtr,
		Note:             &note,
		Status:           models.ContractStatusPending,
		Currency:         "HNL",
	}
	if len(documentPaths) > 0 {
		if data, err := json.Marshal(documentPaths); err == nil {
			paths := string(data)
			contract.DocumentPaths = &paths
		}
	}

	// 7. Call Service
	if err := h.contractService.Create(c.Request.Context(), contract); err != nil {
		if guest && strings.Contains(strings.ToLower(err.Error()), "no está disponible") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "El lote ya no está disponible"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	created, err := h.contractService.FindByIDWithDetails(c.Request.Context(), contract.ID)
	if err != nil {
		c.JSON(http.StatusCreated, gin.H{"message": "Solicitud de contrato creada exitosamente", "contract_id": contract.ID})
		return
	}

	var reservationPayment interface{}
	for i := range created.Payments {
		if created.Payments[i].PaymentType == models.PaymentTypeReservation {
			reservationPayment = created.Payments[i].ToResponse()
			break
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":             "Solicitud de contrato creada exitosamente",
		"contract_id":         contract.ID,
		"contract":            created.ToResponse(),
		"reservation_payment": reservationPayment,
	})
}

// UpdateContractRequest is the body for PATCH contract
type UpdateContractRequest struct {
	PaymentTerm      *int     `json:"payment_term"`
	ReserveAmount    *float64 `json:"reserve_amount"`
	DownPayment      *float64 `json:"down_payment"`
	MaxPaymentDate   *string  `json:"max_payment_date"` // YYYY-MM-DD; for bank/cash
	BankName         *string  `json:"bank_name"`
	BankCreditStatus *string  `json:"bank_credit_status"`
	Note             *string  `json:"note"`
}

// @Summary Resend guest account setup email
// @Description Regenerate the temporary password and resend the customer account setup email for a reservation (no auth).
// @Tags Public Catalog
// @Produce json
// @Param guid path string true "Contract GUID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /public/reservations/{guid}/resend-access [post]
func (h *ContractHandler) ResendGuestAccess(c *gin.Context) {
	err := h.contractService.ResendAccountSetup(c.Request.Context(), c.Param("guid"))
	if err != nil {
		if errors.Is(err, services.ErrAccountAlreadyConfigured) {
			c.JSON(http.StatusConflict, gin.H{"error": "Tu cuenta ya está configurada. Inicia sesión con tu contraseña."})
			return
		}
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Reserva no encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo reenviar el correo de acceso"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Te enviamos un nuevo correo para configurar tu cuenta"})
}

// @Summary Update Contract
// @Description Update contract schedule fields (payment_term, reserve_amount, down_payment). Allowed only when status is pending, rejected, or submitted. Schedule is recalculated on approval.
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param request body UpdateContractRequest true "Contract Data"
// @Success 200 {object} models.ContractResponse
// @Failure 400,403,404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id} [patch]
func (h *ContractHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	contract, err := h.contractService.FindByIDWithDetails(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}

	var req UpdateContractRequest

	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
	}
	// Restore body for future binding
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 1. Try Nested Structure { "contract": { ... } }
	var nestedReq struct {
		Contract UpdateContractRequest `json:"contract"`
	}
	if err := json.Unmarshal(bodyBytes, &nestedReq); err == nil && (nestedReq.Contract.Note != nil || nestedReq.Contract.PaymentTerm != nil || nestedReq.Contract.BankName != nil || nestedReq.Contract.BankCreditStatus != nil) {
		req = nestedReq.Contract
	} else {
		// 2. Try Flat Structure { ... }
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
			return
		}
	}

	// 1. Check for schedule-changing fields
	status := contract.Status
	isScheduleUpdate := req.PaymentTerm != nil || req.ReserveAmount != nil || req.DownPayment != nil || req.MaxPaymentDate != nil

	if isScheduleUpdate {
		if status != models.ContractStatusPending && status != models.ContractStatusRejected && status != models.ContractStatusSubmitted {
			c.JSON(http.StatusForbidden, gin.H{"error": "Solo se pueden editar plazos, reserva y pago inicial cuando el contrato está en estado pendiente, rechazado o enviado"})
			return
		}
	}

	if req.PaymentTerm != nil {
		if *req.PaymentTerm < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El plazo de pago debe ser al menos 1"})
			return
		}
		contract.PaymentTerm = *req.PaymentTerm
	}
	if req.ReserveAmount != nil {
		if *req.ReserveAmount < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El monto de reserva no puede ser negativo"})
			return
		}
		contract.ReserveAmount = req.ReserveAmount
	}
	if req.DownPayment != nil {
		if *req.DownPayment < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El pago inicial no puede ser negativo"})
			return
		}
		contract.DownPayment = req.DownPayment
	}
	if req.MaxPaymentDate != nil {
		ft := strings.ToLower(contract.FinancingType)
		if ft == models.FinancingTypeBank || ft == models.FinancingTypeCash {
			s := strings.TrimSpace(*req.MaxPaymentDate)
			if s == "" {
				contract.MaxPaymentDate = nil
			} else {
				parsed, err := time.Parse("2006-01-02", s)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha máxima de pago debe tener formato YYYY-MM-DD"})
					return
				}
				contract.MaxPaymentDate = &parsed
			}
		}
	}

	if req.Note != nil {
		contract.Note = req.Note
	}

	if strings.EqualFold(contract.FinancingType, models.FinancingTypeBank) {
		if req.BankName != nil {
			trimmed := strings.TrimSpace(*req.BankName)
			if trimmed == "" {
				contract.BankName = nil
			} else {
				if len([]rune(trimmed)) > maxBankNameLength {
					c.JSON(http.StatusBadRequest, gin.H{"error": "El nombre del banco no puede superar los 120 caracteres"})
					return
				}
				contract.BankName = &trimmed
			}
		}
		if req.BankCreditStatus != nil {
			status, ok := models.NormalizeBankCreditStatus(*req.BankCreditStatus)
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "El estado del crédito bancario no es válido"})
				return
			}
			if status == "" {
				contract.BankCreditStatus = nil
			} else {
				contract.BankCreditStatus = &status
			}
		}
	}

	if err := h.contractService.Update(c.Request.Context(), contract); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato actualizado"})
}

// @Summary Approve Contract
// @Description Approve a pending contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} models.ContractResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/approve [post]
func (h *ContractHandler) Approve(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	contract, err := h.contractService.Approve(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato aprobado"})
}

// @Summary Pay Contract Commission
// @Description Mark the seller commission of an approved/closed contract as paid (Admin)
// @Tags Contracts
// @Produce json
// @Param guid path string true "Contract GUID"
// @Success 200 {object} models.ContractResponse
// @Security BearerAuth
// @Router /contracts/{guid}/commission/pay [patch]
func (h *ContractHandler) PayCommission(c *gin.Context) {
	contract, err := h.contractService.FindByGUIDWithDetails(c.Request.Context(), c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}

	updated, err := h.contractService.MarkCommissionPaid(c.Request.Context(), contract.ID, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contract": updated.ToResponse(), "message": "Comisión marcada como pagada"})
}

type RejectContractRequest struct {
	Reason string `json:"reason"`
}

// @Summary Reject Contract
// @Description Reject a pending contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param request body RejectContractRequest true "Reason"
// @Success 200 {object} models.ContractResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/reject [post]
func (h *ContractHandler) Reject(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	var req RejectContractRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cuerpo de la solicitud inválido"})
		return
	}

	contract, err := h.contractService.Reject(c.Request.Context(), uint(id), req.Reason)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato rechazado"})
}

type CancelContractRequest struct {
	Note string `json:"note"`
}

// @Summary Cancel Contract
// @Description Cancel an active contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param request body CancelContractRequest true "Note"
// @Success 200 {object} models.ContractResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/cancel [post]
func (h *ContractHandler) Cancel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	var req CancelContractRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cuerpo de la solicitud inválido"})
		return
	}

	contract, err := h.contractService.Cancel(c.Request.Context(), uint(id), req.Note)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato cancelado"})
}

// @Summary Reopen Contract
// @Description Reopen a cancelled contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/reopen [post]
func (h *ContractHandler) Reopen(c *gin.Context) {
	// TODO: Implement
	c.JSON(http.StatusOK, gin.H{"message": "Contrato reabierto"})
}

type CapitalRepaymentRequest struct {
	CapitalRepaymentAmount float64 `json:"capital_repayment_amount"`
	// Amount is accepted as a legacy alias used by older admin clients.
	Amount float64 `json:"amount"`
}

// resolvedAmount returns the requested repayment using the documented field
// first and the legacy alias as a fallback.
func (r CapitalRepaymentRequest) resolvedAmount() float64 {
	if r.CapitalRepaymentAmount != 0 {
		return r.CapitalRepaymentAmount
	}
	return r.Amount
}

// @Summary Capital Repayment
// @Description Apply capital repayment to contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param contract_id path int true "Contract ID"
// @Param request body CapitalRepaymentRequest true "Amount"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/capital_repayment [post]
func (h *ContractHandler) CapitalRepayment(c *gin.Context) {
	contractID, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	var req CapitalRepaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Monto requerido y debe ser mayor a 0"})
		return
	}
	amount := req.resolvedAmount()
	if amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Monto requerido y debe ser mayor a 0"})
		return
	}

	err := h.contractService.CapitalRepayment(c.Request.Context(), uint(contractID), amount,
		middleware.GetUserID(c),
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Fetch updated contract with details to return to frontend
	contract, err := h.contractService.FindByIDWithDetails(c.Request.Context(), uint(contractID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Abono a capital aplicado exitosamente, pero hubo un error al recargar los detalles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Abono a capital aplicado exitosamente",
		"contract": contract.ToResponse(),
	})
}

// @Summary Delete Rejected Contract
// @Description Delete a rejected contract and release the lot so it can be reserved again. Only allowed when contract status is rejected.
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]string
// @Failure 403,404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id} [delete]
func (h *ContractHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	if err := h.contractService.DeleteRejected(c.Request.Context(), uint(id)); err != nil {
		if err.Error() == "record not found" || strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
			return
		}
		if strings.Contains(err.Error(), "rechazado") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Contrato eliminado. El lote está disponible para nueva reserva"})
}

// @Summary Get Contract Ledger
// @Description Get ledger entries for a contract
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/ledger [get]
func (h *ContractHandler) Ledger(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	_, err := h.contractService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}
	entries, err := h.contractService.GetLedgerEntries(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ledger_entries": entries})
}

const maxBankNameLength = 120

// parseMaxPaymentDate enforces the payment-date rules per financing modality.
// Cash reservations require a date because the buyer controls the transfer.
// Bank reservations accept none, because the bank does not commit to a
// disbursement date at reservation time. Direct reservations ignore it.
func parseMaxPaymentDate(financingType, raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	switch financingType {
	case models.FinancingTypeCash:
		if raw == "" {
			return nil, errors.New("la fecha máxima de pago es requerida para financiamiento de contado")
		}
	case models.FinancingTypeBank:
		if raw == "" {
			return nil, nil
		}
	default:
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, errors.New("la fecha máxima de pago debe tener formato YYYY-MM-DD")
	}
	today := time.Now().Truncate(24 * time.Hour)
	maxDateOnly := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
	todayOnly := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if maxDateOnly.Before(todayOnly) {
		return nil, errors.New("la fecha máxima de pago no puede ser anterior a hoy")
	}
	return &parsed, nil
}

// parseBankCreditDetails normalizes the optional bank fields reported by the
// customer. Bank fields only apply to bank financing; other modalities get
// nulls so no meaningless data is stored.
func parseBankCreditDetails(financingType, bankName, creditStatus string) (*string, *string, error) {
	if financingType != models.FinancingTypeBank {
		return nil, nil, nil
	}

	name := strings.TrimSpace(bankName)
	var namePtr *string
	if name != "" {
		if len([]rune(name)) > maxBankNameLength {
			return nil, nil, errors.New("el nombre del banco no puede superar los 120 caracteres")
		}
		namePtr = &name
	}

	normalized, ok := models.NormalizeBankCreditStatus(creditStatus)
	if !ok {
		return nil, nil, errors.New("el estado del crédito bancario no es válido")
	}
	var statusPtr *string
	if normalized != "" {
		statusPtr = &normalized
	}
	return namePtr, statusPtr, nil
}
