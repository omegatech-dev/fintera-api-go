package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"gorm.io/gorm"
)

// HistoricalSaleService manages migrated sales. It deliberately depends only
// on the historical repository and the commission resolver: it never creates a
// reservation, touches a lot or recomputes a seller score.
type HistoricalSaleService struct {
	repo          repository.HistoricalSaleRepository
	commissionSvc *CommissionService
	auditSvc      *AuditService
	clients       *HistoricalClientService
	now           func() time.Time
}

// NewHistoricalSaleService creates the historical-sale service.
func NewHistoricalSaleService(
	repo repository.HistoricalSaleRepository,
	commissionSvc *CommissionService,
	auditSvc *AuditService,
	clients *HistoricalClientService,
) *HistoricalSaleService {
	return &HistoricalSaleService{
		repo:          repo,
		commissionSvc: commissionSvc,
		auditSvc:      auditSvc,
		clients:       clients,
		now:           time.Now,
	}
}

// SetClock is used by tests to make dates deterministic.
func (s *HistoricalSaleService) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// HistoricalSaleInput accepts both the persisted client_* names and the short
// aliases sent by the seller UI.
type HistoricalSaleInput struct {
	ClientName       *string  `json:"client_name"`
	FullName         *string  `json:"full_name"`
	ClientIdentity   *string  `json:"client_identity"`
	Identity         *string  `json:"identity"`
	ClientPhone      *string  `json:"client_phone"`
	Phone            *string  `json:"phone"`
	ClientEmail      *string  `json:"client_email"`
	Email            *string  `json:"email"`
	ProjectName      *string  `json:"project_name"`
	LotLabel         *string  `json:"lot_label"`
	AreaM2           *float64 `json:"area_m2"`
	SalePrice        *float64 `json:"sale_price"`
	SaleDate         *string  `json:"sale_date"`
	PaymentMode      *string  `json:"payment_mode"`
	SaleStatus       *string  `json:"sale_status"`
	InstallmentsPaid *int     `json:"installments_paid"`
	Rate             *float64 `json:"rate"`
	CommissionAmount *float64 `json:"commission_amount"`
	Amount           *float64 `json:"amount"`
	CommissionStatus *string  `json:"commission_status"`
	EvidenceFile     *string  `json:"evidence_file"`
	IsHistorical     *bool    `json:"is_historical"`
}

// HistoricalImportOptions carries the CSV import toggles and column mapping.
type HistoricalImportOptions struct {
	MarkHistorical         bool
	SkipDuplicates         bool
	CommissionWithSaleDate bool
	Mapping                map[string]string
}

// HistoricalImportRowError reports one invalid CSV row.
type HistoricalImportRowError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// HistoricalImportSkip reports one duplicate row skipped by the toggle.
type HistoricalImportSkip struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
	Name   string `json:"name,omitempty"`
}

// HistoricalImportResult is the valid/skipped/errors summary returned by the
// CSV import.
type HistoricalImportResult struct {
	Total       int                        `json:"total"`
	Valid       int                        `json:"valid"`
	Imported    int                        `json:"imported"`
	Skipped     int                        `json:"skipped"`
	Errors      []HistoricalImportRowError `json:"errors"`
	SkippedRows []HistoricalImportSkip     `json:"skipped_rows,omitempty"`
}

// Create stores one manually entered historical sale.
func (s *HistoricalSaleService) Create(ctx context.Context, actorID uint, input HistoricalSaleInput, ip, userAgent string) (*models.HistoricalSale, error) {
	sale, err := buildHistoricalSale(input)
	if err != nil {
		return nil, err
	}
	sale.CreatedBy = actorID
	sale.ValidationStatus = models.HistoricalSaleStatusPending

	duplicate, err := s.repo.ExistsByIdentity(ctx, actorID, derefHistoricalString(sale.ClientIdentity), sale.SaleDate)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, ErrHistoricalSaleDuplicate
	}

	if err := s.clients.EnsureLinked(ctx, sale, actorID); err != nil {
		return nil, err
	}
	if err := s.applyCommission(ctx, sale, actorID, input.CommissionAmount, input.Amount, true); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, sale); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrHistoricalSaleDuplicate
		}
		return nil, err
	}

	s.audit(ctx, actorID, "HISTORICAL_CREATE", sale.ID,
		fmt.Sprintf("Venta histórica creada: %s por L%.2f", sale.ClientName, sale.SalePrice), ip, userAgent)
	return s.reload(ctx, sale.ID)
}

// Update edits a pending historical sale. Only the creator or an admin may
// change it; validated records are immutable for sellers.
func (s *HistoricalSaleService) Update(ctx context.Context, id, actorID uint, input HistoricalSaleInput, isAdmin bool, ip, userAgent string) (*models.HistoricalSale, error) {
	sale, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHistoricalSaleNotFound
		}
		return nil, err
	}
	if !isAdmin {
		if sale.CreatedBy != actorID {
			return nil, ErrHistoricalSaleForbidden
		}
		if sale.ValidationStatus == models.HistoricalSaleStatusValidated {
			return nil, ErrHistoricalSaleForbidden
		}
	}

	updated, err := buildHistoricalSale(input)
	if err != nil {
		return nil, err
	}
	sale.ClientName = updated.ClientName
	sale.ClientIdentity = updated.ClientIdentity
	sale.ClientPhone = updated.ClientPhone
	sale.ClientEmail = updated.ClientEmail
	sale.ProjectName = updated.ProjectName
	sale.LotLabel = updated.LotLabel
	sale.AreaM2 = updated.AreaM2
	sale.SalePrice = updated.SalePrice
	sale.SaleDate = updated.SaleDate
	sale.PaymentMode = updated.PaymentMode
	sale.SaleStatus = updated.SaleStatus
	sale.InstallmentsPaid = updated.InstallmentsPaid
	if updated.Rate != nil {
		sale.Rate = updated.Rate
	}
	// The evidence path is only replaced when the payload carries a non-empty
	// value; an edit that does not re-attach a file must keep the evidence
	// uploaded earlier. New files always come through the dedicated upload
	// endpoint, which overwrites the path.
	if updated.EvidenceFile != nil {
		sale.EvidenceFile = updated.EvidenceFile
	}

	if err := s.clients.EnsureLinked(ctx, sale, actorID); err != nil {
		return nil, err
	}
	if err := s.clients.SyncUser(ctx, sale); err != nil {
		return nil, err
	}

	if input.CommissionStatus != nil {
		sale.CommissionStatus = normalizeCommissionStatus(*input.CommissionStatus)
	}

	if input.CommissionAmount != nil || input.Amount != nil {
		amount := input.CommissionAmount
		if amount == nil {
			amount = input.Amount
		}
		if amount != nil {
			if *amount < 0 {
				return nil, ErrHistoricalSaleInvalid
			}
			sale.CommissionAmount = *amount
		}
	} else if err := s.applyCommission(ctx, sale, sale.CreatedBy, nil, nil, true); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, sale); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrHistoricalSaleDuplicate
		}
		return nil, err
	}
	s.audit(ctx, actorID, "HISTORICAL_UPDATE", sale.ID,
		fmt.Sprintf("Venta histórica actualizada: %s", sale.ClientName), ip, userAgent)
	return s.reload(ctx, sale.ID)
}

// SetEvidence stores the path of an uploaded evidence ("respaldo") file on a
// historical sale. The creating seller or an admin may attach evidence at any
// time; the file itself is stored by the handler before this method is called.
func (s *HistoricalSaleService) SetEvidence(ctx context.Context, id, actorID uint, isAdmin bool, path, ip, userAgent string) (*models.HistoricalSale, error) {
	sale, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHistoricalSaleNotFound
		}
		return nil, err
	}
	if !isAdmin && sale.CreatedBy != actorID {
		return nil, ErrHistoricalSaleForbidden
	}
	sale.EvidenceFile = &path
	if err := s.repo.Update(ctx, sale); err != nil {
		return nil, err
	}
	s.audit(ctx, actorID, "HISTORICAL_EVIDENCE_UPLOAD", sale.ID,
		fmt.Sprintf("Evidencia subida para la venta histórica: %s", sale.ClientName), ip, userAgent)
	return s.reload(ctx, sale.ID)
}

// List returns the historical sales visible to the caller.
func (s *HistoricalSaleService) List(ctx context.Context, query *repository.HistoricalSaleQuery) ([]models.HistoricalSale, int64, error) {
	return s.repo.List(ctx, query)
}

// BackfillClients links existing historical sales to customer accounts. It is
// idempotent and is safe to run asynchronously at API startup.
func (s *HistoricalSaleService) BackfillClients(ctx context.Context, batchSize int) (HistoricalClientBackfillResult, error) {
	if s == nil || s.clients == nil {
		return HistoricalClientBackfillResult{}, nil
	}
	return s.clients.Backfill(ctx, batchSize)
}

// Get returns one historical sale. Sellers can only read their own records;
// admins can read any record.
func (s *HistoricalSaleService) Get(ctx context.Context, id, actorID uint, isAdmin bool) (*models.HistoricalSale, error) {
	sale, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHistoricalSaleNotFound
		}
		return nil, err
	}
	if !isAdmin && sale.CreatedBy != actorID {
		return nil, ErrHistoricalSaleForbidden
	}
	return sale, nil
}

// Validate marks a historical sale as validated. It never creates a reservation.
func (s *HistoricalSaleService) Validate(ctx context.Context, id, actorID uint, ip, userAgent string) (*models.HistoricalSale, error) {
	sale, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHistoricalSaleNotFound
		}
		return nil, err
	}
	if sale.ValidationStatus == models.HistoricalSaleStatusValidated {
		return s.reload(ctx, id)
	}
	at := s.now()
	if err := s.repo.UpdateValidation(ctx, id, models.HistoricalSaleStatusValidated, actorID, at, nil); err != nil {
		return nil, err
	}
	s.audit(ctx, actorID, "HISTORICAL_VALIDATE", id,
		fmt.Sprintf("Venta histórica validada: %s", sale.ClientName), ip, userAgent)
	return s.reload(ctx, id)
}

// Reject marks a historical sale as rejected with an optional reason.
func (s *HistoricalSaleService) Reject(ctx context.Context, id, actorID uint, reason string, ip, userAgent string) (*models.HistoricalSale, error) {
	sale, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHistoricalSaleNotFound
		}
		return nil, err
	}
	if sale.ValidationStatus == models.HistoricalSaleStatusRejected {
		return s.reload(ctx, id)
	}
	at := s.now()
	trimmed := strings.TrimSpace(reason)
	var reasonPtr *string
	if trimmed != "" {
		reasonPtr = &trimmed
	}
	if err := s.repo.UpdateValidation(ctx, id, models.HistoricalSaleStatusRejected, actorID, at, reasonPtr); err != nil {
		return nil, err
	}
	s.audit(ctx, actorID, "HISTORICAL_REJECT", id,
		fmt.Sprintf("Venta histórica rechazada: %s. Motivo: %s", sale.ClientName, firstNonEmpty(trimmed, "sin motivo")), ip, userAgent)
	return s.reload(ctx, id)
}

// ImportCSV parses and imports a historical-sales CSV. Invalid rows are
// reported instead of aborting the whole file.
func (s *HistoricalSaleService) ImportCSV(ctx context.Context, actorID uint, reader io.Reader, options HistoricalImportOptions, ip, userAgent string) (*HistoricalImportResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	records, err := parseHistoricalCSV(data)
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("el archivo CSV está vacío")
	}

	headers := records[0]
	index := resolveHistoricalMapping(headers, options.Mapping)
	missing := missingHistoricalColumns(index)
	result := &HistoricalImportResult{Errors: []HistoricalImportRowError{}}
	if len(missing) > 0 {
		result.Errors = append(result.Errors, HistoricalImportRowError{
			Row:    1,
			Reason: "faltan columnas obligatorias: " + strings.Join(missing, ", "),
		})
		return result, ErrHistoricalSaleInvalid
	}

	for rowIndex, record := range records[1:] {
		if isBlankCSVRow(record) {
			continue
		}
		result.Total++
		rowNumber := rowIndex + 2
		value := func(key string) string {
			col, ok := index[key]
			if !ok || col < 0 || col >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[col])
		}

		name := value("full_name")
		identity := value("identity")
		price, priceErr := parseHistoricalAmount(value("sale_price"))
		saleDate, dateErr := parseHistoricalDate(value("sale_date"))

		reasons := make([]string, 0, 4)
		if name == "" {
			reasons = append(reasons, "el nombre es obligatorio")
		}
		if identity == "" {
			reasons = append(reasons, "la identidad es obligatoria")
		}
		if priceErr != nil || price <= 0 {
			reasons = append(reasons, "el precio de venta es inválido")
		}
		if dateErr != nil {
			reasons = append(reasons, "la fecha de venta es inválida")
		}
		if len(reasons) > 0 {
			result.Errors = append(result.Errors, HistoricalImportRowError{Row: rowNumber, Reason: strings.Join(reasons, "; ")})
			continue
		}

		duplicate, dupErr := s.repo.ExistsByIdentity(ctx, actorID, identity, saleDate)
		if dupErr != nil {
			result.Errors = append(result.Errors, HistoricalImportRowError{Row: rowNumber, Reason: "no se pudo verificar el duplicado"})
			continue
		}
		if duplicate && options.SkipDuplicates {
			result.Skipped++
			result.SkippedRows = append(result.SkippedRows, HistoricalImportSkip{
				Row:    rowNumber,
				Reason: "identidad y fecha ya registradas",
				Name:   name,
			})
			continue
		}

		sale := &models.HistoricalSale{
			ClientName:       name,
			ClientIdentity:   historicalStringPtr(identity),
			ProjectName:      value("project_name"),
			SalePrice:        price,
			SaleDate:         saleDate,
			InstallmentsPaid: parseHistoricalInt(value("installments_paid")),
			CommissionStatus: normalizeCommissionStatus(value("commission_status")),
			ValidationStatus: models.HistoricalSaleStatusPending,
			CreatedBy:        actorID,
		}
		if phone := value("phone"); phone != "" {
			sale.ClientPhone = historicalStringPtr(phone)
		}
		if email := value("email"); email != "" {
			sale.ClientEmail = historicalStringPtr(email)
		}
		if lot := value("lot_label"); lot != "" {
			sale.LotLabel = historicalStringPtr(lot)
		}
		if area := parseHistoricalOptionalAmount(value("area_m2")); area != nil {
			sale.AreaM2 = area
		}
		if mode := value("payment_mode"); mode != "" {
			sale.PaymentMode = historicalStringPtr(normalizePaymentMode(mode))
		}
		if status := value("sale_status"); status != "" {
			sale.SaleStatus = historicalStringPtr(status)
		}
		if rate := parseHistoricalOptionalAmount(value("rate")); rate != nil {
			sale.Rate = rate
		}
		var csvCommission *float64
		if amount := parseHistoricalOptionalAmount(value("commission_amount")); amount != nil {
			csvCommission = amount
		}

		if options.CommissionWithSaleDate {
			if err := s.applyCommission(ctx, sale, actorID, csvCommission, nil, true); err != nil {
				result.Errors = append(result.Errors, HistoricalImportRowError{Row: rowNumber, Reason: "no se pudo calcular la comisión"})
				continue
			}
		}

		if err := s.clients.EnsureLinked(ctx, sale, actorID); err != nil {
			result.Errors = append(result.Errors, HistoricalImportRowError{Row: rowNumber, Reason: "no se pudo crear el cliente: " + err.Error()})
			continue
		}

		if err := s.repo.Create(ctx, sale); err != nil {
			if isUniqueViolation(err) {
				if options.SkipDuplicates {
					result.Skipped++
					result.SkippedRows = append(result.SkippedRows, HistoricalImportSkip{
						Row:    rowNumber,
						Reason: "identidad y fecha ya registradas",
						Name:   name,
					})
					continue
				}
				result.Errors = append(result.Errors, HistoricalImportRowError{Row: rowNumber, Reason: "identidad y fecha ya registradas"})
				continue
			}
			result.Errors = append(result.Errors, HistoricalImportRowError{Row: rowNumber, Reason: "no se pudo guardar la fila"})
			continue
		}
		result.Valid++
		result.Imported++
	}

	s.audit(ctx, actorID, "HISTORICAL_IMPORT", 0,
		fmt.Sprintf("Importación de históricos: %d válidas, %d omitidas, %d con error", result.Valid, result.Skipped, len(result.Errors)), ip, userAgent)
	return result, nil
}

// HistoricalSalesCSVTemplate returns the canonical import template.
func HistoricalSalesCSVTemplate() string {
	buffer := &bytes.Buffer{}
	buffer.WriteString("\uFEFF") // UTF-8 BOM so Excel detects the encoding
	writer := csv.NewWriter(buffer)
	_ = writer.Write([]string{"nombre", "identidad", "proyecto", "lote", "area_m2", "precio", "fecha_venta", "telefono", "email", "forma_pago", "estado_venta", "tasa", "comision"})
	_ = writer.Write([]string{"María López", "0801-1990-12345", "Residencial Los Pinos", "Lote 12", "500", "750000", "2023-05-15", "9999-8888", "maria@example.com", "financiado", "en_cuotas", "5", "37500"})
	writer.Flush()
	return buffer.String()
}

func (s *HistoricalSaleService) applyCommission(ctx context.Context, sale *models.HistoricalSale, sellerID uint, explicit, legacy *float64, useSaleDate bool) error {
	if sale == nil {
		return ErrHistoricalSaleInvalid
	}
	amount := explicit
	if amount == nil {
		amount = legacy
	}
	if amount != nil {
		if *amount < 0 {
			return ErrHistoricalSaleInvalid
		}
		sale.CommissionAmount = *amount
		if sale.Rate == nil {
			if sale.SalePrice > 0 {
				rate := roundRate(*amount / sale.SalePrice * 100)
				sale.Rate = &rate
			}
		}
		return nil
	}

	rate := models.DefaultCommissionRate
	if sale.Rate != nil {
		rate = *sale.Rate
	} else if s.commissionSvc != nil {
		resolved, err := s.commissionSvc.ResolveHistoricalRate(ctx, sellerID)
		if err != nil {
			return err
		}
		rate = resolved
	}
	sale.Rate = &rate
	sale.CommissionAmount = sale.SalePrice * (rate / 100)
	_ = useSaleDate // historical commissions always accrue on the sale date
	return nil
}

func (s *HistoricalSaleService) reload(ctx context.Context, id uint) (*models.HistoricalSale, error) {
	sale, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHistoricalSaleNotFound
		}
		return nil, err
	}
	return sale, nil
}

func (s *HistoricalSaleService) audit(ctx context.Context, actorID uint, action string, entityID uint, details, ip, userAgent string) {
	if s.auditSvc == nil {
		return
	}
	logIgnoredError("write historical sale audit log", s.auditSvc.Log(ctx, actorID, action, "HistoricalSale", entityID, details, ip, userAgent))
}

func buildHistoricalSale(input HistoricalSaleInput) (*models.HistoricalSale, error) {
	name := strings.TrimSpace(firstHistoricalString(input.ClientName, input.FullName))
	identity := strings.TrimSpace(firstHistoricalString(input.ClientIdentity, input.Identity))
	if name == "" || identity == "" {
		return nil, ErrHistoricalSaleInvalid
	}
	if input.SalePrice == nil || *input.SalePrice <= 0 {
		return nil, ErrHistoricalSaleInvalid
	}
	if input.SaleDate == nil {
		return nil, ErrHistoricalSaleInvalid
	}
	saleDate, err := parseHistoricalDate(*input.SaleDate)
	if err != nil {
		return nil, ErrHistoricalSaleInvalid
	}

	sale := &models.HistoricalSale{
		ClientName:       name,
		ClientIdentity:   historicalStringPtr(identity),
		ProjectName:      strings.TrimSpace(derefHistoricalString(input.ProjectName)),
		SalePrice:        *input.SalePrice,
		SaleDate:         saleDate,
		InstallmentsPaid: 0,
		CommissionStatus: normalizeCommissionStatus(derefHistoricalString(input.CommissionStatus)),
		ValidationStatus: models.HistoricalSaleStatusPending,
	}
	if phone := strings.TrimSpace(firstHistoricalString(input.ClientPhone, input.Phone)); phone != "" {
		sale.ClientPhone = historicalStringPtr(phone)
	}
	if email := strings.TrimSpace(firstHistoricalString(input.ClientEmail, input.Email)); email != "" {
		sale.ClientEmail = historicalStringPtr(email)
	}
	if input.LotLabel != nil {
		lot := strings.TrimSpace(*input.LotLabel)
		if lot != "" {
			sale.LotLabel = &lot
		}
	}
	if input.AreaM2 != nil {
		if *input.AreaM2 < 0 {
			return nil, ErrHistoricalSaleInvalid
		}
		sale.AreaM2 = input.AreaM2
	}
	if input.PaymentMode != nil {
		mode := normalizePaymentMode(*input.PaymentMode)
		if mode != "" {
			sale.PaymentMode = &mode
		}
	}
	if input.SaleStatus != nil {
		status := strings.TrimSpace(*input.SaleStatus)
		if status != "" {
			sale.SaleStatus = &status
		}
	}
	if input.InstallmentsPaid != nil {
		if *input.InstallmentsPaid < 0 {
			return nil, ErrHistoricalSaleInvalid
		}
		sale.InstallmentsPaid = *input.InstallmentsPaid
	}
	if input.Rate != nil {
		if !validRate(*input.Rate) {
			return nil, ErrInvalidCommissionRate
		}
		rate := *input.Rate
		sale.Rate = &rate
	}
	if input.EvidenceFile != nil {
		evidence := strings.TrimSpace(*input.EvidenceFile)
		if evidence != "" {
			sale.EvidenceFile = &evidence
		}
	}
	return sale, nil
}

func firstHistoricalString(values ...*string) string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			return *value
		}
	}
	return ""
}

func derefHistoricalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func historicalStringPtr(value string) *string {
	return &value
}

func parseHistoricalDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, ErrHistoricalSaleInvalid
	}
	layouts := []string{
		models.HistoricalSaleDateLayout,
		"2006/01/02",
		"02/01/2006",
		"02-01-2006",
		"2006-1-2",
		"2/1/2006",
		"1/2/2006", // M/D/YYYY (US style, e.g. 5/15/2023)
		time.RFC3339,
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return dateOnly(parsed), nil
		}
	}
	return time.Time{}, ErrHistoricalSaleInvalid
}

func parseHistoricalAmount(value string) (float64, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return 0, ErrHistoricalSaleInvalid
	}
	clean = strings.NewReplacer(",", "", "L", "", "$", "", " ", "").Replace(clean)
	amount, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, ErrHistoricalSaleInvalid
	}
	return amount, nil
}

func parseHistoricalOptionalAmount(value string) *float64 {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	amount, err := parseHistoricalAmount(value)
	if err != nil {
		return nil
	}
	return &amount
}

func parseHistoricalInt(value string) int {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return 0
	}
	parsed, err := strconv.Atoi(clean)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func normalizePaymentMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cash", "contado", "efectivo":
		return models.HistoricalPaymentModeCash
	case "bank", "banco", "bancario", "financed", "financiado":
		return models.HistoricalPaymentModeBank
	case "direct", "directo":
		return models.HistoricalPaymentModeDirect
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeCommissionStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "paid", "pagado", "pagada":
		return "paid"
	default:
		return "pending"
	}
}

func roundRate(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "sqlstate 23505")
}

type historicalCSVField struct {
	Key      string
	Aliases  []string
	Required bool
}

var historicalCSVFields = []historicalCSVField{
	{Key: "full_name", Aliases: []string{"nombre", "nombre completo", "nombre_completo", "cliente", "client", "name", "full_name", "client_name"}, Required: true},
	{Key: "identity", Aliases: []string{"identidad", "dni", "identity", "cedula", "documento", "client_identity"}, Required: true},
	{Key: "project_name", Aliases: []string{"proyecto", "project", "project_name"}},
	{Key: "lot_label", Aliases: []string{"lote", "lot", "lot_label"}},
	{Key: "area_m2", Aliases: []string{"area", "area_m2", "metros", "m2"}},
	{Key: "sale_price", Aliases: []string{"precio", "precio de venta", "precio_venta", "sale_price", "monto", "valor", "amount"}, Required: true},
	{Key: "sale_date", Aliases: []string{"fecha_venta", "fecha de venta", "fecha", "sale_date", "date"}, Required: true},
	{Key: "phone", Aliases: []string{"telefono", "phone", "client_phone"}},
	{Key: "email", Aliases: []string{"email", "correo", "client_email"}},
	{Key: "payment_mode", Aliases: []string{"forma_pago", "payment_mode", "pago"}},
	{Key: "sale_status", Aliases: []string{"estado_venta", "sale_status", "estado"}},
	{Key: "installments_paid", Aliases: []string{"cuotas_pagadas", "installments_paid"}},
	{Key: "rate", Aliases: []string{"tasa", "rate", "porcentaje", "commission_rate"}},
	{Key: "commission_amount", Aliases: []string{"comision", "commission", "commission_amount", "monto_comision"}},
	{Key: "commission_status", Aliases: []string{"estado_comision", "commission_status"}},
}

var csvHeaderReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c",
)

func normalizeCSVHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = csvHeaderReplacer.Replace(value)
	value = strings.ReplaceAll(value, "_", " ")
	return strings.Join(strings.Fields(value), " ")
}

func parseHistoricalCSV(data []byte) ([][]string, error) {
	// UTF-8 BOM is common in spreadsheets exports; strip it before parsing.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el CSV: %w", err)
	}
	return records, nil
}

func resolveHistoricalMapping(headers []string, mapping map[string]string) map[string]int {
	index := make(map[string]int, len(historicalCSVFields))
	normalizedHeaders := make([]string, len(headers))
	for i, header := range headers {
		normalizedHeaders[i] = normalizeCSVHeader(header)
	}

	for _, field := range historicalCSVFields {
		if raw, ok := mapping[field.Key]; ok {
			if col := resolveHistoricalColumn(raw, headers, normalizedHeaders); col >= 0 {
				index[field.Key] = col
				continue
			}
		}
		for _, alias := range field.Aliases {
			target := normalizeCSVHeader(alias)
			for i, header := range normalizedHeaders {
				if header == target {
					index[field.Key] = i
					break
				}
			}
			if _, ok := index[field.Key]; ok {
				break
			}
		}
	}
	return index
}

func resolveHistoricalColumn(raw string, headers, normalizedHeaders []string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return -1
	}
	if col, err := strconv.Atoi(raw); err == nil {
		if col >= 0 && col < len(headers) {
			return col
		}
		return -1
	}
	target := normalizeCSVHeader(raw)
	for i, header := range normalizedHeaders {
		if header == target {
			return i
		}
	}
	return -1
}

func missingHistoricalColumns(index map[string]int) []string {
	var missing []string
	for _, field := range historicalCSVFields {
		if !field.Required {
			continue
		}
		if _, ok := index[field.Key]; !ok {
			missing = append(missing, field.Key)
		}
	}
	return missing
}

func isBlankCSVRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
