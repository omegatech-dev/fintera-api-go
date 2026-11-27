package services

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
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
	ContractStatus   string               `json:"contract_status,omitempty"`
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
	CreditApplied    float64              `json:"credit_applied"`
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
	commissionRepo  repository.CommissionRepository
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

// SetCommissionRepository adds the seller commission credits to the commission
// report. It is optional so focused tests can build a service without it.
func (s *ReportService) SetCommissionRepository(repo repository.CommissionRepository) {
	s.commissionRepo = repo
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
	// The stream reads in fixed batches; Page/PerPage are not used.
	listQuery.PerPage = 0

	if startDate != "" && endDate != "" {
		// Commission reports follow the accrual date, which is the approval
		// date for reservations. Legacy rows fall back to approved_at.
		listQuery.Filters["commission_from"] = startDate
		listQuery.Filters["commission_to"] = endDate
	}

	// Include approved, closed and cancelled contracts. Cancelling a contract
	// must not hide a commission that was already paid; every row exposes the
	// contract status so the UI can tell them apart.
	listQuery.Filters["status_in"] = fmt.Sprintf("%s,%s,%s",
		models.ContractStatusApproved, models.ContractStatusClosed, models.ContractStatusCancelled)

	query := &repository.ContractQuery{
		ListQuery:      listQuery,
		UserID:         userID,
		IsAdmin:        !filterByCreator,
		FilterBySeller: filterByCreator,
		// Status field in struct is ignored if status_in filter is present, but clear it to be safe/explicit
		Status: "",
	}

	start, end, hasPeriod := parseReportPeriod(startDate, endDate)

	var items []CommissionReportItem

	financingTypeTranslations := map[string]string{
		models.FinancingTypeDirect: "Directo",
		models.FinancingTypeBank:   "Bancario",
		models.FinancingTypeCash:   "Contado",
	}

	// Read contracts in keyset batches: peak memory is one batch of contracts
	// (preloading only the associations this report reads, and skipping the
	// TotalPaid aggregate and next-payment attachment List would add) instead
	// of every approved and closed contract at once. Row order matches the
	// previous List path (created_at DESC), so the JSON report is unchanged.
	err := s.contractRepo.StreamForCommissions(ctx, query, repository.PaymentScanBatchSize, func(contracts []models.Contract) error {
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
			} else if c.Creator != nil && c.Creator.FullName != "" &&
				(c.Creator.IsSeller() || c.Creator.IsAdmin()) {
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
				ContractStatus:   c.Status,
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
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Netted commissions report how much of the payout was settled with the
	// seller's credits. One grouped query for the whole report.
	if s.commissionRepo != nil {
		ids := make([]uint, 0, len(items))
		for i := range items {
			if items[i].CommissionPaidAt != nil && items[i].ContractID > 0 {
				ids = append(ids, items[i].ContractID)
			}
		}
		applied, appliedErr := s.commissionRepo.ListAppliedTotalByContractIDs(ctx, ids)
		if appliedErr != nil {
			return nil, appliedErr
		}
		for i := range items {
			if applied := applied[items[i].ContractID]; applied > 0 {
				items[i].CreditApplied = models.RoundMoney(applied)
			}
		}
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

// CommissionCredits returns the seller commission credits created by cancelled
// contracts. sellerID > 0 limits the list to one seller. The second return
// value is the still-pending remaining balance so the report can expose it.
func (s *ReportService) CommissionCredits(ctx context.Context, sellerID uint) ([]models.SellerCommissionCreditResponse, float64, error) {
	if s.commissionRepo == nil {
		return []models.SellerCommissionCreditResponse{}, 0, nil
	}
	credits, err := s.commissionRepo.ListCredits(ctx, sellerID)
	if err != nil {
		return nil, 0, err
	}
	responses, err := creditResponses(ctx, s.commissionRepo, credits)
	if err != nil {
		return nil, 0, err
	}
	pending := 0.0
	for i := range responses {
		if responses[i].Status != models.CommissionCreditStatusSettled {
			pending += responses[i].RemainingAmount
		}
	}
	return responses, models.RoundMoney(pending), nil
}

// PendingCommissionTotal sums the commission rows that have not been paid out.
func PendingCommissionTotal(items []CommissionReportItem) float64 {
	total := 0.0
	for _, item := range items {
		if item.CommissionPaidAt == nil {
			total += item.Commission
		}
	}
	return total
}

// CommissionTotals are the KPIs of the commission report over every row of the
// filtered period, independent of the page being returned.
type CommissionTotals struct {
	Commission  float64 `json:"commission"`
	Amount      float64 `json:"amount"`
	Paid        float64 `json:"paid"`
	Pending     float64 `json:"pending"`
	PaidCount   int     `json:"paid_count"`
	AverageRate float64 `json:"average_rate"`
	NetPending  float64 `json:"net_pending"`
}

// CommissionTotalsFor sums the filtered commission rows. The average rate is
// weighted by amount and net pending subtracts the pending seller credits.
func CommissionTotalsFor(items []CommissionReportItem, pendingCredits float64) CommissionTotals {
	var totals CommissionTotals
	for _, item := range items {
		totals.Commission += item.Commission
		totals.Amount += item.Amount
		if item.CommissionPaidAt != nil {
			totals.Paid += item.Commission
			totals.PaidCount++
		} else {
			totals.Pending += item.Commission
		}
	}
	if totals.Amount != 0 {
		totals.AverageRate = totals.Commission / totals.Amount * 100
	}
	totals.NetPending = totals.Pending - pendingCredits
	return totals
}

// CommissionSeller is one entry of the seller selector.
type CommissionSeller struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// CommissionSellersFor lists the sellers that appear in the rows, sorted by
// name. Rows without a seller id are skipped.
func CommissionSellersFor(items []CommissionReportItem) []CommissionSeller {
	seen := make(map[uint]bool)
	sellers := []CommissionSeller{}
	for _, item := range items {
		if item.SellerID == nil || seen[*item.SellerID] {
			continue
		}
		seen[*item.SellerID] = true
		sellers = append(sellers, CommissionSeller{ID: *item.SellerID, Name: item.Seller})
	}
	slices.SortFunc(sellers, func(a, b CommissionSeller) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return sellers
}

// FilterCommissionsBySeller keeps the rows whose seller id matches. Rows
// without a seller id never match.
func FilterCommissionsBySeller(items []CommissionReportItem, sellerID uint) []CommissionReportItem {
	filtered := []CommissionReportItem{}
	for _, item := range items {
		if item.SellerID != nil && *item.SellerID == sellerID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// FilterCommissionsBySearch keeps the rows that contain the term,
// case-insensitively, in the client, project, lot, seller or contract id. The
// contract and historical sale ids match with and without the "#" prefix, so
// both "#43" and "43" find the same row. A blank term leaves every row
// untouched and the original order is preserved.
func FilterCommissionsBySearch(items []CommissionReportItem, term string) []CommissionReportItem {
	needle := strings.ToLower(strings.TrimSpace(term))
	if needle == "" {
		return items
	}

	filtered := make([]CommissionReportItem, 0, len(items))
	for _, item := range items {
		if commissionMatchesSearch(item, needle) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// commissionMatchesSearch reports whether one row contains the lower-cased
// needle in any of its searchable fields.
func commissionMatchesSearch(item CommissionReportItem, needle string) bool {
	fields := []string{item.ClientName, item.Project, item.Lot, item.Seller}
	if item.ContractID > 0 {
		fields = append(fields, contractReference(item.ContractID), strconv.FormatUint(uint64(item.ContractID), 10))
	}
	if item.HistoricalSaleID != nil {
		fields = append(fields, contractReference(*item.HistoricalSaleID), strconv.FormatUint(uint64(*item.HistoricalSaleID), 10))
	}

	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

// contractReference renders an id with the "#" prefix used by the report.
func contractReference(id uint) string {
	return fmt.Sprintf("#%d", id)
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
			if payment.Status == models.PaymentStatusPending && models.IsBeforeBusinessDay(payment.DueDate, now) {
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
		if statusKey == "pending" && models.IsBeforeBusinessDay(payment.DueDate, today) {
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
