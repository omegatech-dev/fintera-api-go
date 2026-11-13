package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

type CommissionReportItem struct {
	ContractID       uint                 `json:"contract_id"`
	ContractGUID     string               `json:"contract_guid"`
	HistoricalSaleID *uint                `json:"historical_sale_id,omitempty"`
	IsHistorical     bool                 `json:"is_historical,omitempty"`
	CommissionPaidAt *time.Time           `json:"commission_paid_at"`
	CommissionStatus string               `json:"commission_status,omitempty"`
	Origin           string               `json:"origin"`
	ClientName       string               `json:"client_name"`
	Project          string               `json:"project"`
	Lot              string               `json:"lot"`
	FinancingType    string               `json:"financing_type"`
	Amount           float64              `json:"amount"`
	Date             string               `json:"date"`
	Seller           string               `json:"seller"`
	SellerID         *uint                `json:"seller_id,omitempty"`
	Commission       float64              `json:"commission"`
	Score            *int                 `json:"score"`
	ScoreLevel       *string              `json:"score_level"`
	ScoreMetrics     *models.ScoreMetrics `json:"score_metrics,omitempty"`
}

type ReportService struct {
	paymentRepo     repository.PaymentRepository
	contractRepo    repository.ContractRepository
	userRepo        repository.UserRepository
	instructionRepo repository.ProjectPaymentInstructionRepository
	historicalRepo  repository.HistoricalSaleRepository
	scoreSvc        *ScoreService
}

func NewReportService(
	paymentRepo repository.PaymentRepository,
	contractRepo repository.ContractRepository,
	userRepo repository.UserRepository,
	instructionRepo repository.ProjectPaymentInstructionRepository,
) *ReportService {
	return &ReportService{
		paymentRepo:     paymentRepo,
		contractRepo:    contractRepo,
		userRepo:        userRepo,
		instructionRepo: instructionRepo,
	}
}

// SetHistoricalSaleRepository adds historical commissions to the commission
// report. It is optional so the PDF-only tests can build a service without it.
func (s *ReportService) SetHistoricalSaleRepository(repo repository.HistoricalSaleRepository) {
	s.historicalRepo = repo
}

// SetScoreService adds the seller score/level and breakdown to the commission
// report rows.
func (s *ReportService) SetScoreService(scoreSvc *ScoreService) {
	s.scoreSvc = scoreSvc
}

// GenerateCommissions returns a list of commission data. When userID > 0 and filterByCreator
// is true (e.g. for sellers), only contracts created by that user are returned.
// Reservation commissions use their accrual date (approval date) and historical
// sales use their sale date, so both land in the period the UI selected.
func (s *ReportService) GenerateCommissions(ctx context.Context, startDate, endDate string, userID uint, filterByCreator bool) ([]CommissionReportItem, error) {
	listQuery := repository.NewListQuery()
	listQuery.PerPage = 0 // Fetch ALL contracts, no pagination limit

	if startDate != "" && endDate != "" {
		// Commission reports follow the accrual date, which is the approval
		// date for reservations. Legacy rows fall back to approved_at.
		listQuery.Filters["commission_from"] = startDate
		listQuery.Filters["commission_to"] = endDate
	}

	// Include both Approved and Closed contracts
	listQuery.Filters["status_in"] = fmt.Sprintf("%s,%s", models.ContractStatusApproved, models.ContractStatusClosed)

	query := &repository.ContractQuery{
		ListQuery:      listQuery,
		UserID:         userID,
		IsAdmin:        !filterByCreator,
		FilterBySeller: filterByCreator,
		// Status field in struct is ignored if status_in filter is present, but clear it to be safe/explicit
		Status: "",
	}

	contracts, _, err := s.contractRepo.List(ctx, query)
	if err != nil {
		return nil, err
	}

	start, end, hasPeriod := parseReportPeriod(startDate, endDate)

	var items []CommissionReportItem

	financingTypeTranslations := map[string]string{
		models.FinancingTypeDirect: "Directo",
		models.FinancingTypeBank:   "Bancario",
		models.FinancingTypeCash:   "Contado",
	}

	for _, c := range contracts {
		commissionDate := c.CommissionDate()
		if hasPeriod && (commissionDate == nil || !withinReportPeriod(*commissionDate, start, end)) {
			continue
		}

		clientName := "N/A"
		if c.ApplicantUser.ID != 0 {
			clientName = c.ApplicantUser.FullName
		}

		projectName := "N/A"
		lotNumber := "N/A"
		if c.Lot.ID != 0 {
			lotNumber = c.Lot.Name
			if c.Lot.Project.ID != 0 {
				projectName = c.Lot.Project.Name
			}
		}

		amount := 0.0
		if c.Amount != nil {
			amount = *c.Amount
		}

		dateStr := ""
		if commissionDate != nil {
			dateStr = commissionDate.Format("2006-01-02")
		}

		sellerName := "Unknown"
		if c.Seller != nil && c.Seller.FullName != "" {
			sellerName = c.Seller.FullName
		} else if c.Creator != nil {
			sellerName = c.Creator.FullName
		}
		commission := c.CommissionAmount
		if commission == 0 {
			commission = c.CalculateCommission()
		}

		financingType := c.FinancingType
		if val, ok := financingTypeTranslations[financingType]; ok {
			financingType = val
		}

		origin := c.CommissionOrigin
		if origin == "" {
			origin = models.CommissionOriginDirect
		}

		sellerID := c.SellerID
		if sellerID == nil && c.Creator != nil && c.Creator.IsSeller() {
			creatorID := c.Creator.ID
			sellerID = &creatorID
		}

		items = append(items, CommissionReportItem{
			ContractID:       c.ID,
			ContractGUID:     c.GUID,
			CommissionPaidAt: c.CommissionPaidAt,
			CommissionStatus: commissionStatusForContract(c),
			Origin:           origin,
			ClientName:       clientName,
			Project:          projectName,
			Lot:              lotNumber,
			FinancingType:    financingType,
			Amount:           amount,
			Date:             dateStr,
			Seller:           sellerName,
			SellerID:         sellerID,
			Commission:       commission,
		})
	}

	if s.historicalRepo != nil {
		var creatorID *uint
		if filterByCreator {
			id := userID
			creatorID = &id
		}
		var since, until *time.Time
		if hasPeriod {
			since, until = &start, &end
		}
		sales, err := s.historicalRepo.ListForReport(ctx, creatorID, since, until)
		if err != nil {
			return nil, err
		}
		for i := range sales {
			sale := &sales[i]
			sellerID := sale.CreatedBy
			sellerName := "Unknown"
			if sale.Creator != nil && sale.Creator.FullName != "" {
				sellerName = sale.Creator.FullName
			}
			item := CommissionReportItem{
				HistoricalSaleID: &sale.ID,
				IsHistorical:     true,
				CommissionStatus: sale.CommissionStatus,
				Origin:           models.CommissionOriginHistorical,
				ClientName:       sale.ClientName,
				Project:          firstNonEmpty(sale.ProjectName, "N/A"),
				Lot:              firstNonEmpty(pointerValue(sale.LotLabel), "N/A"),
				FinancingType:    translateHistoricalPaymentMode(pointerValue(sale.PaymentMode)),
				Amount:           sale.SalePrice,
				Date:             sale.SaleDate.Format(models.HistoricalSaleDateLayout),
				Seller:           sellerName,
				SellerID:         &sellerID,
				Commission:       sale.CommissionAmount,
			}
			// Historical sales have no payout timestamp; when the commission is
			// already paid, use the sale date so the UI counts it as paid.
			if sale.CommissionStatus == "paid" && !sale.SaleDate.IsZero() {
				paidAt := sale.SaleDate
				item.CommissionPaidAt = &paidAt
			}
			items = append(items, item)
		}
	}

	s.enrichCommissionScores(ctx, items)
	return items, nil
}

func (s *ReportService) enrichCommissionScores(ctx context.Context, items []CommissionReportItem) {
	if len(items) == 0 {
		return
	}
	ids := make([]uint, 0, len(items))
	seen := make(map[uint]bool, len(items))
	for _, item := range items {
		if item.SellerID == nil || seen[*item.SellerID] {
			continue
		}
		seen[*item.SellerID] = true
		ids = append(ids, *item.SellerID)
	}
	if len(ids) == 0 {
		return
	}

	usersByID := make(map[uint]models.User, len(ids))
	if s.userRepo != nil {
		if users, err := s.userRepo.FindByIDs(ctx, ids); err == nil {
			for _, user := range users {
				usersByID[user.ID] = user
			}
		}
	}
	metrics := make(map[uint]models.ScoreMetrics, len(ids))
	if s.scoreSvc != nil {
		if computed, err := s.scoreSvc.MetricsForUsers(ctx, ids); err == nil {
			metrics = computed
		}
	}

	for i := range items {
		if items[i].SellerID == nil {
			continue
		}
		id := *items[i].SellerID
		user, ok := usersByID[id]
		if !ok || !user.IsSeller() {
			continue
		}
		score := user.Score
		level := user.ScoreLevel
		if level == "" {
			level = models.ScoreLevelFor(score)
		}
		items[i].Score = &score
		items[i].ScoreLevel = &level
		if value, ok := metrics[id]; ok {
			metric := value
			items[i].ScoreMetrics = &metric
		}
	}
}

func parseReportPeriod(startDate, endDate string) (time.Time, time.Time, bool) {
	if startDate == "" || endDate == "" {
		return time.Time{}, time.Time{}, false
	}
	start, err := time.ParseInLocation(models.HistoricalSaleDateLayout, startDate, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	end, err := time.ParseInLocation(models.HistoricalSaleDateLayout, endDate, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func withinReportPeriod(value time.Time, start, end time.Time) bool {
	date := dateOnly(value)
	return !date.Before(start) && !date.After(end)
}

func commissionStatusForContract(contract models.Contract) string {
	if contract.CommissionPaidAt != nil {
		return "paid"
	}
	return "pending"
}

func translateCommissionOrigin(origin string) string {
	switch origin {
	case models.CommissionOriginReferral:
		return "Enlace"
	case models.CommissionOriginHistorical:
		return "Histórico"
	default:
		return "Directo"
	}
}

func translateHistoricalPaymentMode(mode string) string {
	switch mode {
	case models.HistoricalPaymentModeCash:
		return "Contado"
	case models.HistoricalPaymentModeBank, "financed":
		return "Bancario"
	case models.HistoricalPaymentModeDirect:
		return "Directo"
	default:
		return firstNonEmpty(mode, "—")
	}
}

// GenerateCommissionsCSV generates a CSV report of sales and commissions
func (s *ReportService) GenerateCommissionsCSV(ctx context.Context, startDate, endDate string, userID uint, filterByCreator bool) (*bytes.Buffer, error) {
	items, err := s.GenerateCommissions(ctx, startDate, endDate, userID, filterByCreator)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	// Header
	header := []string{"Contrato ID", "Cliente", "Lote", "Proyecto", "Tipo Financiamiento", "Valor Contrato", "Fecha Venta", "Vendedor", "Origen", "Comisión Est.", "Comisión Pagada"}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, item := range items {
		record := []string{
			fmt.Sprintf("%d", item.ContractID),
			item.ClientName,
			item.Lot,
			item.Project,
			item.FinancingType,
			fmt.Sprintf("%.2f", item.Amount),
			item.Date,
			item.Seller,
			translateCommissionOrigin(item.Origin),
			fmt.Sprintf("%.2f", item.Commission),
			func() string {
				if item.CommissionPaidAt == nil {
					return "No"
				}
				return "Sí " + item.CommissionPaidAt.Format("2006-01-02")
			}(),
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRevenueCSV generates a CSV report of revenue
func (s *ReportService) GenerateRevenueCSV(ctx context.Context) (*bytes.Buffer, error) {
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	// Header
	header := []string{
		"Pago ID", "Contrato", "Tipo", "Monto Pagado", "Fecha Pago",
		"Cliente", "Identidad", "Proyecto", "Lote",
		"Financiamiento", "Plazo",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	// Translation maps
	paymentTypeTranslations := map[string]string{
		models.PaymentTypeReservation: "Reservación",
		models.PaymentTypeDownPayment: "Prima",
		models.PaymentTypeInstallment: "Cuota",
		models.PaymentTypeFull:        "Pago Total",
		models.PaymentTypeAdvance:     "Abono a Capital",
	}

	financingTypeTranslations := map[string]string{
		models.FinancingTypeDirect: "Directo",
		models.FinancingTypeBank:   "Bancario",
		models.FinancingTypeCash:   "Contado",
	}

	// Process in batches
	page := 1
	pageSize := 1000

	for {
		query := repository.NewListQuery()
		query.Filters["status"] = "paid"
		query.Page = page
		query.PerPage = pageSize

		payments, _, err := s.paymentRepo.List(ctx, query)
		if err != nil {
			return nil, err
		}

		if len(payments) == 0 {
			break
		}

		for _, p := range payments {
			// Process payment (same logic as before)
			paidAmount := 0.0
			if p.PaidAmount != nil {
				paidAmount = *p.PaidAmount
			}

			payDate := ""
			if p.PaymentDate != nil {
				payDate = p.PaymentDate.Format("2006-01-02")
			}

			clientName := "N/A"
			clientIdentity := "N/A"
			projectName := "N/A"
			lotName := "N/A"
			financingType := "N/A"
			paymentTerm := "N/A"

			if p.Contract.ID != 0 {
				if p.Contract.ApplicantUser.ID != 0 {
					clientName = p.Contract.ApplicantUser.FullName
					clientIdentity = p.Contract.ApplicantUser.Identity
				}

				if p.Contract.Lot.ID != 0 {
					lotName = p.Contract.Lot.Name
					if p.Contract.Lot.Project.ID != 0 {
						projectName = p.Contract.Lot.Project.Name
					}
				}

				if val, ok := financingTypeTranslations[p.Contract.FinancingType]; ok {
					financingType = val
				} else {
					financingType = p.Contract.FinancingType
				}

				paymentTerm = fmt.Sprintf("%d meses", p.Contract.PaymentTerm)
			}

			paymentType := p.PaymentType
			if val, ok := paymentTypeTranslations[paymentType]; ok {
				paymentType = val
			}

			record := []string{
				fmt.Sprintf("%d", p.ID),
				fmt.Sprintf("%d", p.ContractID),
				paymentType,
				fmt.Sprintf("%.2f", paidAmount),
				payDate,
				clientName,
				clientIdentity,
				projectName,
				lotName,
				financingType,
				paymentTerm,
			}
			if err := w.Write(record); err != nil {
				return nil, err
			}
		}

		w.Flush()
		if len(payments) < pageSize {
			break
		}
		page++
	}

	return b, nil
}

// GenerateOverduePaymentsCSV generates a CSV of overdue payments
func (s *ReportService) GenerateOverduePaymentsCSV(ctx context.Context) (*bytes.Buffer, error) {
	payments, err := s.paymentRepo.FindOverdue(ctx)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	header := []string{"ID", "Contrato", "Cliente", "Teléfono", "Fecha Vencimiento", "Días Mora", "Monto", "Interés Acum."}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, p := range payments {
		daysOverdue := int(time.Since(p.DueDate).Hours() / 24)

		clientName := "N/A"
		phone := "N/A"
		if p.Contract.ID != 0 && p.Contract.ApplicantUser.ID != 0 {
			clientName = p.Contract.ApplicantUser.FullName
			phone = p.Contract.ApplicantUser.Phone
		}

		interest := 0.0
		if p.InterestAmount != nil {
			interest = *p.InterestAmount
		}

		record := []string{
			fmt.Sprintf("%d", p.ID),
			fmt.Sprintf("%d", p.ContractID),
			clientName,
			phone,
			p.DueDate.Format("2006-01-02"),
			fmt.Sprintf("%d", daysOverdue),
			fmt.Sprintf("%.2f", p.Amount),
			fmt.Sprintf("%.2f", interest),
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}

	w.Flush()
	return b, nil
}

// BalancePaymentData is one row in a statement's payment schedule.
type BalancePaymentData struct {
	Index     int
	Type      string
	DueDate   string
	Amount    string
	Paid      string
	Status    string
	StatusKey string
}

// BalanceLedgerData is one non-initial ledger movement.
type BalanceLedgerData struct {
	Date        string
	EntryType   string
	Description string
	Amount      string
}

// BalanceContractData is one contract section in the statement.
type BalanceContractData struct {
	ID              uint
	Reference       string
	Status          string
	StatusKey       string
	FinancingType   string
	ProjectName     string
	ProjectAddress  string
	LotName         string
	LotArea         string
	LotDimensions   string
	LotRegistration string
	LotAddress      string
	Amount          string
	Reserve         string
	DownPayment     string
	Financed        string
	MonthlyPayment  string
	TotalPaid       string
	Balance         string
	PaymentTerm     string
	StartDate       string
	EndDate         string
	ApprovedAt      string
	Payments        []BalancePaymentData
	PaymentCount    int
	PaidCount       int
	PendingCount    int
	HasPayments     bool
	Ledger          []BalanceLedgerData
	HasLedger       bool
}

// BalanceCustomerData is the account holder block.
type BalanceCustomerData struct {
	FullName string
	Identity string
	Phone    string
	Email    string
	Address  string
}

// BalanceTotalsData is the account-level summary.
type BalanceTotalsData struct {
	Contracts  int
	Financed   string
	Paid       string
	Balance    string
	Overdue    string
	HasOverdue bool
}

// BalanceReportData is the full payload for the account statement PDF.
type BalanceReportData struct {
	Customer     BalanceCustomerData
	Date         string
	Time         string
	Reference    string
	Totals       BalanceTotalsData
	Contracts    []BalanceContractData
	HasContracts bool
}

// GenerateUserBalancePDF generates a PDF statement of account for a user. The
// statement is rendered natively with gofpdf (see balance_pdf.go).
func (s *ReportService) GenerateUserBalancePDF(ctx context.Context, userID uint) (*bytes.Buffer, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	contracts, err := s.contractRepo.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	details := make([]*models.Contract, 0, len(contracts))
	for _, c := range contracts {
		detailed, err := s.contractRepo.FindByIDWithDetails(ctx, c.ID)
		if err == nil && detailed != nil {
			details = append(details, detailed)
		}
	}

	data := s.prepareBalancePDFData(user, details, time.Now())
	return buildBalancePDF(data)
}

// prepareBalancePDFData builds the statement payload independently of PDF
// generation so the content can be unit-tested.
func (s *ReportService) prepareBalancePDFData(
	user *models.User,
	contracts []*models.Contract,
	now time.Time,
) BalanceReportData {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	customer := BalanceCustomerData{
		FullName: firstNonEmpty(user.FullName, "—"),
		Identity: firstNonEmpty(user.Identity, "—"),
		Phone:    firstNonEmpty(user.Phone, "—"),
		Email:    firstNonEmpty(user.Email, "—"),
		Address:  firstNonEmpty(pointerValue(user.Address), "—"),
	}

	var (
		totalFinanced float64
		totalPaid     float64
		totalBalance  float64
		totalOverdue  float64
		contractsData []BalanceContractData
	)

	for _, contract := range contracts {
		contractData := s.buildBalanceContractData(contract, today)
		contractsData = append(contractsData, contractData)

		totalFinanced += floatValue(contract.Amount)
		totalBalance += math.Abs(balanceOf(contract))
		for _, payment := range contract.Payments {
			if isPaidPaymentStatus(payment.Status) {
				totalPaid += paidAmountOf(payment)
			}
			if payment.Status == models.PaymentStatusPending && payment.DueDate.Before(today) {
				totalOverdue += payment.Amount
			}
		}
	}

	return BalanceReportData{
		Customer:  customer,
		Date:      s.formatDateShort(now),
		Time:      now.Format("03:04 PM"),
		Reference: fmt.Sprintf("EC-%06d", user.ID),
		Totals: BalanceTotalsData{
			Contracts:  len(contracts),
			Financed:   s.formatMoney(totalFinanced),
			Paid:       s.formatMoney(totalPaid),
			Balance:    s.formatMoney(totalBalance),
			Overdue:    s.formatMoney(totalOverdue),
			HasOverdue: totalOverdue > 0,
		},
		Contracts:    contractsData,
		HasContracts: len(contractsData) > 0,
	}
}

func (s *ReportService) buildBalanceContractData(
	contract *models.Contract,
	today time.Time,
) BalanceContractData {
	amount := floatValue(contract.Amount)
	reserve := floatValue(contract.ReserveAmount)
	downPayment := floatValue(contract.DownPayment)
	financed := amount - reserve - downPayment
	if financed < 0 {
		financed = 0
	}

	var (
		totalPaid    float64
		overdue      float64
		paidCount    int
		pendingCount int
		payments     []BalancePaymentData
		startDate    time.Time
		endDate      time.Time
		monthly      float64
	)

	for index, payment := range contract.Payments {
		statusLabel, statusKey := translatePaymentStatus(payment.Status)
		if statusKey == "pending" && payment.DueDate.Before(today) {
			statusKey = "overdue"
			overdue += payment.Amount
		}

		paid := 0.0
		if isPaidPaymentStatus(payment.Status) {
			paid = paidAmountOf(payment)
			totalPaid += paid
			paidCount++
		} else if payment.Status == models.PaymentStatusPending || payment.Status == models.PaymentStatusSubmitted {
			pendingCount++
		}

		if payment.PaymentType == models.PaymentTypeInstallment && monthly == 0 {
			monthly = payment.Amount
		}
		if !payment.DueDate.IsZero() {
			if startDate.IsZero() || payment.DueDate.Before(startDate) {
				startDate = payment.DueDate
			}
			if endDate.IsZero() || payment.DueDate.After(endDate) {
				endDate = payment.DueDate
			}
		}

		payments = append(payments, BalancePaymentData{
			Index:     index + 1,
			Type:      translatePaymentType(payment.PaymentType),
			DueDate:   s.formatDateShort(payment.DueDate),
			Amount:    s.formatMoneyCurrency(contract.Currency, payment.Amount),
			Paid:      s.formatMoneyCurrency(contract.Currency, paid),
			Status:    statusLabel,
			StatusKey: statusKey,
		})
	}

	balance := math.Abs(balanceOf(contract))
	approvedAt := "—"
	if contract.ApprovedAt != nil {
		approvedAt = s.formatDateLong(*contract.ApprovedAt)
	}

	area := contract.Lot.Area()
	unit := models.ResolveMeasurementUnit(contract.Lot.MeasurementUnit, contract.Lot.Project.MeasurementUnit)
	dimensions := "—"
	if contract.Lot.Length > 0 && contract.Lot.Width > 0 {
		dimensions = fmt.Sprintf("%.2f m × %.2f m", contract.Lot.Length, contract.Lot.Width)
	}
	registration := firstNonEmpty(pointerValue(contract.Lot.RegistrationNumber), "—")
	lotAddress := firstNonEmpty(pointerValue(contract.Lot.Address), "—")
	projectAddress := firstNonEmpty(contract.Lot.Project.Address, "—")

	ledger := make([]BalanceLedgerData, 0)
	for _, entry := range contract.LedgerEntries {
		if entry.EntryType == models.EntryTypeInitial {
			continue
		}
		ledger = append(ledger, BalanceLedgerData{
			Date:        s.formatDateShort(entry.EntryDate),
			EntryType:   translateLedgerType(entry.EntryType),
			Description: firstNonEmpty(entry.Description, "—"),
			Amount:      s.formatSignedMoney(entry.Amount),
		})
	}

	statusLabel, statusKey := translateContractStatus(contract.Status)

	return BalanceContractData{
		ID:              contract.ID,
		Reference:       shortContractReference(contract),
		Status:          statusLabel,
		StatusKey:       statusKey,
		FinancingType:   translateFinancingType(contract.FinancingType),
		ProjectName:     firstNonEmpty(contract.Lot.Project.Name, "—"),
		ProjectAddress:  projectAddress,
		LotName:         firstNonEmpty(contract.Lot.Name, "—"),
		LotArea:         fmt.Sprintf("%.2f %s", area, unit),
		LotDimensions:   dimensions,
		LotRegistration: registration,
		LotAddress:      lotAddress,
		Amount:          s.formatMoneyCurrency(contract.Currency, amount),
		Reserve:         s.formatMoneyCurrency(contract.Currency, reserve),
		DownPayment:     s.formatMoneyCurrency(contract.Currency, downPayment),
		Financed:        s.formatMoneyCurrency(contract.Currency, financed),
		MonthlyPayment:  s.formatMoneyCurrency(contract.Currency, monthly),
		TotalPaid:       s.formatMoneyCurrency(contract.Currency, totalPaid),
		Balance:         s.formatMoneyCurrency(contract.Currency, balance),
		PaymentTerm:     fmt.Sprintf("%d meses", contract.PaymentTerm),
		StartDate:       formatOptionalDate(s, startDate),
		EndDate:         formatOptionalDate(s, endDate),
		ApprovedAt:      approvedAt,
		Payments:        payments,
		PaymentCount:    len(payments),
		PaidCount:       paidCount,
		PendingCount:    pendingCount,
		HasPayments:     len(payments) > 0,
		Ledger:          ledger,
		HasLedger:       len(ledger) > 0,
	}
}

func translateContractStatus(status string) (string, string) {
	switch status {
	case models.ContractStatusPending:
		return "Pendiente de pago", "pending"
	case models.ContractStatusSubmitted:
		return "En validación", "submitted"
	case models.ContractStatusApproved, models.ContractStatusSigned:
		return "Aprobado", "paid"
	case models.ContractStatusClosed:
		return "Cerrado", "closed"
	case models.ContractStatusRejected:
		return "Rechazado", "danger"
	case models.ContractStatusCancelled:
		return "Cancelado", "danger"
	default:
		return firstNonEmpty(status, "—"), "closed"
	}
}

func translatePaymentStatus(status string) (string, string) {
	switch status {
	case models.PaymentStatusPending:
		return "Pendiente", "pending"
	case models.PaymentStatusSubmitted:
		return "En validación", "submitted"
	case models.PaymentStatusPaid, "approved":
		return "Pagado", "paid"
	case models.PaymentStatusRejected:
		return "Rechazado", "danger"
	case models.PaymentStatusReadjustment:
		return "Reajuste", "pending"
	default:
		return firstNonEmpty(status, "—"), "closed"
	}
}

func translateFinancingType(financingType string) string {
	switch financingType {
	case models.FinancingTypeDirect:
		return "Financiamiento directo"
	case models.FinancingTypeBank:
		return "Crédito bancario"
	case models.FinancingTypeCash:
		return "Pago de contado"
	default:
		return firstNonEmpty(financingType, "—")
	}
}

func translatePaymentType(paymentType string) string {
	switch paymentType {
	case models.PaymentTypeReservation:
		return "Reservación"
	case models.PaymentTypeDownPayment:
		return "Prima"
	case models.PaymentTypeInstallment:
		return "Cuota"
	case models.PaymentTypeFull:
		return "Pago total"
	case models.PaymentTypeAdvance:
		return "Abono a capital"
	case models.PaymentTypeCapitalRepayment:
		return "Pago anticipado"
	default:
		return firstNonEmpty(paymentType, "—")
	}
}

func translateLedgerType(entryType string) string {
	switch entryType {
	case models.EntryTypePayment:
		return "Pago"
	case models.EntryTypeInterest:
		return "Interés"
	case models.EntryTypePrepayment:
		return "Abono a capital"
	case models.EntryTypeAdjustment:
		return "Ajuste"
	case models.EntryTypeInitial:
		return "Saldo inicial"
	default:
		return firstNonEmpty(entryType, "—")
	}
}

// shortContractReference returns the UI-facing contract reference (#ID), which
// is also used in the downloaded file names.
func shortContractReference(contract *models.Contract) string {
	return fmt.Sprintf("#%d", contract.ID)
}

func balanceOf(contract *models.Contract) float64 {
	if contract.Balance != nil {
		return *contract.Balance
	}
	paid := 0.0
	for _, payment := range contract.Payments {
		paid += paidAmountOf(payment)
	}
	return floatValue(contract.Amount) - paid
}

func paidAmountOf(payment models.Payment) float64 {
	if payment.PaidAmount != nil {
		return *payment.PaidAmount
	}
	return payment.Amount
}

func isPaidPaymentStatus(status string) bool {
	switch status {
	case models.PaymentStatusPaid, "approved":
		return true
	default:
		return false
	}
}

func floatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatOptionalDate(s *ReportService, value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return s.formatDateLong(value)
}

// formatMoney renders an HNL amount with thousands separators, e.g. L 1,125,000.00.
func (s *ReportService) formatMoney(amount float64) string {
	return s.formatMoneyCurrency("HNL", amount)
}

// formatMoneyCurrency renders an amount with the currency symbol and thousands
// separators, e.g. L 1,125,000.00 or $ 1,125,000.00.
func (s *ReportService) formatMoneyCurrency(currency string, amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	raw := fmt.Sprintf("%.2f", amount)
	parts := strings.SplitN(raw, ".", 2)
	intPart := parts[0]

	var formatted strings.Builder
	for index, char := range intPart {
		if index > 0 && (len(intPart)-index)%3 == 0 {
			formatted.WriteByte(',')
		}
		formatted.WriteRune(char)
	}

	symbol := "L"
	if strings.EqualFold(currency, "USD") {
		symbol = "$"
	}
	return fmt.Sprintf("%s%s %s.%s", sign, symbol, formatted.String(), parts[1])
}

func (s *ReportService) formatSignedMoney(amount float64) string {
	if amount > 0 {
		return "+" + s.formatMoney(amount)
	}
	return s.formatMoney(amount)
}

// GenerateContractPDF generates a PDF for a specific contract. The contract
// PDF is rendered natively with gofpdf (see contract_pdf.go).
func (s *ReportService) GenerateContractPDF(ctx context.Context, contractID uint) (*bytes.Buffer, error) {
	contract, err := s.contractRepo.FindByIDWithDetails(ctx, contractID)
	if err != nil {
		return nil, err
	}

	data := s.prepareContractPDFData(ctx, contract)
	return buildContractPDF(data)
}

// GenerateCustomerRecordPDF generates a native PDF report for a customer
// record.
func (s *ReportService) GenerateCustomerRecordPDF(ctx context.Context, contractID uint) (*bytes.Buffer, error) {
	contract, err := s.contractRepo.FindByIDWithDetails(ctx, contractID)
	if err != nil {
		return nil, err
	}

	data := s.prepareCustomerRecordPDFData(ctx, contract)
	return buildCustomerRecordPDF(data)
}

// GenerateRescissionContractPDF generates a native PDF for a contract
// rescission.
func (s *ReportService) GenerateRescissionContractPDF(ctx context.Context, contractID uint, refundAmount, penaltyAmount float64) (*bytes.Buffer, error) {
	contract, err := s.contractRepo.FindByIDWithDetails(ctx, contractID)
	if err != nil {
		return nil, err
	}

	data := s.prepareRescissionPDFData(ctx, contract, refundAmount, penaltyAmount)
	return buildRescissionPDF(data)
}

// SellerDashboardStats holds aggregated data for the seller dashboard
type SellerDashboardStats struct {
	TotalSalesValue   float64          `json:"total_sales_value"`
	ActiveLeads       int64            `json:"active_leads"`
	PendingCommission float64          `json:"pending_commission"`
	ConversionRate    float64          `json:"conversion_rate"`
	ChartData         DashboardChart   `json:"chart_data"`
	RecentCustomers   []RecentCustomer `json:"recent_customers"`
}

type DashboardChart struct {
	Labels []string  `json:"labels"`
	Data   []float64 `json:"data"`
}

type RecentCustomer struct {
	Name    string `json:"name"`
	Project string `json:"project"`
	Status  string `json:"status"`
	Amount  string `json:"amount"`
	Date    string `json:"date"`
}

// GetSellerDashboardStats generates statistics for the seller dashboard.
// All metrics (Volumen de Ventas, Prospectos Activos, Comisión Estimada, Tasa de Conversión)
// are computed over the last 6 months ending at the given month/year.
func (s *ReportService) GetSellerDashboardStats(ctx context.Context, userID uint, month, year int) (*SellerDashboardStats, error) {
	stats := &SellerDashboardStats{}

	// 1. Last 6 months date range (ending at end of target month)
	targetDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endOfRange := targetDate.AddDate(0, 1, 0).Add(-time.Second)
	startOfRange := targetDate.AddDate(0, -5, 0)

	// 2. Active Leads (Status = Pending or Submitted, created in last 6 months)
	activeLeadsQuery := &repository.ContractQuery{
		ListQuery: repository.NewListQuery(),
		UserID:    userID,
	}
	activeLeadsQuery.Filters["status_in"] = fmt.Sprintf("%s,%s", models.ContractStatusPending, models.ContractStatusSubmitted)
	activeLeadsQuery.Filters["start_date"] = startOfRange.Format("2006-01-02")
	activeLeadsQuery.Filters["end_date"] = endOfRange.Format("2006-01-02")
	_, count, err := s.contractRepo.List(ctx, activeLeadsQuery)
	if err != nil {
		return nil, err
	}
	stats.ActiveLeads = count

	// 3. Total Sales & Commission (last 6 months)
	monthSalesQuery := &repository.ContractQuery{
		ListQuery: repository.NewListQuery(),
		UserID:    userID,
		Status:    models.ContractStatusApproved,
	}
	monthSalesQuery.Filters["approved_from"] = startOfRange.Format("2006-01-02")
	monthSalesQuery.Filters["approved_to"] = endOfRange.Format("2006-01-02")

	approvedContracts, _, err := s.contractRepo.List(ctx, monthSalesQuery)
	if err != nil {
		return nil, err
	}

	var totalSales float64
	var totalCommission float64
	for _, c := range approvedContracts {
		if c.Amount != nil {
			totalSales += *c.Amount
		}
		commission := c.CommissionAmount
		if commission == 0 {
			commission = c.CalculateCommission()
		}
		totalCommission += commission
	}
	stats.TotalSalesValue = totalSales
	stats.PendingCommission = totalCommission

	// 4. Conversion Rate (last 6 months: approved / (approved + active leads) * 100)
	approvedCount := float64(len(approvedContracts))
	totalActiveAndApproved := approvedCount + float64(stats.ActiveLeads)
	if totalActiveAndApproved > 0 {
		stats.ConversionRate = (approvedCount / totalActiveAndApproved) * 100
	}

	// 5. Recent Customers
	recentQuery := &repository.ContractQuery{
		ListQuery: &repository.ListQuery{
			Page:    1,
			PerPage: 5,
			SortBy:  "updated_at",
			SortDir: "desc",
			Filters: make(map[string]string),
		},
		UserID: userID,
	}
	recentContracts, _, err := s.contractRepo.List(ctx, recentQuery)
	if err != nil {
		return nil, err
	}

	for _, c := range recentContracts {
		clientName := "N/A"
		if c.ApplicantUser.ID != 0 {
			clientName = c.ApplicantUser.FullName
		}

		projectName := "N/A"
		if c.Lot.ID != 0 && c.Lot.Project.ID != 0 {
			projectName = c.Lot.Project.Name
		}

		amount := 0.0
		if c.Amount != nil {
			amount = *c.Amount
		}

		stats.RecentCustomers = append(stats.RecentCustomers, RecentCustomer{
			Name:    clientName,
			Project: projectName,
			Status:  c.Status,
			Amount:  fmt.Sprintf("L %.2f", amount),
			Date:    c.UpdatedAt.Format(time.RFC3339),
		})
	}

	// 6. Chart Data (Last 6 Months)
	iterDate := startOfRange

	for i := 0; i < 6; i++ {
		// Calculate sales for this month
		mStart := iterDate
		mEnd := mStart.AddDate(0, 1, 0).Add(-time.Second)

		// Query approved contracts for this specific month
		chartQuery := &repository.ContractQuery{
			ListQuery: repository.NewListQuery(),
			UserID:    userID,
			Status:    models.ContractStatusApproved,
		}
		chartQuery.Filters["approved_from"] = mStart.Format("2006-01-02")
		chartQuery.Filters["approved_to"] = mEnd.Format("2006-01-02")

		// Note: This is N+1 queries (6 queries). Acceptable for a dashboard with low concurrency.
		contracts, _, err := s.contractRepo.List(ctx, chartQuery)
		if err != nil {
			return nil, err
		}

		sales := 0.0
		for _, c := range contracts {
			if c.Amount != nil {
				sales += *c.Amount
			}
		}

		// Translate month
		monthName := getMonthNameSpanish(mStart.Month())
		stats.ChartData.Labels = append(stats.ChartData.Labels, monthName)
		stats.ChartData.Data = append(stats.ChartData.Data, sales)

		// Next month
		iterDate = iterDate.AddDate(0, 1, 0)
	}

	return stats, nil
}

func getMonthNameSpanish(m time.Month) string {
	months := []string{"", "Ene", "Feb", "Mar", "Abr", "May", "Jun", "Jul", "Ago", "Sep", "Oct", "Nov", "Dic"}
	if int(m) >= 1 && int(m) <= 12 {
		return months[int(m)]
	}
	return m.String()[:3]
}

func (s *ReportService) formatDateLong(t time.Time) string {
	return fmt.Sprintf("%d de %s del %d", t.Day(), s.getSpanishMonthFull(t.Month()), t.Year())
}

func (s *ReportService) getSpanishMonthFull(m time.Month) string {
	months := []string{"", "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}
	if int(m) >= 1 && int(m) <= 12 {
		return months[int(m)]
	}
	return m.String()
}

func (s *ReportService) formatDateShort(t time.Time) string {
	return t.Format("02/01/2006")
}
