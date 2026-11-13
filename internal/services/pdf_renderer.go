package services

import (
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

const (
	reportMarginX      = 16.0
	reportMarginTop    = 14.0
	reportMarginBottom = 16.0
	reportPageHeight   = 297.0

	pdfBodyFontSize      = 9.2
	pdfLineHeight        = 4.6
	pdfTableRowHeight    = 6.6
	pdfTableHeaderHeight = 7.0
)

const reportLegalName = "INVERSIONES FAMA, S.A. DE C.V."

const (
	reportSignatoryName     = "RUBÉN DE JESÚS MENJIVAR AYALA"
	reportSignatoryIdentity = "0506-1990-01420"
)

type pdfRGB struct {
	r, g, b int
}

var (
	pdfGreen     = pdfRGB{0x15, 0x80, 0x3D}
	pdfSoftGreen = pdfRGB{0xDC, 0xFC, 0xE7}
	pdfText      = pdfRGB{0x0F, 0x17, 0x2A}
	pdfMuted     = pdfRGB{0x47, 0x55, 0x69}
	pdfBorder    = pdfRGB{0xE5, 0xE9, 0xF0}
	pdfZebra     = pdfRGB{0xF8, 0xFA, 0xFC}
	pdfDanger    = pdfRGB{0xB9, 0x1C, 0x1C}
	pdfWarning   = pdfRGB{0xB4, 0x53, 0x09}
	pdfWhite     = pdfRGB{0xFF, 0xFF, 0xFF}
	pdfSoftAmber = pdfRGB{0xFE, 0xF3, 0xC7}
	pdfSoftRed   = pdfRGB{0xFE, 0xE2, 0xE2}
	pdfSoftSlate = pdfRGB{0xF1, 0xF5, 0xF9}
)

// reportPDFRenderer centralises the visual language shared by the native
// report PDFs (contract and account statement).
type reportPDFRenderer struct {
	pdf          *gofpdf.Fpdf
	tr           func(string) string
	pageWidth    float64
	contentWidth float64
}

func newReportPDFRenderer(pdf *gofpdf.Fpdf, tr func(string) string) *reportPDFRenderer {
	pageWidth, _ := pdf.GetPageSize()
	return &reportPDFRenderer{
		pdf:          pdf,
		tr:           tr,
		pageWidth:    pageWidth,
		contentWidth: pageWidth - 2*reportMarginX,
	}
}

func (r *reportPDFRenderer) ensureSpace(height float64) {
	if r.pdf.GetY()+height > reportPageHeight-reportMarginBottom {
		r.pdf.AddPage()
	}
}

type pdfKeyValue struct {
	Label string
	Value string
}

type pdfStat struct {
	Label     string
	Value     string
	Hint      string
	Highlight bool
	Danger    bool
}

func (r *reportPDFRenderer) setTextColor(color pdfRGB) {
	r.pdf.SetTextColor(color.r, color.g, color.b)
}

func (r *reportPDFRenderer) setFillColor(color pdfRGB) {
	r.pdf.SetFillColor(color.r, color.g, color.b)
}

func (r *reportPDFRenderer) setDrawColor(color pdfRGB) {
	r.pdf.SetDrawColor(color.r, color.g, color.b)
}

func (r *reportPDFRenderer) wrap(value string, width float64) []string {
	lines := r.pdf.SplitLines([]byte(r.tr(value)), width)
	if len(lines) == 0 {
		return []string{""}
	}
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, string(line))
	}
	return wrapped
}

func (r *reportPDFRenderer) fit(value string, width float64) string {
	if r.pdf.GetStringWidth(r.tr(value)) <= width {
		return value
	}
	const ellipsis = "..."
	runes := []rune(value)
	for len(runes) > 1 && r.pdf.GetStringWidth(r.tr(string(runes)+ellipsis)) > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

func (r *reportPDFRenderer) continuationHeader(title string) {
	pdf := r.pdf

	r.setFillColor(pdfGreen)
	pdf.Rect(0, 0, r.pageWidth, 2, "F")

	pdf.SetXY(reportMarginX, 5)
	pdf.SetFont("Helvetica", "B", 9)
	r.setTextColor(pdfGreen)
	pdf.CellFormat(40, 5, "FINTERA", "", 0, "L", false, 0, "")

	pdf.SetXY(r.pageWidth-reportMarginX-130, 5)
	pdf.SetFont("Helvetica", "", 7.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(130, 5, r.tr(title), "", 0, "R", false, 0, "")

	r.setDrawColor(pdfBorder)
	pdf.SetLineWidth(0.3)
	pdf.Line(reportMarginX, 11, r.pageWidth-reportMarginX, 11)
	pdf.SetXY(reportMarginX, reportMarginTop)
}

func (r *reportPDFRenderer) drawFooter(leftText string) {
	pdf := r.pdf

	pdf.SetY(-12)
	r.setDrawColor(pdfBorder)
	pdf.SetLineWidth(0.3)
	pdf.Line(reportMarginX, reportPageHeight-13, r.pageWidth-reportMarginX, reportPageHeight-13)

	pdf.SetFont("Helvetica", "", 7.5)
	r.setTextColor(pdfMuted)
	pdf.SetXY(reportMarginX, reportPageHeight-11)
	pdf.CellFormat(110, 4, r.tr(leftText), "", 0, "L", false, 0, "")
	pdf.SetXY(r.pageWidth-reportMarginX-90, reportPageHeight-11)
	pdf.CellFormat(90, 4, r.tr(fmt.Sprintf("Página %d de {nb}", pdf.PageNo())), "", 0, "R", false, 0, "")
}

func (r *reportPDFRenderer) sectionTitle(label string) {
	r.ensureSpace(16)
	y := r.pdf.GetY()

	r.setFillColor(pdfGreen)
	r.pdf.Rect(reportMarginX, y+0.8, 2, 4.8, "F")

	r.pdf.SetXY(reportMarginX+5, y)
	r.pdf.SetFont("Helvetica", "B", 10.5)
	r.setTextColor(pdfGreen)
	r.pdf.CellFormat(r.contentWidth-5, 6, r.tr(label), "", 0, "L", false, 0, "")
	r.pdf.SetY(y + 9)
}

func (r *reportPDFRenderer) subSectionTitle(label string) {
	r.ensureSpace(10)
	y := r.pdf.GetY()

	r.pdf.SetXY(reportMarginX, y)
	r.pdf.SetFont("Helvetica", "B", 8.5)
	r.setTextColor(pdfMuted)
	r.pdf.CellFormat(r.contentWidth, 5, r.tr(label), "", 0, "L", false, 0, "")
	r.pdf.SetY(y + 7)
}

func (r *reportPDFRenderer) infoCardHeight(cardWidth float64, rows []pdfKeyValue) float64 {
	innerWidth := cardWidth - 10
	r.pdf.SetFont("Helvetica", "", 9)

	height := 9.0 + 4.0
	for _, row := range rows {
		height += 3.6 + float64(len(r.wrap(row.Value, innerWidth)))*4.3 + 2.2
	}
	return height + 1.5
}

func (r *reportPDFRenderer) drawInfoCard(x, y, width, height float64, title string, rows []pdfKeyValue) {
	pdf := r.pdf

	pdf.SetFillColor(pdfWhite.r, pdfWhite.g, pdfWhite.b)
	pdf.SetDrawColor(pdfBorder.r, pdfBorder.g, pdfBorder.b)
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(x, y, width, height, 2, "1234", "DF")

	r.setFillColor(pdfSoftGreen)
	pdf.RoundedRect(x, y, width, 9, 2, "12", "F")

	pdf.SetXY(x+5, y+2)
	pdf.SetFont("Helvetica", "B", 8.3)
	r.setTextColor(pdfGreen)
	pdf.CellFormat(width-10, 5, r.tr(title), "", 0, "L", false, 0, "")

	innerWidth := width - 10
	cursorY := y + 9 + 3.5
	for _, row := range rows {
		pdf.SetXY(x+5, cursorY)
		pdf.SetFont("Helvetica", "B", 6.5)
		r.setTextColor(pdfMuted)
		pdf.CellFormat(innerWidth, 3.4, r.tr(strings.ToUpper(row.Label)), "", 0, "L", false, 0, "")
		cursorY += 3.6

		pdf.SetFont("Helvetica", "", 9)
		r.setTextColor(pdfText)
		for _, line := range r.wrap(row.Value, innerWidth) {
			pdf.SetXY(x+5, cursorY)
			pdf.CellFormat(innerWidth, 4.3, line, "", 0, "L", false, 0, "")
			cursorY += 4.3
		}
		cursorY += 2.2
	}
}

func (r *reportPDFRenderer) drawStatGrid(stats []pdfStat, columns int) {
	if len(stats) == 0 {
		return
	}
	if columns < 1 {
		columns = 1
	}

	const gap = 3.0
	const cellHeight = 15.0
	cellWidth := (r.contentWidth - float64(columns-1)*gap) / float64(columns)
	rows := (len(stats) + columns - 1) / columns

	r.ensureSpace(float64(rows)*cellHeight + float64(rows-1)*gap + 6)
	startY := r.pdf.GetY()

	for index, stat := range stats {
		column := index % columns
		row := index / columns
		x := reportMarginX + float64(column)*(cellWidth+gap)
		y := startY + float64(row)*(cellHeight+gap)

		fill := pdfZebra
		accent := pdfMuted
		if stat.Highlight {
			fill = pdfSoftGreen
			accent = pdfGreen
		}

		r.setFillColor(fill)
		r.setDrawColor(pdfBorder)
		r.pdf.RoundedRect(x, y, cellWidth, cellHeight, 1.8, "1234", "DF")

		r.pdf.SetXY(x+3, y+2.6)
		r.pdf.SetFont("Helvetica", "B", 6.3)
		r.setTextColor(accent)
		r.pdf.CellFormat(cellWidth-6, 3.2, r.tr(stat.Label), "", 0, "L", false, 0, "")

		r.pdf.SetXY(x+3, y+6.3)
		r.pdf.SetFont("Helvetica", "B", 10)
		valueColor := pdfText
		if stat.Highlight {
			valueColor = pdfGreen
		}
		if stat.Danger {
			valueColor = pdfDanger
		}
		r.setTextColor(valueColor)
		r.pdf.CellFormat(cellWidth-6, 5, r.tr(r.fit(stat.Value, cellWidth-6)), "", 0, "L", false, 0, "")

		if stat.Hint != "" {
			r.pdf.SetXY(x+3, y+11.4)
			r.pdf.SetFont("Helvetica", "", 6.3)
			r.setTextColor(pdfMuted)
			r.pdf.CellFormat(cellWidth-6, 3, r.tr(r.fit(stat.Hint, cellWidth-6)), "", 0, "L", false, 0, "")
		}
	}

	r.pdf.SetY(startY + float64(rows)*cellHeight + float64(rows-1)*gap + 8)
}

func (r *reportPDFRenderer) tableHeader(widths []float64, headers, aligns []string) {
	tableWidth := 0.0
	for _, width := range widths {
		tableWidth += width
	}

	y := r.pdf.GetY()
	r.setFillColor(pdfSoftGreen)
	r.pdf.Rect(reportMarginX, y, tableWidth, pdfTableHeaderHeight, "F")

	r.pdf.SetFont("Helvetica", "B", 7.5)
	r.setTextColor(pdfGreen)
	x := reportMarginX
	for index, header := range headers {
		r.pdf.SetXY(x, y+1.2)
		r.pdf.CellFormat(widths[index], pdfTableHeaderHeight-1.2, r.tr(header), "", 0, aligns[index], false, 0, "")
		x += widths[index]
	}
	r.pdf.SetY(y + pdfTableHeaderHeight)
}

func (r *reportPDFRenderer) tableRow(widths []float64, aligns, cells []string, zebra bool, colors []pdfRGB) {
	tableWidth := 0.0
	for _, width := range widths {
		tableWidth += width
	}

	y := r.pdf.GetY()
	if zebra {
		r.setFillColor(pdfZebra)
		r.pdf.Rect(reportMarginX, y, tableWidth, pdfTableRowHeight, "F")
	}

	r.setDrawColor(pdfBorder)
	r.pdf.SetLineWidth(0.2)
	r.pdf.Line(reportMarginX, y+pdfTableRowHeight, reportMarginX+tableWidth, y+pdfTableRowHeight)

	r.pdf.SetFont("Helvetica", "", 8)
	x := reportMarginX
	for index, cell := range cells {
		color := pdfText
		if index < len(colors) {
			color = colors[index]
		}
		r.setTextColor(color)
		r.pdf.SetXY(x, y)
		r.pdf.CellFormat(widths[index], pdfTableRowHeight, r.tr(cell), "", 0, aligns[index], false, 0, "")
		x += widths[index]
	}
	r.pdf.SetY(y + pdfTableRowHeight)
}

func (r *reportPDFRenderer) statusBadge(label, statusKey string, rightX, y float64) {
	fill, text := pdfBadgeColors(statusKey)

	r.pdf.SetFont("Helvetica", "B", 7.8)
	width := r.pdf.GetStringWidth(r.tr(label)) + 9
	x := rightX - width

	r.setFillColor(fill)
	r.setDrawColor(fill)
	r.pdf.RoundedRect(x, y, width, 6.2, 1.5, "1234", "DF")
	r.setTextColor(text)
	r.pdf.SetXY(x, y+0.6)
	r.pdf.CellFormat(width, 5, r.tr(label), "", 0, "C", false, 0, "")
}

func pdfBadgeColors(statusKey string) (pdfRGB, pdfRGB) {
	switch statusKey {
	case "paid":
		return pdfSoftGreen, pdfGreen
	case "overdue", "danger":
		return pdfSoftRed, pdfDanger
	case "submitted":
		return pdfSoftAmber, pdfWarning
	default:
		return pdfSoftSlate, pdfMuted
	}
}

func pdfStatusColor(statusKey string) pdfRGB {
	switch statusKey {
	case "paid":
		return pdfGreen
	case "overdue", "danger":
		return pdfDanger
	case "submitted":
		return pdfWarning
	default:
		return pdfMuted
	}
}

func (r *reportPDFRenderer) gridCardRowHeights(cardWidth float64, rows []pdfKeyValue, columns int) []float64 {
	if columns < 1 {
		columns = 1
	}
	const gap = 4.0
	innerWidth := cardWidth - 10
	columnWidth := (innerWidth - float64(columns-1)*gap) / float64(columns)

	rowCount := (len(rows) + columns - 1) / columns
	heights := make([]float64, rowCount)

	r.pdf.SetFont("Helvetica", "", 8.5)
	for index, row := range rows {
		height := 3.4 + float64(len(r.wrap(row.Value, columnWidth)))*4.1 + 2.4
		rowIndex := index / columns
		if height > heights[rowIndex] {
			heights[rowIndex] = height
		}
	}
	return heights
}

func (r *reportPDFRenderer) gridCardHeight(cardWidth float64, rows []pdfKeyValue, columns int) float64 {
	height := 13.0
	for _, rowHeight := range r.gridCardRowHeights(cardWidth, rows, columns) {
		height += rowHeight
	}
	return height + 1.5
}

func (r *reportPDFRenderer) drawGridCard(x, y, width, height float64, title string, rows []pdfKeyValue, columns int) {
	if columns < 1 {
		columns = 1
	}
	const gap = 4.0
	innerWidth := width - 10
	columnWidth := (innerWidth - float64(columns-1)*gap) / float64(columns)

	pdf := r.pdf
	pdf.SetFillColor(pdfWhite.r, pdfWhite.g, pdfWhite.b)
	pdf.SetDrawColor(pdfBorder.r, pdfBorder.g, pdfBorder.b)
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(x, y, width, height, 2, "1234", "DF")

	cursorY := y + 4
	if title != "" {
		r.setFillColor(pdfSoftGreen)
		pdf.RoundedRect(x, y, width, 9, 2, "12", "F")

		pdf.SetXY(x+5, y+2)
		pdf.SetFont("Helvetica", "B", 8.3)
		r.setTextColor(pdfGreen)
		pdf.CellFormat(width-10, 5, r.tr(title), "", 0, "L", false, 0, "")
		cursorY = y + 13
	}

	heights := r.gridCardRowHeights(width, rows, columns)
	for rowIndex, rowHeight := range heights {
		for column := 0; column < columns; column++ {
			index := rowIndex*columns + column
			if index >= len(rows) {
				break
			}
			row := rows[index]
			cellX := x + 5 + float64(column)*(columnWidth+gap)

			pdf.SetXY(cellX, cursorY)
			pdf.SetFont("Helvetica", "B", 6.5)
			r.setTextColor(pdfMuted)
			pdf.CellFormat(columnWidth, 3.4, r.tr(strings.ToUpper(row.Label)), "", 0, "L", false, 0, "")

			pdf.SetFont("Helvetica", "", 8.5)
			r.setTextColor(pdfText)
			valueY := cursorY + 3.4
			for _, line := range r.wrap(row.Value, columnWidth) {
				pdf.SetXY(cellX, valueY)
				pdf.CellFormat(columnWidth, 4.1, line, "", 0, "L", false, 0, "")
				valueY += 4.1
			}
		}
		cursorY += rowHeight
	}
}

func (r *reportPDFRenderer) titleBanner(title string) {
	r.ensureSpace(24)

	y := r.pdf.GetY()
	r.pdf.SetFont("Helvetica", "B", 11)
	lines := r.wrap(title, r.contentWidth-12)
	height := 6.0 + float64(len(lines))*5.2

	r.setFillColor(pdfSoftGreen)
	r.setDrawColor(pdfBorder)
	r.pdf.SetLineWidth(0.3)
	r.pdf.RoundedRect(reportMarginX, y, r.contentWidth, height, 2, "1234", "DF")

	r.setTextColor(pdfText)
	cursorY := y + 3
	for _, line := range lines {
		r.pdf.SetXY(reportMarginX+6, cursorY)
		r.pdf.CellFormat(r.contentWidth-12, 5.2, line, "", 0, "C", false, 0, "")
		cursorY += 5.2
	}
	r.pdf.SetY(y + height + 7)
}

func (r *reportPDFRenderer) paragraph(text string) {
	r.ensureSpace(12)
	r.pdf.SetFont("Helvetica", "", pdfBodyFontSize)
	r.setTextColor(pdfText)
	r.pdf.MultiCell(0, pdfLineHeight, r.tr(text), "", "J", false)
	r.pdf.Ln(2.4)
}

func (r *reportPDFRenderer) clause(title, body string) {
	r.ensureSpace(16)
	r.pdf.SetFont("Helvetica", "B", pdfBodyFontSize)
	r.setTextColor(pdfText)
	r.pdf.MultiCell(0, pdfLineHeight, r.tr(title), "", "L", false)
	r.pdf.SetFont("Helvetica", "", pdfBodyFontSize)
	r.pdf.MultiCell(0, pdfLineHeight, r.tr(body), "", "J", false)
	r.pdf.Ln(2.4)
}

func (r *reportPDFRenderer) signatureBlock(x, y, width float64, role, name, identity string) {
	pdf := r.pdf

	pdf.SetXY(x, y)
	pdf.SetFont("Helvetica", "B", 7)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(width, 4, r.tr(role), "", 0, "C", false, 0, "")

	r.setDrawColor(pdfMuted)
	pdf.SetLineWidth(0.3)
	pdf.Line(x, y+18, x+width, y+18)

	pdf.SetXY(x, y+20)
	pdf.SetFont("Helvetica", "B", 8.5)
	r.setTextColor(pdfText)
	pdf.CellFormat(width, 4.5, r.tr(r.fit(name, width)), "", 0, "C", false, 0, "")

	pdf.SetXY(x, y+24.5)
	pdf.SetFont("Helvetica", "", 7.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(width, 4, r.tr("DNI: "+identity), "", 0, "C", false, 0, "")

	pdf.SetXY(x, y+32)
	pdf.SetFont("Helvetica", "", 6.5)
	pdf.CellFormat(width, 4, "HUELLA", "", 0, "C", false, 0, "")
}
