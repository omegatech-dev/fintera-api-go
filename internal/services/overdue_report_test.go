package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overdueTestRepository returns the fixture payments after applying the same
// active-contract rule as the SQL repository: payments of cancelled (or
// otherwise inactive) contracts never reach the report, and the service must
// not re-add them.
type overdueTestRepository struct {
	repository.OverdueReportRepository
	payments []models.Payment
	err      error
	calls    int
	scope    overdueCapturedScope
}

func (r *overdueTestRepository) FindOverdueForActiveContractsDetailed(context.Context) ([]models.Payment, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}

	active := make([]models.Payment, 0, len(r.payments))
	for _, payment := range r.payments {
		switch payment.Contract.Status {
		case models.ContractStatusApproved, models.ContractStatusSigned:
			active = append(active, payment)
		}
	}
	return active, nil
}

// capturedScope records the last seller/project scope the report requested so
// tests can assert the batched stream receives it.
type overdueCapturedScope struct {
	projectID      *uint
	sellerID       uint
	filterBySeller bool
	batchSize      int
	batches        int
}

func (r *overdueTestRepository) StreamOverdueForActiveContractsDetailed(ctx context.Context, projectID *uint, sellerID uint, filterBySeller bool, batchSize int, fn func(batch []models.Payment) error) error {
	payments, err := r.FindOverdueForActiveContractsDetailed(ctx)
	if err != nil {
		return err
	}

	// Mirror the SQL seller/project scoping so fixtures with mixed owners are
	// filtered exactly as the export expects.
	scoped := make([]models.Payment, 0, len(payments))
	for _, payment := range payments {
		if filterBySeller && sellerID > 0 {
			contract := payment.Contract
			sellerMatch := contract.SellerID != nil && *contract.SellerID == sellerID
			creatorMatch := contract.CreatorID != nil && *contract.CreatorID == sellerID
			if !sellerMatch && !creatorMatch {
				continue
			}
		}
		if projectID != nil && payment.Contract.Lot.ProjectID != *projectID {
			continue
		}
		scoped = append(scoped, payment)
	}

	r.scope = overdueCapturedScope{projectID: projectID, sellerID: sellerID, filterBySeller: filterBySeller, batchSize: batchSize}
	if batchSize <= 0 {
		batchSize = len(scoped)
	}
	if batchSize == 0 {
		return nil
	}
	for start := 0; start < len(scoped); start += batchSize {
		end := start + batchSize
		if end > len(scoped) {
			end = len(scoped)
		}
		r.scope.batches++
		if err := fn(scoped[start:end]); err != nil {
			return err
		}
	}
	return nil
}

// overdueTestContract builds an active contract with the fiscal, balance and
// project data used by the overdue detail.
func overdueTestContract(id uint, status, currency string, balance *float64) models.Contract {
	return models.Contract{
		ID:            id,
		Status:        status,
		Currency:      currency,
		Balance:       balance,
		FinancingType: models.FinancingTypeDirect,
		ApplicantUser: models.User{
			ID:       200 + id,
			FullName: "Cliente Contrato",
			Identity: "0801-1990-12345",
			RTN:      "08011990123456",
			Phone:    "9999-9999",
		},
		Lot: models.Lot{ID: id, Name: "Lote", ProjectID: 1, Project: models.Project{ID: 1, Name: "Proyecto"}},
	}
}

// overdueTestPayment builds a pending payment that is days business-calendar
// days past due. The due date is a `date` column, so the fixture stores the
// calendar date the same way the API does: business creation date plus days.
func overdueTestPayment(id uint, contract models.Contract, days int, amount, interest float64) models.Payment {
	value := interest
	business := time.Now().In(models.DeploymentLocation()).AddDate(0, 0, -days)
	due := time.Date(business.Year(), business.Month(), business.Day(), 0, 0, 0, 0, time.UTC)
	return models.Payment{
		ID:             id,
		ContractID:     contract.ID,
		Amount:         amount,
		InterestAmount: &value,
		DueDate:        due,
		Status:         models.PaymentStatusPending,
		PaymentType:    models.PaymentTypeInstallment,
		Contract:       contract,
	}
}

// overdueTestSection returns the section with the given title.
func overdueTestSection(t *testing.T, doc *ReportDocument, title string) ReportSection {
	t.Helper()
	for _, section := range doc.Sections {
		if section.Title == title {
			return section
		}
	}
	t.Fatalf("section %q not found in the overdue document", title)
	return ReportSection{}
}

// overdueTestMoney reads the numeric value of a money cell.
func overdueTestMoney(t *testing.T, cell ReportCell) float64 {
	t.Helper()
	require.NotNil(t, cell.Money, "expected a money cell")
	return *cell.Money
}

// overdueTestInt reads the numeric value of an integer cell.
func overdueTestInt(t *testing.T, cell ReportCell) int {
	t.Helper()
	require.NotNil(t, cell.Int, "expected an integer cell")
	return *cell.Int
}

// overdueTestDocument builds the document for the given fixture payments.
func overdueTestDocument(t *testing.T, payments []models.Payment, filters ReportExportFilters) *ReportDocument {
	t.Helper()
	repo := &overdueTestRepository{payments: payments}
	exportSvc := NewReportExportService(nil, repo, nil, nil, nil)

	doc, err := exportSvc.BuildOverdueReport(context.Background(), filters)
	require.NoError(t, err)
	require.NotNil(t, doc)
	return doc
}

func TestOverdueBucketsByDays(t *testing.T) {
	cases := map[int]string{
		1:  AgingBucket1To30,
		30: AgingBucket1To30,
		31: AgingBucket31To60,
		60: AgingBucket31To60,
		61: AgingBucket61To90,
		90: AgingBucket61To90,
		91: AgingBucket90Plus,
	}

	for days, expected := range cases {
		assert.Equal(t, expected, overdueBucketForDays(days), "days=%d", days)
	}
}

func TestOverdueBucketsBoundaries(t *testing.T) {
	days := []int{1, 30, 31, 60, 61, 90, 91}
	payments := make([]models.Payment, 0, len(days))
	for index, value := range days {
		contract := overdueTestContract(uint(index+1), models.ContractStatusApproved, "HNL", nil)
		// Distinct amounts make every bucket identifiable in the summary.
		payments = append(payments, overdueTestPayment(
			uint(index+1), contract, value,
			float64((index+1)*100), float64((index+1)*10),
		))
	}

	doc := overdueTestDocument(t, payments, ReportExportFilters{})

	assert.Equal(t, ReportTitleOverdue, doc.Title)
	assert.Equal(t, ReportBaseOverdue, doc.BaseName)
	assert.Equal(t, "HNL", doc.Currency)
	period, periodKey := reportPeriodLabel(nil, nil, doc.GeneratedAt)
	assert.Equal(t, period, doc.Period)
	assert.Equal(t, periodKey, doc.PeriodKey)

	summary := overdueTestSection(t, doc, "Resumen por Antigüedad")
	assert.Equal(t, []string{"Tramo de Mora", "Pagos", "Capital", "Interés Acumulado", "Total"}, summary.Headers)
	require.Len(t, summary.Rows, 4, "every bucket is always present")

	expected := []struct {
		label    string
		payments int
		capital  float64
		interest float64
	}{
		{TranslateAgingBucket(AgingBucket1To30), 2, 300, 30},
		{TranslateAgingBucket(AgingBucket31To60), 2, 700, 70},
		{TranslateAgingBucket(AgingBucket61To90), 2, 1100, 110},
		{TranslateAgingBucket(AgingBucket90Plus), 1, 700, 70},
	}
	for index, want := range expected {
		row := summary.Rows[index]
		assert.Equal(t, want.label, row.Cells[0].Text)
		assert.Equal(t, want.payments, overdueTestInt(t, row.Cells[1]))
		assert.InDelta(t, want.capital, overdueTestMoney(t, row.Cells[2]), 0.01)
		assert.InDelta(t, want.interest, overdueTestMoney(t, row.Cells[3]), 0.01)
		assert.InDelta(t, want.capital+want.interest, overdueTestMoney(t, row.Cells[4]), 0.01)
	}

	require.NotNil(t, summary.Totals)
	assert.Equal(t, ReportRowTotal, summary.Totals.Kind)
	assert.Equal(t, 7, overdueTestInt(t, summary.Totals.Cells[1]))
	assert.InDelta(t, 2800.0, overdueTestMoney(t, summary.Totals.Cells[2]), 0.01)
	assert.InDelta(t, 280.0, overdueTestMoney(t, summary.Totals.Cells[3]), 0.01)
	assert.InDelta(t, 3080.0, overdueTestMoney(t, summary.Totals.Cells[4]), 0.01)

	detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
	require.Len(t, detail.Rows, len(days))
	assert.Equal(t, 91, overdueTestInt(t, detail.Rows[0].Cells[6]), "oldest due date first")
	assert.Equal(t, 1, overdueTestInt(t, detail.Rows[len(detail.Rows)-1].Cells[6]))

	// Severity styling grows with the bucket.
	assert.Equal(t, ReportRowDanger, detail.Rows[0].Kind)
	assert.Equal(t, ReportRowWarning, detail.Rows[2].Kind)
	assert.Equal(t, ReportRowNormal, detail.Rows[len(detail.Rows)-1].Kind)
}

func TestOverdueSummaryMatchesDetailTotals(t *testing.T) {
	days := []int{5, 45, 75, 120}

	var payments []models.Payment
	for index, value := range days {
		contract := overdueTestContract(uint(index+1), models.ContractStatusApproved, "HNL", nil)
		payments = append(payments, overdueTestPayment(uint(index+1), contract, value, 1000, 100))
	}

	doc := overdueTestDocument(t, payments, ReportExportFilters{})

	summary := overdueTestSection(t, doc, "Resumen por Antigüedad")
	detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")

	bucketPayments := make(map[string]int)
	bucketCapital := make(map[string]float64)
	bucketInterest := make(map[string]float64)
	for _, row := range detail.Rows {
		bucket := overdueBucketForDays(overdueTestInt(t, row.Cells[6]))
		bucketPayments[bucket]++
		bucketCapital[bucket] += overdueTestMoney(t, row.Cells[8])
		bucketInterest[bucket] += overdueTestMoney(t, row.Cells[9])
	}

	for _, row := range summary.Rows {
		bucketLabel := row.Cells[0].Text
		var bucket string
		for _, key := range overdueBucketOrder {
			if TranslateAgingBucket(key) == bucketLabel {
				bucket = key
				break
			}
		}
		require.NotEmpty(t, bucket)
		assert.Equal(t, bucketPayments[bucket], overdueTestInt(t, row.Cells[1]))
		assert.InDelta(t, bucketCapital[bucket], overdueTestMoney(t, row.Cells[2]), 0.01)
		assert.InDelta(t, bucketInterest[bucket], overdueTestMoney(t, row.Cells[3]), 0.01)
	}

	require.NotNil(t, summary.Totals)
	assert.Equal(t, len(detail.Rows), overdueTestInt(t, summary.Totals.Cells[1]))
	assert.InDelta(t, 4000.0, overdueTestMoney(t, summary.Totals.Cells[2]), 0.01)
	assert.InDelta(t, 400.0, overdueTestMoney(t, summary.Totals.Cells[3]), 0.01)
	assert.InDelta(t, 4400.0, overdueTestMoney(t, summary.Totals.Cells[4]), 0.01)
}

func TestOverdueReportExcludesCancelledContracts(t *testing.T) {
	active := overdueTestContract(1, models.ContractStatusApproved, "HNL", nil)
	cancelled := overdueTestContract(2, models.ContractStatusCancelled, "HNL", nil)

	payments := []models.Payment{
		overdueTestPayment(1, active, 15, 1000, 0),
		overdueTestPayment(2, cancelled, 15, 9000, 900),
	}

	doc := overdueTestDocument(t, payments, ReportExportFilters{})

	detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
	require.Len(t, detail.Rows, 1, "the cancelled contract payment must not be exported")
	assert.Equal(t, "#1", detail.Rows[0].Cells[0].Text)

	summary := overdueTestSection(t, doc, "Resumen por Antigüedad")
	require.NotNil(t, summary.Totals)
	assert.Equal(t, 1, overdueTestInt(t, summary.Totals.Cells[1]))
	assert.InDelta(t, 1000.0, overdueTestMoney(t, summary.Totals.Cells[2]), 0.01)
	assert.InDelta(t, 0.0, overdueTestMoney(t, summary.Totals.Cells[3]), 0.01)

	for _, row := range summary.Rows {
		for _, cell := range row.Cells {
			if cell.Money != nil {
				assert.NotEqual(t, 9000.0, *cell.Money)
			}
		}
	}
}

func TestOverdueReportFiltersBySellerAndProject(t *testing.T) {
	sellerA := uint(7)
	sellerB := uint(8)
	creator := uint(9)

	contractA := overdueTestContract(1, models.ContractStatusApproved, "HNL", nil)
	contractA.SellerID = &sellerA
	contractA.Lot.ProjectID = 1

	contractB := overdueTestContract(2, models.ContractStatusSigned, "HNL", nil)
	contractB.SellerID = &sellerB
	contractB.CreatorID = &creator
	contractB.Lot.ProjectID = 1

	contractC := overdueTestContract(3, models.ContractStatusApproved, "HNL", nil)
	contractC.SellerID = &sellerB
	contractC.Lot.ProjectID = 2

	payments := []models.Payment{
		overdueTestPayment(1, contractA, 10, 1000, 0),
		overdueTestPayment(2, contractB, 20, 2000, 0),
		overdueTestPayment(3, contractC, 30, 3000, 0),
	}

	projectOne := uint(1)
	projectTwo := uint(2)

	t.Run("seller scope matches current seller or creator", func(t *testing.T) {
		doc := overdueTestDocument(t, payments, ReportExportFilters{
			SellerID:       sellerA,
			FilterBySeller: true,
		})
		detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
		require.Len(t, detail.Rows, 1)
		assert.Equal(t, "#1", detail.Rows[0].Cells[0].Text)
	})

	t.Run("creator fallback scopes seller 9", func(t *testing.T) {
		doc := overdueTestDocument(t, payments, ReportExportFilters{
			SellerID:       creator,
			FilterBySeller: true,
		})
		detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
		require.Len(t, detail.Rows, 1)
		assert.Equal(t, "#2", detail.Rows[0].Cells[0].Text)
	})

	t.Run("project scope", func(t *testing.T) {
		doc := overdueTestDocument(t, payments, ReportExportFilters{ProjectID: &projectTwo})
		detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
		require.Len(t, detail.Rows, 1)
		assert.Equal(t, "#3", detail.Rows[0].Cells[0].Text)
	})

	t.Run("seller and project scope combine", func(t *testing.T) {
		doc := overdueTestDocument(t, payments, ReportExportFilters{
			SellerID:       sellerB,
			FilterBySeller: true,
			ProjectID:      &projectOne,
		})
		detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
		require.Len(t, detail.Rows, 1)
		assert.Equal(t, "#2", detail.Rows[0].Cells[0].Text)
	})

	t.Run("no matches yields an empty detail with zero totals", func(t *testing.T) {
		doc := overdueTestDocument(t, payments, ReportExportFilters{
			SellerID:       sellerA,
			FilterBySeller: true,
			ProjectID:      &projectTwo,
		})
		detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
		assert.Empty(t, detail.Rows)
		summary := overdueTestSection(t, doc, "Resumen por Antigüedad")
		require.NotNil(t, summary.Totals)
		assert.Equal(t, 0, overdueTestInt(t, summary.Totals.Cells[1]))
		assert.InDelta(t, 0.0, overdueTestMoney(t, summary.Totals.Cells[4]), 0.01)
	})
}

func TestOverdueReportFiscalDetailAndBankReference(t *testing.T) {
	balance := -15000.0
	contract := overdueTestContract(1, models.ContractStatusApproved, "HNL", &balance)
	contract.ApplicantUser.Identity = ""
	contract.ApplicantUser.RTN = ""
	contract.ApplicantUser.Phone = ""

	payment := overdueTestPayment(1, contract, 10, 1000, 50)

	doc := overdueTestDocument(t, []models.Payment{payment}, ReportExportFilters{})
	detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
	require.Len(t, detail.Rows, 1)

	row := detail.Rows[0]
	assert.Equal(t, "Cliente Contrato", row.Cells[1].Text)
	assert.Equal(t, "", row.Cells[2].Text, "an empty identity must stay empty, never masked")
	assert.Equal(t, "", row.Cells[3].Text, "an empty RTN must stay empty, never masked")
	assert.Equal(t, "", row.Cells[4].Text, "an empty phone must stay empty")
	assert.Equal(t, "", row.Cells[11].Text, "no bank reference renders empty")
	assert.InDelta(t, 15000.0, overdueTestMoney(t, row.Cells[10]), 0.01,
		"the outstanding balance is exported as a positive magnitude")

	authorization := "AUTH-123"
	withAuthorization := overdueTestPayment(2, contract, 10, 1000, 50)
	withAuthorization.AuthorizationNumber = &authorization
	doc = overdueTestDocument(t, []models.Payment{withAuthorization}, ReportExportFilters{})
	row = overdueTestSection(t, doc, "Detalle de Pagos Vencidos").Rows[0]
	assert.Equal(t, "AUTH-123", row.Cells[11].Text)

	payer := "Cuenta de Ana"
	withPayer := overdueTestPayment(3, contract, 10, 1000, 50)
	withPayer.PayerAccountName = &payer
	doc = overdueTestDocument(t, []models.Payment{withPayer}, ReportExportFilters{})
	row = overdueTestSection(t, doc, "Detalle de Pagos Vencidos").Rows[0]
	assert.Equal(t, "Cuenta de Ana", row.Cells[11].Text, "the payer name is the fallback reference")

	positive := 5000.0
	paidContract := overdueTestContract(4, models.ContractStatusApproved, "USD", &positive)
	doc = overdueTestDocument(t, []models.Payment{overdueTestPayment(4, paidContract, 5, 100, 0)}, ReportExportFilters{})
	row = overdueTestSection(t, doc, "Detalle de Pagos Vencidos").Rows[0]
	assert.InDelta(t, 0.0, overdueTestMoney(t, row.Cells[10]), 0.01,
		"a contract without debt reports a zero outstanding balance")
}

func TestOverdueReportIncludesSpanishNoteAndDetailHeaders(t *testing.T) {
	doc := overdueTestDocument(t, nil, ReportExportFilters{})

	detail := overdueTestSection(t, doc, "Detalle de Pagos Vencidos")
	assert.Equal(t, []string{
		"Contrato", "Cliente", "Identidad", "RTN", "Teléfono",
		"Fecha de Vencimiento", "Días de Mora", "Tipo de Pago", "Capital",
		"Interés Acumulado", "Saldo del Contrato", "Referencia Bancaria",
	}, detail.Headers)

	note := overdueTestSection(t, doc, "Notas")
	assert.Equal(t, ReportSectionNote, note.Kind)
	assert.Equal(t, "No se incluyen pagos de contratos cancelados.", note.Note)
}

func TestBuildOverdueReportPropagatesRepositoryError(t *testing.T) {
	repo := &overdueTestRepository{err: errors.New("database unavailable")}
	exportSvc := NewReportExportService(nil, repo, nil, nil, nil)

	doc, err := exportSvc.BuildOverdueReport(context.Background(), ReportExportFilters{})

	require.Error(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, 1, repo.calls)
}
