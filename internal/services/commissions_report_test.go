package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commissionsTestCreditRepo is a commission repository that only answers the
// pending-credits lookup used by the commissions report.
type commissionsTestCreditRepo struct {
	repository.CommissionRepository
	pending map[uint]float64
	err     error
	calls   int
}

func (r *commissionsTestCreditRepo) ListPendingCreditsBySeller(context.Context) (map[uint]float64, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return r.pending, nil
}

func (r *commissionsTestCreditRepo) ListAppliedTotalByContractIDs(context.Context, []uint) (map[uint]float64, error) {
	return map[uint]float64{}, nil
}

func (r *commissionsTestCreditRepo) ListApplicationsByCredits(context.Context, []uint) (map[uint][]models.CreditApplication, error) {
	return map[uint][]models.CreditApplication{}, nil
}

// commissionsTestContract builds an approved contract for the commissions
// fixtures with the seller, client, lot and commission data the report needs.
func commissionsTestContract(
	id uint,
	sellerID *uint,
	sellerName, client, lot, project string,
	amount, commission float64,
	accrual time.Time,
	paidAt *time.Time,
) models.Contract {
	contract := models.Contract{
		ID:                    id,
		GUID:                  fmt.Sprintf("contract-%d", id),
		Status:                models.ContractStatusApproved,
		FinancingType:         models.FinancingTypeDirect,
		Amount:                &amount,
		CommissionAmount:      commission,
		CommissionOrigin:      models.CommissionOriginDirect,
		CommissionAccrualDate: &accrual,
		CommissionPaidAt:      paidAt,
		SellerID:              sellerID,
		ApplicantUser:         models.User{ID: 100 + id, FullName: client},
		Lot: models.Lot{
			ID:      id,
			Name:    lot,
			Project: models.Project{ID: 1, Name: project},
		},
	}
	if sellerID != nil {
		contract.Seller = &models.User{ID: *sellerID, FullName: sellerName, Role: models.RoleSeller}
	}
	return contract
}

// commissionsTestContractRepo returns the fixture contracts and records the
// query so the seller scope can be asserted. It mimics the SQL layer by
// filtering on the seller when FilterBySeller is set.
func commissionsTestContractRepo(contracts []models.Contract, captured **repository.ContractQuery) *mockContractRepository {
	return &mockContractRepository{
		mockList: func(_ context.Context, query *repository.ContractQuery) ([]models.Contract, int64, error) {
			if captured != nil {
				*captured = query
			}
			if !query.FilterBySeller {
				return contracts, int64(len(contracts)), nil
			}
			scoped := make([]models.Contract, 0, len(contracts))
			for _, contract := range contracts {
				if contract.SellerID != nil && *contract.SellerID == query.UserID {
					scoped = append(scoped, contract)
				}
			}
			return scoped, int64(len(scoped)), nil
		},
	}
}

// commissionsTestSection returns the section with the given title.
func commissionsTestSection(t *testing.T, doc *ReportDocument, title string) ReportSection {
	t.Helper()
	for _, section := range doc.Sections {
		if section.Title == title {
			return section
		}
	}
	t.Fatalf("section %q not found in the commissions document", title)
	return ReportSection{}
}

// commissionsTestMoney reads the numeric value of a money cell.
func commissionsTestMoney(t *testing.T, cell ReportCell) float64 {
	t.Helper()
	require.NotNil(t, cell.Money, "expected a money cell")
	return *cell.Money
}

func TestBuildCommissionsReportGroupsBySellerWithSubtotals(t *testing.T) {
	sellerA := uint(7)
	sellerB := uint(8)
	paidAt := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)

	contracts := []models.Contract{
		commissionsTestContract(101, &sellerA, "Ana Vendedora", "Cliente Uno", "Lote 1", "Proyecto Norte",
			100000, 4000, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), nil),
		commissionsTestContract(102, &sellerA, "Ana Vendedora", "Cliente Dos", "Lote 2", "Proyecto Norte",
			200000, 8000, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), &paidAt),
		commissionsTestContract(103, &sellerB, "Carlos Vendedor", "Cliente Tres", "Lote 3", "Proyecto Sur",
			50000, 1500, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), nil),
	}

	var captured *repository.ContractQuery
	contractRepo := commissionsTestContractRepo(contracts, &captured)

	historicalRepo := newFakeHistoricalSaleRepository()
	historicalRepo.sales = []models.HistoricalSale{{
		ID:               900,
		ClientName:       "Cliente Histórico",
		ProjectName:      "Proyecto Histórico",
		SalePrice:        120000,
		SaleDate:         time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		CommissionAmount: 900,
		CommissionStatus: "paid",
		ValidationStatus: models.HistoricalSaleStatusValidated,
		CreatedBy:        sellerA,
		Creator:          &models.User{ID: sellerA, FullName: "Ana Vendedora", Role: models.RoleSeller},
	}}

	reportSvc := NewReportService(nil, contractRepo, nil, nil)
	reportSvc.SetHistoricalSaleRepository(historicalRepo)

	creditRepo := &commissionsTestCreditRepo{pending: map[uint]float64{
		sellerA: 1000,
		sellerB: 500,
		9:       200,
	}}

	exportSvc := NewReportExportService(nil, nil, nil, reportSvc, creditRepo)

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local)
	doc, err := exportSvc.BuildCommissionsReport(context.Background(), ReportExportFilters{
		StartDate: &start,
		EndDate:   &end,
	})
	require.NoError(t, err)
	require.NotNil(t, doc)

	assert.Equal(t, ReportTitleCommissions, doc.Title)
	assert.Equal(t, ReportBaseCommissions, doc.BaseName)
	assert.Equal(t, "HNL", doc.Currency)
	assert.Equal(t, "Octubre 2026", doc.Period)
	assert.Equal(t, "2026-10", doc.PeriodKey)
	assert.False(t, doc.GeneratedAt.IsZero())

	require.NotNil(t, captured)
	assert.Equal(t, "2026-10-01", captured.Filters["commission_from"])
	assert.Equal(t, "2026-10-31", captured.Filters["commission_to"])

	// Two seller detail tables, two seller summaries, the grand total and the
	// credits section, in that order.
	require.Len(t, doc.Sections, 6)
	assert.Equal(t, "Comisiones de Ana Vendedora", doc.Sections[0].Title)
	assert.Equal(t, "Resumen de Ana Vendedora", doc.Sections[1].Title)
	assert.Equal(t, "Comisiones de Carlos Vendedor", doc.Sections[2].Title)
	assert.Equal(t, "Resumen de Carlos Vendedor", doc.Sections[3].Title)
	assert.Equal(t, "Totales Generales", doc.Sections[4].Title)
	assert.Equal(t, "Créditos y Neto Pendiente", doc.Sections[5].Title)

	ana := commissionsTestSection(t, doc, "Comisiones de Ana Vendedora")
	assert.Equal(t, []string{
		"Contrato", "Cliente", "Lote", "Proyecto", "Tipo de Financiamiento",
		"Valor del Contrato", "Fecha de Venta", "Origen", "Comisión", "Estado",
	}, ana.Headers)
	require.Len(t, ana.Rows, 3, "contracts plus the historical sale")

	assert.Equal(t, ReportRowPending, ana.Rows[0].Kind)
	assert.Equal(t, "#101", ana.Rows[0].Cells[0].Text)
	assert.Equal(t, "Pendiente", ana.Rows[0].Cells[9].Text)

	assert.Equal(t, ReportRowPaid, ana.Rows[1].Kind)
	assert.Equal(t, "Pagada el 25/10/2026", ana.Rows[1].Cells[9].Text)

	historicalRow := ana.Rows[2]
	assert.Equal(t, ReportRowPaid, historicalRow.Kind)
	assert.Equal(t, "Histórico #900", historicalRow.Cells[0].Text)
	assert.Equal(t, "Histórico", historicalRow.Cells[7].Text)
	assert.Equal(t, "Pagada el 15/10/2026", historicalRow.Cells[9].Text)
	require.NotNil(t, historicalRow.Cells[6].Date, "sale date must be a typed date cell")
	assert.Equal(t, "2026-10-15", FormatReportDateISO(*historicalRow.Cells[6].Date))

	require.NotNil(t, ana.Totals)
	assert.Equal(t, ReportRowSubtotal, ana.Totals.Kind)
	assert.InDelta(t, 420000.0, commissionsTestMoney(t, ana.Totals.Cells[5]), 0.01)
	assert.InDelta(t, 12900.0, commissionsTestMoney(t, ana.Totals.Cells[8]), 0.01)

	anaSummary := commissionsTestSection(t, doc, "Resumen de Ana Vendedora")
	require.Len(t, anaSummary.Rows, 3)
	assert.Equal(t, "Comisión estimada", anaSummary.Rows[0].Cells[0].Text)
	assert.InDelta(t, 12900.0, commissionsTestMoney(t, anaSummary.Rows[0].Cells[1]), 0.01)
	assert.Equal(t, "Comisión pagada", anaSummary.Rows[1].Cells[0].Text)
	assert.InDelta(t, 8900.0, commissionsTestMoney(t, anaSummary.Rows[1].Cells[1]), 0.01)
	assert.Equal(t, "Comisión pendiente", anaSummary.Rows[2].Cells[0].Text)
	assert.InDelta(t, 4000.0, commissionsTestMoney(t, anaSummary.Rows[2].Cells[1]), 0.01)

	grand := commissionsTestSection(t, doc, "Totales Generales")
	require.Len(t, grand.Rows, 3)
	assert.InDelta(t, 14400.0, commissionsTestMoney(t, grand.Rows[0].Cells[1]), 0.01)
	assert.InDelta(t, 8900.0, commissionsTestMoney(t, grand.Rows[1].Cells[1]), 0.01)
	assert.InDelta(t, 5500.0, commissionsTestMoney(t, grand.Rows[2].Cells[1]), 0.01)
	assert.Equal(t, ReportRowTotal, grand.Rows[0].Kind)

	credits := commissionsTestSection(t, doc, "Créditos y Neto Pendiente")
	assert.Equal(t, []string{"Vendedor", "Comisiones Pendientes", "Créditos Pendientes", "Neto Pendiente"}, credits.Headers)
	require.Len(t, credits.Rows, 3, "two sellers with commissions plus a credit-only seller")

	assert.Equal(t, "Ana Vendedora", credits.Rows[0].Cells[0].Text)
	assert.InDelta(t, 4000.0, commissionsTestMoney(t, credits.Rows[0].Cells[1]), 0.01)
	assert.InDelta(t, 1000.0, commissionsTestMoney(t, credits.Rows[0].Cells[2]), 0.01)
	assert.InDelta(t, 3000.0, commissionsTestMoney(t, credits.Rows[0].Cells[3]), 0.01)

	assert.Equal(t, "Carlos Vendedor", credits.Rows[1].Cells[0].Text)
	assert.InDelta(t, 1500.0, commissionsTestMoney(t, credits.Rows[1].Cells[1]), 0.01)
	assert.InDelta(t, 500.0, commissionsTestMoney(t, credits.Rows[1].Cells[2]), 0.01)
	assert.InDelta(t, 1000.0, commissionsTestMoney(t, credits.Rows[1].Cells[3]), 0.01)

	assert.Equal(t, "Vendedor #9", credits.Rows[2].Cells[0].Text)
	assert.InDelta(t, 0.0, commissionsTestMoney(t, credits.Rows[2].Cells[1]), 0.01)
	assert.InDelta(t, 200.0, commissionsTestMoney(t, credits.Rows[2].Cells[2]), 0.01)
	assert.InDelta(t, -200.0, commissionsTestMoney(t, credits.Rows[2].Cells[3]), 0.01)

	require.NotNil(t, credits.Totals)
	assert.Equal(t, ReportRowTotal, credits.Totals.Kind)
	assert.InDelta(t, 5500.0, commissionsTestMoney(t, credits.Totals.Cells[1]), 0.01)
	assert.InDelta(t, 1700.0, commissionsTestMoney(t, credits.Totals.Cells[2]), 0.01)
	assert.InDelta(t, 3800.0, commissionsTestMoney(t, credits.Totals.Cells[3]), 0.01)
}

func TestBuildCommissionsReportSellerScoped(t *testing.T) {
	sellerA := uint(7)
	sellerB := uint(8)

	contracts := []models.Contract{
		commissionsTestContract(101, &sellerA, "Ana Vendedora", "Cliente Uno", "Lote 1", "Proyecto Norte",
			100000, 4000, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), nil),
		commissionsTestContract(103, &sellerB, "Carlos Vendedor", "Cliente Tres", "Lote 3", "Proyecto Sur",
			50000, 1500, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), nil),
	}

	var captured *repository.ContractQuery
	contractRepo := commissionsTestContractRepo(contracts, &captured)
	reportSvc := NewReportService(nil, contractRepo, nil, nil)

	creditRepo := &commissionsTestCreditRepo{pending: map[uint]float64{sellerA: 1000, sellerB: 500}}
	exportSvc := NewReportExportService(nil, nil, nil, reportSvc, creditRepo)

	doc, err := exportSvc.BuildCommissionsReport(context.Background(), ReportExportFilters{
		SellerID:       sellerA,
		FilterBySeller: true,
	})
	require.NoError(t, err)

	require.NotNil(t, captured)
	assert.True(t, captured.FilterBySeller)
	assert.Equal(t, sellerA, captured.UserID)
	assert.Empty(t, captured.Filters["commission_from"], "an unscoped period must not add a date filter")

	for _, section := range doc.Sections {
		assert.NotContains(t, section.Title, "Carlos Vendedor")
	}

	ana := commissionsTestSection(t, doc, "Comisiones de Ana Vendedora")
	require.Len(t, ana.Rows, 1)
	assert.Equal(t, "#101", ana.Rows[0].Cells[0].Text)

	credits := commissionsTestSection(t, doc, "Créditos y Neto Pendiente")
	require.Len(t, credits.Rows, 1, "a seller-scoped export only reports the scoped seller")
	assert.Equal(t, "Ana Vendedora", credits.Rows[0].Cells[0].Text)
	assert.InDelta(t, 1000.0, commissionsTestMoney(t, credits.Rows[0].Cells[2]), 0.01)
	assert.InDelta(t, 3000.0, commissionsTestMoney(t, credits.Rows[0].Cells[3]), 0.01)

	require.NotNil(t, credits.Totals)
	assert.InDelta(t, 1000.0, commissionsTestMoney(t, credits.Totals.Cells[2]), 0.01)
}

func TestBuildCommissionsReportCreditsFallbackOnError(t *testing.T) {
	seller := uint(7)
	contracts := []models.Contract{
		commissionsTestContract(101, &seller, "Ana Vendedora", "Cliente Uno", "Lote 1", "Proyecto Norte",
			100000, 4000, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), nil),
	}

	reportSvc := NewReportService(nil, commissionsTestContractRepo(contracts, nil), nil, nil)
	creditRepo := &commissionsTestCreditRepo{err: errors.New("lookup failed")}
	exportSvc := NewReportExportService(nil, nil, nil, reportSvc, creditRepo)

	doc, err := exportSvc.BuildCommissionsReport(context.Background(), ReportExportFilters{})
	require.NoError(t, err, "a credits lookup failure must not fail the export")

	credits := commissionsTestSection(t, doc, "Créditos y Neto Pendiente")
	require.Len(t, credits.Rows, 1)
	assert.InDelta(t, 0.0, commissionsTestMoney(t, credits.Rows[0].Cells[2]), 0.01)
	assert.InDelta(t, 4000.0, commissionsTestMoney(t, credits.Rows[0].Cells[3]), 0.01,
		"with no credits the net pending equals the pending commission")
	require.NotNil(t, credits.Totals)
	assert.InDelta(t, 4000.0, commissionsTestMoney(t, credits.Totals.Cells[3]), 0.01)
}

func TestBuildCommissionsReportWithoutCreditRepository(t *testing.T) {
	seller := uint(7)
	contracts := []models.Contract{
		commissionsTestContract(101, &seller, "Ana Vendedora", "Cliente Uno", "Lote 1", "Proyecto Norte",
			100000, 4000, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), nil),
	}

	reportSvc := NewReportService(nil, commissionsTestContractRepo(contracts, nil), nil, nil)
	exportSvc := NewReportExportService(nil, nil, nil, reportSvc, nil)

	doc, err := exportSvc.BuildCommissionsReport(context.Background(), ReportExportFilters{})
	require.NoError(t, err)

	credits := commissionsTestSection(t, doc, "Créditos y Neto Pendiente")
	require.Len(t, credits.Rows, 1)
	assert.InDelta(t, 0.0, commissionsTestMoney(t, credits.Rows[0].Cells[2]), 0.01)
	assert.InDelta(t, 4000.0, commissionsTestMoney(t, credits.Rows[0].Cells[3]), 0.01)
}

func TestBuildCommissionsReportGroupsUnknownSellerAsSinVendedor(t *testing.T) {
	amount := 100000.0
	accrual := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	contractRepo := &mockContractRepository{
		mockList: func(context.Context, *repository.ContractQuery) ([]models.Contract, int64, error) {
			return []models.Contract{{
				ID:                    101,
				Status:                models.ContractStatusApproved,
				FinancingType:         models.FinancingTypeDirect,
				Amount:                &amount,
				CommissionAmount:      4000,
				CommissionOrigin:      models.CommissionOriginDirect,
				CommissionAccrualDate: &accrual,
			}}, 1, nil
		},
	}

	reportSvc := NewReportService(nil, contractRepo, nil, nil)
	exportSvc := NewReportExportService(nil, nil, nil, reportSvc, &commissionsTestCreditRepo{})

	doc, err := exportSvc.BuildCommissionsReport(context.Background(), ReportExportFilters{})
	require.NoError(t, err)

	section := commissionsTestSection(t, doc, "Comisiones de Sin vendedor")
	require.Len(t, section.Rows, 1)
	assert.InDelta(t, 4000.0, commissionsTestMoney(t, section.Rows[0].Cells[8]), 0.01)
}

// TestGenerateCommissionsSpansMultipleBatches proves the keyset stream feeds
// every contract to the builder across batch boundaries: a fixture larger than
// the batch size yields exactly one item per contract, in input order.
func TestGenerateCommissionsSpansMultipleBatches(t *testing.T) {
	const total = repository.PaymentScanBatchSize*2 + 7
	accrual := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

	contracts := make([]models.Contract, 0, total)
	for i := 0; i < total; i++ {
		contracts = append(contracts, commissionsTestContract(
			uint(1000+i), nil, "", "Cliente", "Lote", "Proyecto", 1000, 40, accrual, nil,
		))
	}

	reportSvc := NewReportService(nil, commissionsTestContractRepo(contracts, nil), nil, nil)

	items, err := reportSvc.GenerateCommissions(context.Background(), "", "", 0, false)
	require.NoError(t, err)
	require.Len(t, items, total, "every contract across all batches is reported")

	// Order must match the fixture order (created_at DESC surrogate).
	for i := range items {
		assert.Equal(t, uint(1000+i), items[i].ContractID)
	}
}

// TestCommissionsDetailRowMarksCancelledContract checks the export state column
// for cancelled contracts: a paid commission stays visible with an explicit
// note, and an unpaid one is rendered as cancelled without payment.
func TestCommissionsDetailRowMarksCancelledContract(t *testing.T) {
	paidAt := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

	cancelledPaid := commissionsDetailRow(CommissionReportItem{
		ContractID: 1, Commission: 3000, CommissionPaidAt: &paidAt,
		CommissionStatus: "paid", ContractStatus: models.ContractStatusCancelled,
	})
	last := len(cancelledPaid.Cells) - 1
	assert.Equal(t, "Pagada el 20/03/2026 (contrato cancelado)", cancelledPaid.Cells[last].Text)
	assert.Equal(t, ReportRowPaid, cancelledPaid.Kind)

	cancelledUnpaid := commissionsDetailRow(CommissionReportItem{
		ContractID: 2, Commission: 3000,
		CommissionStatus: "pending", ContractStatus: models.ContractStatusCancelled,
	})
	assert.Equal(t, "Cancelada (sin pago)", cancelledUnpaid.Cells[last].Text)
	assert.Equal(t, ReportRowMuted, cancelledUnpaid.Kind)

	active := commissionsDetailRow(CommissionReportItem{
		ContractID: 3, Commission: 3000, CommissionPaidAt: &paidAt,
		CommissionStatus: "paid", ContractStatus: models.ContractStatusApproved,
	})
	assert.Equal(t, "Pagada el 20/03/2026", active.Cells[last].Text)
	assert.Equal(t, ReportRowPaid, active.Kind)
}

func TestCommissionsDetailRowStatesCreditAdjustment(t *testing.T) {
	paidAt := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

	netted := commissionsDetailRow(CommissionReportItem{
		ContractID: 1, Commission: 16800, CommissionPaidAt: &paidAt,
		CommissionStatus: "paid", CreditApplied: 13800,
	})
	last := len(netted.Cells) - 1
	assert.Equal(t, "Pagada el 20/03/2026 · Reajuste por crédito: L 13,800.00 (efectivo L 3,000.00)", netted.Cells[last].Text)
	assert.Equal(t, ReportRowPaid, netted.Kind)

	unnetted := commissionsDetailRow(CommissionReportItem{
		ContractID: 2, Commission: 3000, CommissionPaidAt: &paidAt,
		CommissionStatus: "paid",
	})
	assert.Equal(t, "Pagada el 20/03/2026", unnetted.Cells[last].Text)
}

// TestBuildCommissionsReportIncludesCancelledContracts verifies the export
// keeps cancelled contracts in the per-seller tables so a paid commission
// survives the cancellation.
func TestBuildCommissionsReportIncludesCancelledContracts(t *testing.T) {
	seller := uint(7)
	paidAt := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	accrual := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

	approved := commissionsTestContract(101, &seller, "Ana Vendedora", "Cliente Uno", "Lote 1", "Proyecto Norte",
		100000, 3000, accrual, nil)
	cancelledPaid := commissionsTestContract(102, &seller, "Ana Vendedora", "Cliente Dos", "Lote 2", "Proyecto Norte",
		100000, 3000, accrual, &paidAt)
	cancelledPaid.Status = models.ContractStatusCancelled

	contracts := []models.Contract{approved, cancelledPaid}
	reportSvc := NewReportService(nil, commissionsTestContractRepo(contracts, nil), nil, nil)
	exportSvc := NewReportExportService(nil, nil, nil, reportSvc, nil)

	filters := ReportExportFilters{}
	doc, err := exportSvc.BuildCommissionsReport(context.Background(), filters)
	require.NoError(t, err)

	section := commissionsTestSection(t, doc, "Comisiones de Ana Vendedora")
	require.Len(t, section.Rows, 2)

	cancelledRow := section.Rows[1]
	assert.Equal(t, "Pagada el 25/10/2026 (contrato cancelado)",
		cancelledRow.Cells[len(cancelledRow.Cells)-1].Text)

	summary := commissionsTestSection(t, doc, "Resumen de Ana Vendedora")
	assert.InDelta(t, 6000.0, commissionsTestMoney(t, summary.Rows[0].Cells[1]), 0.01)
	assert.InDelta(t, 3000.0, commissionsTestMoney(t, summary.Rows[1].Cells[1]), 0.01)
	assert.InDelta(t, 3000.0, commissionsTestMoney(t, summary.Rows[2].Cells[1]), 0.01)
}
