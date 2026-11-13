package services

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/sjperalta/fintera-api/internal/models"
)

// RescissionPDFData is the prepared payload for the rescission contract PDF.
type RescissionPDFData struct {
	Reference     string
	IssuedAt      string
	ContractDate  string
	Client        ContractPDFClient
	Property      ContractPDFProperty
	RefundAmount  string
	PenaltyAmount string
	Day           string
	Month         string
	Year          string
}

// buildRescissionPDF renders the rescission contract natively with gofpdf.
func buildRescissionPDF(data RescissionPDFData) (*bytes.Buffer, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to initialise pdf: %w", err)
	}

	pdf.SetMargins(reportMarginX, reportMarginTop, reportMarginX)
	pdf.SetAutoPageBreak(true, reportMarginBottom)
	pdf.SetTitle("Rescisión de Contrato "+data.Reference, false)
	pdf.SetAuthor("FINTERA", false)
	pdf.SetCreator("FINTERA", false)
	pdf.AliasNbPages("{nb}")

	renderer := newReportPDFRenderer(pdf, tr)

	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() > 1 {
			renderer.continuationHeader("Rescisión de Contrato · " + data.Reference)
		}
	})
	pdf.SetFooterFunc(func() {
		renderer.drawFooter("FINTERA · Rescisión de contrato generada electrónicamente")
	})

	pdf.AddPage()
	renderer.rescissionHeader(data)
	renderer.rescissionBody(data)

	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to render rescission pdf: %w", err)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to write rescission pdf: %w", err)
	}
	return &buf, nil
}

func (r *reportPDFRenderer) rescissionHeader(data RescissionPDFData) {
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
	pdf.SetFont("Helvetica", "B", 13)
	r.setTextColor(pdfText)
	pdf.CellFormat(rightWidth, 7, r.tr("RESCISIÓN DE CONTRATO"), "", 0, "R", false, 0, "")

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
	pdf.CellFormat(110, 5, r.tr("Fecha de emisión: "+data.IssuedAt), "", 0, "L", false, 0, "")

	pdf.SetXY(r.pageWidth-reportMarginX-rightWidth, dividerY+4)
	pdf.CellFormat(rightWidth, 5, r.tr("Contrato original: "+data.ContractDate), "", 0, "R", false, 0, "")
	pdf.SetY(dividerY + 11)
}

func (r *reportPDFRenderer) rescissionBody(data RescissionPDFData) {
	r.titleBanner("RESCISIÓN DE DOCUMENTO PRIVADO DE PROMESA DE COMPRA-VENTA DE UN BIEN INMUEBLE")

	r.paragraph("Comparecen personalmente los señores:")
	r.clause("RUBÉN DE JESÚS MENJIVAR AYALA",
		"mayor de edad, soltero, Ingeniero en Producción Industrial, con número de Documento Nacional de Identificación: 0506-1990-01420, domiciliado en Cienaguita, Municipio de Puerto Cortés, Departamento de Cortés.")
	r.clause(data.Client.Name,
		fmt.Sprintf("mayor de edad, hondureño(a), con DNI %s, domiciliado(a) en %s.", data.Client.Identity, firstNonEmpty(data.Client.Address, "—")))
	r.paragraph(fmt.Sprintf(
		"Manifiestan que en fecha %s, convinieron suscribir un CONTRATO PRIVADO DE PROMESA DE VENTA DE UN BIEN INMUEBLE, consistente en el lote %s del proyecto denominado %s, ubicado en %s.",
		data.ContractDate,
		data.Property.LotName,
		data.Property.ProjectName,
		data.Property.ProjectAddress,
	))

	r.paragraph("El lote se describe así:")
	r.clause("AL NORTE:", firstNonEmpty(data.Property.North, "—"))
	r.clause("AL SUR:", firstNonEmpty(data.Property.South, "—"))
	r.clause("AL ESTE:", firstNonEmpty(data.Property.East, "—"))
	r.clause("AL OESTE:", firstNonEmpty(data.Property.West, "—"))

	r.paragraph(fmt.Sprintf(
		"Se acuerda la devolución de %s, sancionando %s por incumplimiento, con depósito en la cuenta _______________________________ de _______________________________ a nombre de %s.",
		data.RefundAmount,
		data.PenaltyAmount,
		data.Client.Name,
	))
	r.paragraph(fmt.Sprintf(
		"En Puerto Cortés, a los %s días del mes de %s del año %s.",
		data.Day,
		data.Month,
		data.Year,
	))

	r.ensureSpace(60)
	r.pdf.Ln(8)
	y := r.pdf.GetY()
	const lineWidth = 62.0
	leftX := reportMarginX + 6
	rightX := r.pageWidth - reportMarginX - lineWidth - 6
	r.signatureBlock(leftX, y, lineWidth, "EL VENDEDOR", reportSignatoryName, reportSignatoryIdentity)
	r.signatureBlock(rightX, y, lineWidth, "EL CLIENTE", data.Client.Name, data.Client.Identity)
}

// prepareRescissionPDFData assembles the rescission payload from a contract.
func (s *ReportService) prepareRescissionPDFData(ctx context.Context, contract *models.Contract, refundAmount, penaltyAmount float64) RescissionPDFData {
	base := s.prepareContractPDFData(ctx, contract)

	contractDate := "—"
	if !contract.CreatedAt.IsZero() {
		contractDate = s.formatDateLong(contract.CreatedAt)
	}

	now := time.Now()
	return RescissionPDFData{
		Reference:     base.Reference,
		IssuedAt:      s.formatDateLong(now),
		ContractDate:  contractDate,
		Client:        base.Client,
		Property:      base.Property,
		RefundAmount:  s.formatMoney(refundAmount),
		PenaltyAmount: s.formatMoney(penaltyAmount),
		Day:           fmt.Sprintf("%d", now.Day()),
		Month:         s.getSpanishMonthFull(now.Month()),
		Year:          fmt.Sprintf("%d", now.Year()),
	}
}
