package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
)

// This file builds the commissions export document. Every line comes from
// ReportService.GenerateCommissions, which already applies the seller scope and
// includes historical sales, so the builder only groups, totals and labels.

// commissionsSellerGroup is one seller's slice of the commissions report: the
// detail lines plus the estimated, paid and pending totals used by the
// per-seller subtotal and summary blocks.
type commissionsSellerGroup struct {
	Name          string
	SellerID      *uint
	Items         []CommissionReportItem
	ContractValue float64
	Estimated     float64
	Paid          float64
	Pending       float64
}

// commissionsCreditEntry is one row of the credits and net pending section.
type commissionsCreditEntry struct {
	Key     string
	Name    string
	Pending float64
	Credits float64
	Net     float64
}

// BuildCommissionsReport builds the commissions report document: one table and
// one summary block per seller, a grand total, and a final credits/net pending
// section that reconciles payouts.
func (s *ReportExportService) BuildCommissionsReport(ctx context.Context, filters ReportExportFilters) (*ReportDocument, error) {
	now := time.Now()

	startDate := ""
	if filters.StartDate != nil {
		startDate = filters.StartDate.Format(models.HistoricalSaleDateLayout)
	}
	endDate := ""
	if filters.EndDate != nil {
		endDate = filters.EndDate.Format(models.HistoricalSaleDateLayout)
	}

	items, err := s.reportSvc.GenerateCommissions(ctx, startDate, endDate, filters.SellerID, filters.FilterBySeller)
	if err != nil {
		return nil, fmt.Errorf("failed to load commissions for the report: %w", err)
	}

	period, periodKey := reportPeriodLabel(filters.StartDate, filters.EndDate, now)

	document := &ReportDocument{
		Title:       ReportTitleCommissions,
		Subtitle:    "Comisiones agrupadas por vendedor con créditos y neto pendiente",
		Period:      period,
		PeriodKey:   periodKey,
		BaseName:    ReportBaseCommissions,
		GeneratedAt: now,
		Currency:    "HNL",
	}

	groups := commissionsGroupItems(items)
	for _, group := range groups {
		document.Sections = append(document.Sections,
			commissionsSellerSection(group),
			commissionsSellerSummarySection(group),
		)
	}
	document.Sections = append(document.Sections, commissionsGrandTotalSection(groups))

	credits := s.commissionsPendingCredits(ctx)
	document.Sections = append(document.Sections,
		commissionsCreditsSection(commissionsCreditEntries(groups, credits, filters)),
	)

	return document, nil
}

// commissionsGroupItems groups commission lines by seller name in ascending
// order. Two sellers that share a display name collapse into one group, which
// keeps the export readable when legacy rows lack a seller id.
func commissionsGroupItems(items []CommissionReportItem) []commissionsSellerGroup {
	groups := make(map[string]*commissionsSellerGroup, len(items))
	names := make([]string, 0, len(items))

	for _, item := range items {
		name := commissionsSellerDisplayName(item.Seller)
		group, ok := groups[name]
		if !ok {
			group = &commissionsSellerGroup{Name: name}
			groups[name] = group
			names = append(names, name)
		}
		if group.SellerID == nil && item.SellerID != nil {
			sellerID := *item.SellerID
			group.SellerID = &sellerID
		}

		group.Items = append(group.Items, item)
		group.ContractValue += item.Amount
		group.Estimated += item.Commission
		if item.CommissionPaidAt != nil {
			group.Paid += item.Commission
		} else {
			group.Pending += item.Commission
		}
	}

	sort.Strings(names)

	result := make([]commissionsSellerGroup, 0, len(names))
	for _, name := range names {
		result = append(result, *groups[name])
	}
	return result
}

// commissionsSellerDisplayName normalises the seller label. GenerateCommissions
// emits "Unknown" when a contract has no seller, so the empty value and that
// placeholder both map to the Spanish fallback.
func commissionsSellerDisplayName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "Unknown" {
		return "Sin vendedor"
	}
	return trimmed
}

// commissionsSellerSection renders one seller's detail table with a subtotal
// row that highlights the estimated commission.
func commissionsSellerSection(group commissionsSellerGroup) ReportSection {
	rows := make([]ReportRow, 0, len(group.Items))
	for _, item := range group.Items {
		rows = append(rows, commissionsDetailRow(item))
	}

	return ReportSection{
		Title:   "Comisiones de " + group.Name,
		Kind:    ReportSectionTable,
		Headers: commissionsDetailHeaders(),
		Rows:    rows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Subtotal " + group.Name),
				TextCell(""),
				TextCell(""),
				TextCell(""),
				TextCell(""),
				MoneyCell(group.ContractValue, "HNL"),
				TextCell(""),
				TextCell(""),
				MoneyCell(group.Estimated, "HNL"),
				TextCell(commissionsCountLabel(len(group.Items))),
			},
			Kind: ReportRowSubtotal,
		},
	}
}

// commissionsSellerSummarySection renders the compact estimated/paid/pending
// block that follows each seller's detail rows.
func commissionsSellerSummarySection(group commissionsSellerGroup) ReportSection {
	return ReportSection{
		Title: "Resumen de " + group.Name,
		Kind:  ReportSectionSummary,
		Rows:  commissionsTotalsRows(group.Estimated, group.Paid, group.Pending, ReportRowNormal),
	}
}

// commissionsGrandTotalSection renders the estimated/paid/pending totals of
// every seller in the document.
func commissionsGrandTotalSection(groups []commissionsSellerGroup) ReportSection {
	var estimated, paid, pending float64
	for _, group := range groups {
		estimated += group.Estimated
		paid += group.Paid
		pending += group.Pending
	}

	return ReportSection{
		Title: "Totales Generales",
		Kind:  ReportSectionSummary,
		Rows:  commissionsTotalsRows(estimated, paid, pending, ReportRowTotal),
	}
}

// commissionsTotalsRows builds the three-row estimated/paid/pending block
// shared by the per-seller summaries and the grand total.
func commissionsTotalsRows(estimated, paid, pending float64, kind ReportRowKind) []ReportRow {
	return []ReportRow{
		{Cells: []ReportCell{TextCell("Comisión estimada"), MoneyCell(estimated, "HNL")}, Kind: kind},
		{Cells: []ReportCell{TextCell("Comisión pagada"), MoneyCell(paid, "HNL")}, Kind: kind},
		{Cells: []ReportCell{TextCell("Comisión pendiente"), MoneyCell(pending, "HNL")}, Kind: kind},
	}
}

// commissionsDetailHeaders returns the Spanish column headers of the
// per-seller commission tables.
func commissionsDetailHeaders() []string {
	return []string{
		"Contrato",
		"Cliente",
		"Lote",
		"Proyecto",
		"Tipo de Financiamiento",
		"Valor del Contrato",
		"Fecha de Venta",
		"Origen",
		"Comisión",
		"Estado",
	}
}

// commissionsDetailRow renders one commission line, marking paid and pending
// rows with their own visual style and Spanish state text. Cancelled contracts
// keep their commission visible with an explicit note, because the payout (or
// the recoverable credit) already happened.
func commissionsDetailRow(item CommissionReportItem) ReportRow {
	kind := ReportRowPending
	state := "Pendiente"
	if item.CommissionPaidAt != nil {
		kind = ReportRowPaid
		state = "Pagada"
		if !item.CommissionPaidAt.IsZero() {
			state = "Pagada el " + FormatReportDate(*item.CommissionPaidAt)
		}
	}
	// A payout can settle part of the commission with the seller's credits of
	// cancelled contracts. State the adjustment so the row never reads as a
	// fresh full cash payment.
	if item.CreditApplied > models.MoneyEpsilon && item.CommissionPaidAt != nil {
		cash := models.RoundMoney(item.Commission - item.CreditApplied)
		if cash < 0 {
			cash = 0
		}
		state += fmt.Sprintf(" · Reajuste por crédito: %s (efectivo %s)",
			FormatReportMoney("HNL", item.CreditApplied), FormatReportMoney("HNL", cash))
	}
	if item.ContractStatus == models.ContractStatusCancelled {
		if item.CommissionPaidAt != nil {
			state += " (contrato cancelado)"
		} else {
			state = "Cancelada (sin pago)"
			kind = ReportRowMuted
		}
	}

	return ReportRow{
		Cells: []ReportCell{
			TextCell(commissionsContractReference(item)),
			TextCell(item.ClientName),
			TextCell(item.Lot),
			TextCell(item.Project),
			TextCell(TranslateFinancingType(item.FinancingType)),
			MoneyCell(item.Amount, "HNL"),
			commissionsDateCell(item.Date),
			TextCell(TranslateCommissionOrigin(item.Origin)),
			MoneyCell(item.Commission, "HNL"),
			TextCell(state),
		},
		Kind: kind,
	}
}

// commissionsContractReference labels historical sales apart from contracts so
// the report never shows a fake contract id for them.
func commissionsContractReference(item CommissionReportItem) string {
	if item.IsHistorical || item.HistoricalSaleID != nil {
		if item.HistoricalSaleID != nil {
			return fmt.Sprintf("Histórico #%d", *item.HistoricalSaleID)
		}
		return "Histórico"
	}
	return fmt.Sprintf("#%d", item.ContractID)
}

// commissionsDateCell parses the ISO sale date into a typed date cell and
// falls back to plain text when the value is missing or malformed.
func commissionsDateCell(value string) ReportCell {
	if strings.TrimSpace(value) == "" {
		return TextCell("")
	}
	parsed, err := time.ParseInLocation(models.HistoricalSaleDateLayout, value, time.Local)
	if err != nil {
		return TextCell(value)
	}
	return DateCell(parsed)
}

// commissionsCountLabel pluralises the line count without leaking English.
func commissionsCountLabel(count int) string {
	if count == 1 {
		return "1 comisión"
	}
	return fmt.Sprintf("%d comisiones", count)
}

// commissionsPendingCredits loads each seller's remaining pending credit
// balance (amount - applied_amount). A failed lookup must not hide the
// commissions already computed, so the export falls back to zero credits and
// the net pending amount equals the pending commissions for every seller.
func (s *ReportExportService) commissionsPendingCredits(ctx context.Context) map[uint]float64 {
	if s.commissionRepo == nil {
		return map[uint]float64{}
	}
	credits, err := s.commissionRepo.ListPendingCreditsBySeller(ctx)
	if err != nil {
		return map[uint]float64{}
	}
	if credits == nil {
		return map[uint]float64{}
	}
	return credits
}

// commissionsCreditEntries merges the sellers present in the commissions table
// with the sellers that only have pending credits, then computes the net
// pending amount as pending commissions minus pending credits.
func commissionsCreditEntries(
	groups []commissionsSellerGroup,
	credits map[uint]float64,
	filters ReportExportFilters,
) []commissionsCreditEntry {
	entries := make(map[string]*commissionsCreditEntry, len(groups)+len(credits))
	sellerKeys := make(map[uint]string, len(groups))

	for _, group := range groups {
		key := "nombre:" + group.Name
		entries[key] = &commissionsCreditEntry{
			Key:     key,
			Name:    group.Name,
			Pending: group.Pending,
		}
		if group.SellerID != nil {
			if _, ok := sellerKeys[*group.SellerID]; !ok {
				sellerKeys[*group.SellerID] = key
			}
		}
	}

	for sellerID, amount := range credits {
		if filters.FilterBySeller && sellerID != filters.SellerID {
			continue
		}
		key, ok := sellerKeys[sellerID]
		if !ok {
			key = fmt.Sprintf("vendedor:%d", sellerID)
			if _, exists := entries[key]; !exists {
				entries[key] = &commissionsCreditEntry{
					Key:  key,
					Name: fmt.Sprintf("Vendedor #%d", sellerID),
				}
			}
		}
		entries[key].Credits = amount
	}

	result := make([]commissionsCreditEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Net = entry.Pending - entry.Credits
		result = append(result, *entry)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].Key < result[j].Key
		}
		return result[i].Name < result[j].Name
	})
	return result
}

// commissionsCreditsSection renders the credits and net pending table with one
// row per seller and a total row.
func commissionsCreditsSection(entries []commissionsCreditEntry) ReportSection {
	rows := make([]ReportRow, 0, len(entries))
	var pendingTotal, creditsTotal, netTotal float64
	for _, entry := range entries {
		pendingTotal += entry.Pending
		creditsTotal += entry.Credits
		netTotal += entry.Net
		rows = append(rows, ReportRow{
			Cells: []ReportCell{
				TextCell(entry.Name),
				MoneyCell(entry.Pending, "HNL"),
				MoneyCell(entry.Credits, "HNL"),
				MoneyCell(entry.Net, "HNL"),
			},
			Kind: ReportRowNormal,
		})
	}

	return ReportSection{
		Title:   "Créditos y Neto Pendiente",
		Kind:    ReportSectionTable,
		Headers: []string{"Vendedor", "Comisiones Pendientes", "Créditos Pendientes", "Neto Pendiente"},
		Rows:    rows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Total"),
				MoneyCell(pendingTotal, "HNL"),
				MoneyCell(creditsTotal, "HNL"),
				MoneyCell(netTotal, "HNL"),
			},
			Kind: ReportRowTotal,
		},
		Note: "El neto pendiente se calcula como las comisiones pendientes menos los créditos pendientes.",
	}
}
