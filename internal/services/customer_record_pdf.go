package services

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/sjperalta/fintera-api/internal/models"
)

// CustomerRecordPDFData is the prepared payload for the customer record PDF.
type CustomerRecordPDFData struct {
	Reference     string
	IssuedAt      string
	Client        ContractPDFClient
	Property      ContractPDFProperty
	FinancingType string
	Price         string
	BasePrice     string
	OverridePrice string
	HasOverride   bool
	Reserve       string
	DownPayment   string
	Installment   string
	Term          string
	StartDate     string
	EndDate       string
}

// buildCustomerRecordPDF renders the customer record natively with gofpdf.
func buildCustomerRecordPDF(data CustomerRecordPDFData) (*bytes.Buffer, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to initialise pdf: %w", err)
	}

	pdf.SetMargins(reportMarginX, reportMarginTop, reportMarginX)
	pdf.SetAutoPageBreak(true, reportMarginBottom)
	pdf.SetTitle("Registro de Cliente "+data.Reference, false)
	pdf.SetAuthor("FINTERA", false)
	pdf.SetCreator("FINTERA", false)
	pdf.AliasNbPages("{nb}")

	renderer := newReportPDFRenderer(pdf, tr)

	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() > 1 {
			renderer.continuationHeader("Registro de Cliente · " + data.Reference)
		}
	})
	pdf.SetFooterFunc(func() {
		renderer.drawFooter("FINTERA · Registro de cliente generado electrónicamente")
	})

	pdf.AddPage()
	renderer.customerRecordHeader(data)
	renderer.customerRecordSections(data)

	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to render customer record pdf: %w", err)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to write customer record pdf: %w", err)
	}
	return &buf, nil
}

func (r *reportPDFRenderer) customerRecordHeader(data CustomerRecordPDFData) {
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
	pdf.SetFont("Helvetica", "B", 14)
	r.setTextColor(pdfText)
	pdf.CellFormat(rightWidth, 7, r.tr("REGISTRO DE CLIENTE"), "", 0, "R", false, 0, "")

	pdf.SetXY(rightX, 22)
	pdf.SetFont("Helvetica", "", 8.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(rightWidth, 5, r.tr("Contrato "+data.Reference), "", 0, "R", false, 0, "")

	dividerY := 32.0
	r.setDrawColor(pdfBorder)
	pdf.SetLineWidth(0.3)
	pdf.Line(reportMarginX, dividerY, r.pageWidth-reportMarginX, dividerY)

	pdf.SetXY(reportMarginX, dividerY+4)
	pdf.SetFont("Helvetica", "", 8.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(110, 5, r.tr("Fecha de generación: "+data.IssuedAt), "", 0, "L", false, 0, "")

	pdf.SetXY(r.pageWidth-reportMarginX-rightWidth, dividerY+4)
	pdf.CellFormat(rightWidth, 5, r.tr(data.FinancingType), "", 0, "R", false, 0, "")
	pdf.SetY(dividerY + 11)
}

func (r *reportPDFRenderer) customerRecordSections(data CustomerRecordPDFData) {
	clientRows := []pdfKeyValue{
		{Label: "Nombre", Value: data.Client.Name},
		{Label: "Identificación", Value: data.Client.Identity},
		{Label: "Teléfono", Value: data.Client.Phone},
		{Label: "Correo", Value: data.Client.Email},
		{Label: "Dirección", Value: data.Client.Address},
	}
	r.sectionTitle("INFORMACIÓN DEL CLIENTE")
	r.drawGridCardBlock("", clientRows, 2)

	propertyRows := []pdfKeyValue{
		{Label: "Proyecto", Value: data.Property.ProjectName},
		{Label: "No. del lote", Value: data.Property.LotName},
		{Label: "Dimensiones", Value: data.Property.Dimensions},
		{Label: "Medida", Value: data.Property.Area},
		{Label: "Matrícula", Value: data.Property.Registration},
		{Label: "Ubicación", Value: data.Property.LotAddress},
	}
	r.sectionTitle("INFORMACIÓN DEL TERRENO")
	r.drawGridCardBlock("", propertyRows, 3)

	stats := []pdfStat{
		{Label: "PRECIO", Value: data.Price, Highlight: true},
		{Label: "RESERVA", Value: data.Reserve},
		{Label: "PRIMA", Value: data.DownPayment},
		{Label: "CUOTA", Value: data.Installment},
		{Label: "PLAZO", Value: data.Term},
		{Label: "FECHA DE INICIO", Value: data.StartDate},
		{Label: "FECHA DE FINALIZACIÓN", Value: data.EndDate},
		{Label: "FINANCIAMIENTO", Value: data.FinancingType},
	}
	if data.HasOverride {
		stats = append(stats,
			pdfStat{Label: "PRECIO BASE", Value: data.BasePrice},
			pdfStat{Label: "PRECIO PERSONALIZADO", Value: data.OverridePrice},
		)
	}
	r.sectionTitle("RESUMEN FINANCIERO")
	r.drawStatGrid(stats, 4)

	referenceRows := []pdfKeyValue{
		{Label: "Nombre completo", Value: "_______________________"},
		{Label: "No. de identidad", Value: "_______________________"},
		{Label: "Celular", Value: "_______________________"},
		{Label: "Parentesco", Value: "_______________________"},
	}
	r.sectionTitle("INFORMACIÓN DE REFERENCIA")
	r.drawGridCardBlock("", referenceRows, 2)

	r.sectionTitle("FIRMAS")
	r.ensureSpace(40)
	r.pdf.Ln(1)
	y := r.pdf.GetY()
	const lineWidth = 62.0
	leftX := reportMarginX + 6
	rightX := r.pageWidth - reportMarginX - lineWidth - 6
	r.signatureBlock(leftX, y, lineWidth, "FIRMA DEL CLIENTE", data.Client.Name, data.Client.Identity)
	r.signatureBlock(rightX, y, lineWidth, "FIRMA VENDEDOR", reportSignatoryName, reportSignatoryIdentity)
}

// drawGridCardBlock draws a titled card that fills the content width. The
// title band is omitted when title is empty so callers can follow a section
// title without repeating it.
func (r *reportPDFRenderer) drawGridCardBlock(title string, rows []pdfKeyValue, columns int) {
	height := r.gridCardHeight(r.contentWidth, rows, columns)
	if title == "" {
		height -= 9
	}
	r.ensureSpace(height + 6)

	y := r.pdf.GetY()
	if title == "" {
		r.drawGridCard(reportMarginX, y, r.contentWidth, height, "", rows, columns)
	} else {
		r.drawGridCard(reportMarginX, y, r.contentWidth, height, title, rows, columns)
	}
	r.pdf.SetY(y + height + 8)
}

// prepareCustomerRecordPDFData assembles the record payload from a contract.
func (s *ReportService) prepareCustomerRecordPDFData(ctx context.Context, contract *models.Contract) CustomerRecordPDFData {
	base := s.prepareContractPDFData(ctx, contract)

	basePrice := 0.0
	overridePrice := 0.0
	effectivePrice := 0.0
	hasOverride := false
	if contract.Lot.ID != 0 {
		basePrice = contract.Lot.Price
		effectivePrice = contract.Lot.EffectivePrice()
		if contract.Lot.OverridePrice != nil {
			overridePrice = *contract.Lot.OverridePrice
			hasOverride = true
		}
	}

	reserve := floatValue(contract.ReserveAmount)
	downPayment := floatValue(contract.DownPayment)
	installment := 0.0
	var endDate time.Time
	for _, payment := range contract.Payments {
		if payment.PaymentType != models.PaymentTypeInstallment {
			continue
		}
		if installment == 0 {
			installment = payment.Amount
		}
		if payment.DueDate.After(endDate) {
			endDate = payment.DueDate
		}
	}

	endDateText := "—"
	if !endDate.IsZero() {
		endDateText = s.formatDateShort(endDate)
	}

	financingType := "—"
	switch contract.FinancingType {
	case models.FinancingTypeDirect:
		financingType = "Directo"
	case models.FinancingTypeBank:
		financingType = "Bancario"
	case models.FinancingTypeCash:
		financingType = "Contado"
	}

	return CustomerRecordPDFData{
		Reference:     base.Reference,
		IssuedAt:      s.formatDateLong(time.Now()),
		Client:        base.Client,
		Property:      base.Property,
		FinancingType: financingType,
		Price:         s.formatMoneyCurrency(base.Financial.Currency, effectivePrice),
		BasePrice:     s.formatMoneyCurrency(base.Financial.Currency, basePrice),
		OverridePrice: s.formatMoneyCurrency(base.Financial.Currency, overridePrice),
		HasOverride:   hasOverride,
		Reserve:       s.formatMoneyCurrency(base.Financial.Currency, reserve),
		DownPayment:   s.formatMoneyCurrency(base.Financial.Currency, downPayment),
		Installment:   s.formatMoneyCurrency(base.Financial.Currency, installment),
		Term:          base.Financial.PaymentTerm,
		StartDate:     s.formatDateShort(contract.CreatedAt),
		EndDate:       endDateText,
	}
}
