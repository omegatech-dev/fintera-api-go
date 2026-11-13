package services

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"
)

// buildBalancePDF renders the account statement natively with gofpdf. It has
// no external binary dependency.
func buildBalancePDF(data BalanceReportData) (*bytes.Buffer, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to initialise pdf: %w", err)
	}

	pdf.SetMargins(reportMarginX, reportMarginTop, reportMarginX)
	pdf.SetAutoPageBreak(true, reportMarginBottom)
	pdf.SetTitle("Estado de Cuenta "+data.Reference, false)
	pdf.SetAuthor("FINTERA", false)
	pdf.SetCreator("FINTERA", false)
	pdf.AliasNbPages("{nb}")

	renderer := newReportPDFRenderer(pdf, tr)

	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() > 1 {
			renderer.continuationHeader("Estado de Cuenta · " + data.Reference)
		}
	})
	pdf.SetFooterFunc(func() {
		renderer.drawFooter("FINTERA · Estado de cuenta generado electrónicamente")
	})

	pdf.AddPage()
	renderer.balanceHeader(data)
	renderer.balanceCustomer(data.Customer)
	renderer.balanceTotals(data.Totals)

	if !data.HasContracts {
		renderer.sectionTitle("CONTRATOS")
		pdf.SetFont("Helvetica", "I", 8.5)
		renderer.setTextColor(pdfMuted)
		pdf.MultiCell(renderer.contentWidth, 5, tr("El cliente no tiene contratos registrados."), "", "L", false)
	} else {
		renderer.sectionTitle("CONTRATOS")
		for index, contract := range data.Contracts {
			renderer.balanceContract(contract)
			if index < len(data.Contracts)-1 {
				renderer.ensureSpace(20)
				renderer.setDrawColor(pdfBorder)
				pdf.SetLineWidth(0.3)
				separatorY := pdf.GetY() + 2
				pdf.Line(reportMarginX, separatorY, renderer.pageWidth-reportMarginX, separatorY)
				pdf.SetY(separatorY + 6)
			}
		}
	}

	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to render balance pdf: %w", err)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to write balance pdf: %w", err)
	}
	return &buf, nil
}

func (r *reportPDFRenderer) balanceHeader(data BalanceReportData) {
	pdf := r.pdf

	r.setFillColor(pdfGreen)
	pdf.Rect(0, 0, r.pageWidth, 3, "F")

	pdf.SetXY(reportMarginX, 14)
	pdf.SetFont("Helvetica", "B", 19)
	r.setTextColor(pdfGreen)
	pdf.CellFormat(60, 8, "FINTERA", "", 0, "L", false, 0, "")

	pdf.SetXY(reportMarginX, 22)
	pdf.SetFont("Helvetica", "", 7.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(85, 5, r.tr(reportLegalName), "", 0, "L", false, 0, "")

	rightWidth := 92.0
	rightX := r.pageWidth - reportMarginX - rightWidth
	pdf.SetXY(rightX, 14)
	pdf.SetFont("Helvetica", "B", 15)
	r.setTextColor(pdfText)
	pdf.CellFormat(rightWidth, 7, r.tr("ESTADO DE CUENTA"), "", 0, "R", false, 0, "")

	pdf.SetXY(rightX, 22)
	pdf.SetFont("Helvetica", "", 8.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(rightWidth, 5, r.tr("Referencia "+data.Reference), "", 0, "R", false, 0, "")

	dividerY := 32.0
	r.setDrawColor(pdfBorder)
	pdf.SetLineWidth(0.3)
	pdf.Line(reportMarginX, dividerY, r.pageWidth-reportMarginX, dividerY)

	pdf.SetXY(reportMarginX, dividerY+4)
	pdf.SetFont("Helvetica", "", 8.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(110, 5, r.tr(fmt.Sprintf("Fecha de emisión: %s · %s", data.Date, data.Time)), "", 0, "L", false, 0, "")

	contractCount := fmt.Sprintf("%d contrato(s)", data.Totals.Contracts)
	pdf.SetXY(r.pageWidth-reportMarginX-rightWidth, dividerY+4)
	pdf.CellFormat(rightWidth, 5, r.tr(contractCount), "", 0, "R", false, 0, "")
	pdf.SetY(dividerY + 11)
}

func (r *reportPDFRenderer) balanceCustomer(customer BalanceCustomerData) {
	rows := []pdfKeyValue{
		{Label: "Nombre", Value: customer.FullName},
		{Label: "Identidad", Value: customer.Identity},
		{Label: "Teléfono", Value: customer.Phone},
		{Label: "Correo", Value: customer.Email},
		{Label: "Dirección", Value: customer.Address},
	}

	const columns = 2
	height := r.gridCardHeight(r.contentWidth, rows, columns)
	r.ensureSpace(height + 6)
	y := r.pdf.GetY()
	r.drawGridCard(reportMarginX, y, r.contentWidth, height, "DATOS DEL CLIENTE", rows, columns)
	r.pdf.SetY(y + height + 8)
}

func (r *reportPDFRenderer) balanceTotals(totals BalanceTotalsData) {
	stats := []pdfStat{
		{Label: "CONTRATOS", Value: fmt.Sprintf("%d", totals.Contracts)},
		{Label: "FINANCIADO", Value: totals.Financed},
		{Label: "PAGADO", Value: totals.Paid},
		{Label: "SALDO", Value: totals.Balance, Highlight: true},
	}
	columns := 4
	if totals.HasOverdue {
		stats = append(stats, pdfStat{Label: "MORA", Value: totals.Overdue, Danger: true})
		columns = 5
	}
	r.drawStatGrid(stats, columns)
}

func (r *reportPDFRenderer) balanceContract(contract BalanceContractData) {
	r.ensureSpace(70)

	headingY := r.pdf.GetY()
	r.pdf.SetXY(reportMarginX, headingY)
	r.pdf.SetFont("Helvetica", "B", 11)
	r.setTextColor(pdfText)
	r.pdf.CellFormat(r.contentWidth-34, 6, r.tr("CONTRATO "+contract.Reference), "", 0, "L", false, 0, "")
	r.statusBadge(contract.Status, contract.StatusKey, r.pageWidth-reportMarginX, headingY+0.2)

	r.pdf.SetXY(reportMarginX, headingY+6.5)
	r.pdf.SetFont("Helvetica", "", 8.5)
	r.setTextColor(pdfMuted)
	subtitle := fmt.Sprintf("%s · %s", contract.FinancingType, contract.PaymentTerm)
	r.pdf.CellFormat(r.contentWidth, 4.5, r.tr(subtitle), "", 0, "L", false, 0, "")
	r.pdf.SetY(headingY + 12)

	propertyRows := []pdfKeyValue{
		{Label: "Proyecto", Value: contract.ProjectName},
		{Label: "Lote", Value: contract.LotName},
		{Label: "Área", Value: contract.LotArea},
		{Label: "Dimensiones", Value: contract.LotDimensions},
		{Label: "Matrícula", Value: contract.LotRegistration},
		{Label: "Ubicación", Value: contract.LotAddress},
	}
	const propertyColumns = 3
	propertyHeight := r.gridCardHeight(r.contentWidth, propertyRows, propertyColumns)
	r.ensureSpace(propertyHeight + 6)
	propertyY := r.pdf.GetY()
	r.drawGridCard(reportMarginX, propertyY, r.contentWidth, propertyHeight, "PROPIEDAD", propertyRows, propertyColumns)
	r.pdf.SetY(propertyY + propertyHeight + 7)

	r.drawStatGrid([]pdfStat{
		{Label: "VALOR TOTAL", Value: contract.Amount},
		{Label: "RESERVA", Value: contract.Reserve},
		{Label: "PRIMA", Value: contract.DownPayment},
		{Label: "FINANCIADO", Value: contract.Financed},
		{Label: "CUOTA MENSUAL", Value: contract.MonthlyPayment},
		{Label: "PAGADO", Value: contract.TotalPaid},
		{Label: "SALDO", Value: contract.Balance, Highlight: true},
		{Label: "PLAZO", Value: contract.PaymentTerm},
	}, 4)

	r.ensureSpace(12 + pdfTableHeaderHeight + pdfTableRowHeight*2)
	r.subSectionTitle("CRONOGRAMA DE PAGOS")
	r.balancePayments(contract.Payments)

	if contract.HasLedger {
		r.ensureSpace(12 + pdfTableHeaderHeight + pdfTableRowHeight*2)
		r.subSectionTitle("OTROS MOVIMIENTOS")
		r.balanceLedger(contract.Ledger)
	}

	r.pdf.Ln(3)
}

func (r *reportPDFRenderer) balancePayments(payments []BalancePaymentData) {
	if len(payments) == 0 {
		r.pdf.SetFont("Helvetica", "I", 8.5)
		r.setTextColor(pdfMuted)
		r.pdf.MultiCell(r.contentWidth, 5, r.tr("Sin pagos registrados."), "", "L", false)
		r.pdf.Ln(4)
		return
	}

	widths := []float64{10, 26, 44, 33, 33, 32}
	headers := []string{"#", "Vencimiento", "Concepto", "Monto", "Pagado", "Estado"}
	aligns := []string{"C", "L", "L", "R", "R", "L"}
	bottomY := reportPageHeight - reportMarginBottom

	r.tableHeader(widths, headers, aligns)
	for index, payment := range payments {
		if r.pdf.GetY()+pdfTableRowHeight > bottomY {
			r.pdf.AddPage()
			r.tableHeader(widths, headers, aligns)
		}
		colors := []pdfRGB{pdfText, pdfText, pdfText, pdfText, pdfText, pdfStatusColor(payment.StatusKey)}
		r.tableRow(widths, aligns, []string{
			fmt.Sprintf("%d", payment.Index),
			payment.DueDate,
			r.fit(payment.Type, widths[2]-3),
			payment.Amount,
			payment.Paid,
			payment.Status,
		}, index%2 == 1, colors)
	}
	r.pdf.Ln(6)
}

func (r *reportPDFRenderer) balanceLedger(ledger []BalanceLedgerData) {
	widths := []float64{26, 92, 30, 30}
	headers := []string{"Fecha", "Concepto", "Tipo", "Monto"}
	aligns := []string{"L", "L", "L", "R"}
	bottomY := reportPageHeight - reportMarginBottom

	r.tableHeader(widths, headers, aligns)
	for index, entry := range ledger {
		if r.pdf.GetY()+pdfTableRowHeight > bottomY {
			r.pdf.AddPage()
			r.tableHeader(widths, headers, aligns)
		}
		colors := []pdfRGB{pdfText, pdfText, pdfText, pdfText}
		r.tableRow(widths, aligns, []string{
			entry.Date,
			r.fit(entry.Description, widths[1]-3),
			entry.EntryType,
			entry.Amount,
		}, index%2 == 1, colors)
	}
	r.pdf.Ln(6)
}
