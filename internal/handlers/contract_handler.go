package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
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
	if middleware.GetUserRole(c) == "seller" {
		query.FilterBySeller = true
	}

	contracts, total, err := h.contractService.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []interface{}
	for _, contract := range contracts {
		resp := contract.ToResponse()
		if middleware.GetUserRole(c) == "user" {
			resp = resp.RedactForCustomer()
		}
		responses = append(responses, resp)
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
	resp := contract.ToResponse()
	if role == "user" {
		resp = resp.RedactForCustomer()
	}
	c.JSON(http.StatusOK, gin.H{"contract": resp})
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

// reservationAssignmentInput resolves the assignment fields for a reservation
// created from the back office. A seller can only assign the reservation to
// themselves and it is accepted immediately; admins keep the explicit choice.
func reservationAssignmentInput(role string, creatorID uint, origin string, sellerID *uint) (string, *uint, bool) {
	if role != models.RoleSeller || creatorID == 0 {
		return origin, sellerID, false
	}
	self := creatorID
	return models.AssignmentOriginManual, &self, true
}

func (h *ContractHandler) create(c *gin.Context, lotID uint, guest bool) {
	creatorID := middleware.GetUserID(c)

	// Admins and sellers administer the marketplace; they must not create a
	// reservation as a customer from the public portal.
	if guest {
		if role := middleware.GetUserRole(c); creatorID > 0 && role != "" && role != models.RoleUser {
			c.JSON(http.StatusForbidden, gin.H{"error": "No puedes reservar desde el portal con una cuenta de administrador o vendedor"})
			return
		}
	}

	// 2. Parse Form Data
	// 8 MiB buffered in memory; larger files spill to temp files on disk. The
	// per-file size limit is enforced separately in the service.
	if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
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

	assignmentOrigin := strings.TrimSpace(c.Request.FormValue("contract[assignment_origin]"))
	if assignmentOrigin == "" {
		assignmentOrigin = strings.TrimSpace(c.Request.FormValue("assignment_origin"))
	}
	sellerIDStr := strings.TrimSpace(c.Request.FormValue("contract[seller_id]"))
	if sellerIDStr == "" {
		sellerIDStr = strings.TrimSpace(c.Request.FormValue("seller_id"))
	}
	referralGUID := strings.TrimSpace(c.Request.FormValue("contract[referral_guid]"))
	if referralGUID == "" {
		referralGUID = strings.TrimSpace(c.Request.FormValue("referral_guid"))
	}
	var requestedSellerID *uint
	if sellerIDStr != "" {
		parsed, err := strconv.ParseUint(sellerIDStr, 10, 32)
		if err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "seller_id inválido"})
			return
		}
		value := uint(parsed)
		requestedSellerID = &value
	}
	assignmentOrigin, requestedSellerID, autoAcceptAssignment := reservationAssignmentInput(
		middleware.GetUserRole(c), creatorID, assignmentOrigin, requestedSellerID,
	)
	attributionCookie, _ := c.Cookie(services.ReferralAttributionCookieName)

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
	//
	// The typed identity fields are the DNI and the RTN of the applicant. The
	// anonymous documents[N] fields keep working as free-form attachments.
	// Both are validated before anything is written: a file the API cannot
	// store is reported instead of being silently dropped.
	identityUploads, attachments, err := collectContractUploads(c)
	if err != nil {
		writeIdentityDocumentError(c, err)
		return
	}

	// The requirement is checked before the attachments are written, so a
	// rejected request leaves no orphan file behind.
	if err := h.contractService.CheckIdentityRequirement(c.Request.Context(), applicantID, identityUploads); err != nil {
		writeIdentityDocumentError(c, err)
		return
	}

	documentPaths, err := h.storeContractAttachments(c, attachments)
	if err != nil {
		writeIdentityDocumentError(c, err)
		return
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
	identityResult, err := h.contractService.CreateWithIdentityDocuments(c.Request.Context(), contract, services.ContractAssignmentInput{
		Origin:            assignmentOrigin,
		SellerID:          requestedSellerID,
		ReferralGUID:      referralGUID,
		AttributionCookie: attributionCookie,
		AutoAccept:        autoAcceptAssignment,
	}, identityUploads, documentPaths)
	if err != nil {
		if errors.Is(err, services.ErrAssignmentSellerInactive) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "El vendedor seleccionado no está activo", "code": "SELLER_INACTIVE"})
			return
		}
		if guest && strings.Contains(strings.ToLower(err.Error()), "no está disponible") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "El lote ya no está disponible", "code": "LOT_UNAVAILABLE"})
			return
		}
		writeIdentityDocumentError(c, err)
		return
	}

	if contract.ReferralGUID != nil {
		h.contractService.ClearReferralAttribution(c.Writer)
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
		// The kinds the applicant already had on file: their uploaded file was
		// not written, on purpose.
		"already_present": identityResult.AlreadyPresent,
	})
}

// collectContractUploads reads the files of a contract creation request and
// validates them. The typed identity fields are the DNI and the RTN of the
// applicant; the anonymous documents[N] fields are free-form attachments.
//
// A file the API cannot store is an error that names the file, not a file
// that is silently dropped.
func collectContractUploads(c *gin.Context) ([]services.IdentityDocumentUpload, []*multipart.FileHeader, error) {
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		return nil, nil, nil
	}

	var identityUploads []services.IdentityDocumentUpload
	for field, kind := range map[string]string{
		"identity_document": models.IdentityDocumentKindDNI,
		"rtn_document":      models.IdentityDocumentKindRTN,
	} {
		headers, ok := form.File[field]
		if !ok || len(headers) == 0 {
			continue
		}
		upload := services.IdentityDocumentUpload{Kind: kind, Header: headers[0]}
		if err := services.ValidateUpload(upload); err != nil {
			return nil, nil, err
		}
		identityUploads = append(identityUploads, upload)
	}

	var attachments []*multipart.FileHeader
	for key, headers := range form.File {
		if len(key) < 9 || key[:9] != "documents" {
			continue
		}
		for _, header := range headers {
			if err := validateContractAttachment(header); err != nil {
				return nil, nil, err
			}
			attachments = append(attachments, header)
		}
	}

	return identityUploads, attachments, nil
}

// validateContractAttachment checks one free-form attachment.
func validateContractAttachment(header *multipart.FileHeader) error {
	if header == nil {
		return nil
	}
	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if !storage.IsValidContentType(contentType) {
		return &services.ErrIdentityDocumentType{Filename: header.Filename, ContentType: contentType}
	}
	if header.Size > storage.MaxFileSize() {
		return &services.ErrIdentityDocumentSize{
			Filename: header.Filename,
			Size:     header.Size,
			Max:      storage.MaxFileSize(),
		}
	}
	return nil
}

// storeContractAttachments writes the free-form attachments of a contract
// creation and returns their stored paths.
func (h *ContractHandler) storeContractAttachments(c *gin.Context, attachments []*multipart.FileHeader) ([]string, error) {
	var paths []string
	for _, header := range attachments {
		file, err := header.Open()
		if err != nil {
			return nil, err
		}
		path, uploadErr := h.storage.Upload(file, header, "contracts")
		_ = file.Close()
		if uploadErr != nil {
			return nil, uploadErr
		}
		paths = append(paths, path)
	}
	return paths, nil
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
		var readErr error
		bodyBytes, readErr = io.ReadAll(c.Request.Body)
		if readErr != nil {
			if IsBodyTooLarge(readErr) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "El cuerpo de la solicitud excede el tamaño máximo permitido"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error leyendo el cuerpo de la solicitud: " + readErr.Error()})
			return
		}
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
	// Reload so the response includes the generated payment schedule and the
	// next installment instead of the pre-approval snapshot.
	if refreshed, refreshErr := h.contractService.FindByIDWithDetails(c.Request.Context(), contract.ID); refreshErr == nil {
		contract = refreshed
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

	contract, err := h.contractService.Cancel(c.Request.Context(), uint(id), req.Note, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato cancelado"})
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

// @Summary List contract assignments
// @Description Assignment history of a contract, newest first. Admin only.
// @Tags Contracts
// @Produce json
// @Param guid path string true "Contract GUID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /contracts/{guid}/assignments [get]
func (h *ContractHandler) Assignments(c *gin.Context) {
	history, err := h.contractService.AssignmentHistory(c.Request.Context(), c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}
	responses := make([]models.AssignmentResponse, 0, len(history))
	for i := range history {
		responses = append(responses, history[i].ToResponse())
	}
	c.JSON(http.StatusOK, gin.H{"assignments": responses})
}

type reassignRequest struct {
	Mode           string `json:"mode"`
	SellerID       *uint  `json:"seller_id"`
	Accept         bool   `json:"accept"`
	ExcludeCurrent bool   `json:"exclude_current"`
}

// @Summary Reassign a contract seller
// @Description Assign or offer the contract to another seller in any status. A paid commission is recovered from the previous seller as a pending credit. Admin only.
// @Tags Contracts
// @Accept json
// @Produce json
// @Param guid path string true "Contract GUID"
// @Param request body map[string]interface{} true "mode (seller|automatic), seller_id, accept, exclude_current"
// @Success 200 {object} map[string]interface{}
// @Failure 400,404,409,422 {object} map[string]string
// @Security BearerAuth
// @Router /contracts/{guid}/assignments [post]
func (h *ContractHandler) Reassign(c *gin.Context) {
	var input reassignRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Solicitud de reasignación inválida"})
		return
	}
	contract, assignment, err := h.contractService.Reassign(c.Request.Context(), c.Param("guid"), services.ContractReassignInput{
		Mode:           input.Mode,
		SellerID:       input.SellerID,
		Accept:         input.Accept,
		ExcludeCurrent: input.ExcludeCurrent,
		ActorID:        middleware.GetUserID(c),
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		case errors.Is(err, services.ErrNoAssignableSeller):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "NO_ASSIGNABLE_SELLER"})
		case errors.Is(err, services.ErrAssignmentSellerInactive):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "SELLER_INACTIVE"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	response := gin.H{"contract": contract.ToResponse()}
	if assignment != nil {
		response["assignment"] = assignment.ToResponse()
	}
	c.JSON(http.StatusOK, response)
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
