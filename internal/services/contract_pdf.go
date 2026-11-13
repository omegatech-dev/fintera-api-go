package services

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

// ContractPDFClient is the client block shown in the contract PDF.
type ContractPDFClient struct {
	Name     string
	Identity string
	Phone    string
	Email    string
	Address  string
}

// ContractPDFProperty is the lot/project block shown in the contract PDF.
type ContractPDFProperty struct {
	ProjectName     string
	ProjectAddress  string
	LotName         string
	LotAddress      string
	Dimensions      string
	Area            string
	AreaM2          string
	MeasurementUnit string
	Registration    string
	North           string
	South           string
	East            string
	West            string
}

// ContractPDFBankAccount is the project account shown in the payment clauses.
type ContractPDFBankAccount struct {
	BankName      string
	Beneficiary   string
	AccountNumber string
	AccountType   string
	Currency      string
}

// HasAccount reports whether a concrete account is available.
func (b ContractPDFBankAccount) HasAccount() bool {
	return strings.TrimSpace(b.AccountNumber) != ""
}

// ContractPDFFinancial holds the financial summary and clause values.
type ContractPDFFinancial struct {
	Amount           string
	Reserve          string
	DownPayment      string
	Financed         string
	Monthly          string
	TotalPaid        string
	Balance          string
	BalanceLabel     string
	NextPayment      string
	NextPaymentDate  string
	PaymentTerm      string
	FirstPaymentDate string
	LastPaymentDate  string
	MaxPaymentDate   string
	InstallmentCount int
	InterestRate     string
	MoratoryRate     string
	FinancingType    string
	Currency         string
	RawDownPayment   float64
}

// ContractPDFPaymentRow is one row of the payment schedule.
type ContractPDFPaymentRow struct {
	Index     int
	DueDate   string
	Concept   string
	Amount    string
	Status    string
	StatusKey string
}

// ContractPDFData is the fully prepared data used to render the contract PDF.
type ContractPDFData struct {
	Reference         string
	Status            string
	StatusKey         string
	IssuedAt          string
	Brand             string
	LegalName         string
	SignatoryName     string
	SignatoryIdentity string
	Client            ContractPDFClient
	Property          ContractPDFProperty
	Financial         ContractPDFFinancial
	BankAccount       ContractPDFBankAccount
	Payments          []ContractPDFPaymentRow
}

// buildContractPDF renders the contract PDF natively with gofpdf. It has no
// external binary dependency.
func buildContractPDF(data ContractPDFData) (*bytes.Buffer, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to initialise pdf: %w", err)
	}

	pdf.SetMargins(reportMarginX, reportMarginTop, reportMarginX)
	pdf.SetAutoPageBreak(true, reportMarginBottom)
	pdf.SetTitle("Promesa de Compra-Venta "+data.Reference, false)
	pdf.SetAuthor("FINTERA", false)
	pdf.SetCreator("FINTERA", false)
	pdf.AliasNbPages("{nb}")

	renderer := newReportPDFRenderer(pdf, tr)

	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() > 1 {
			renderer.continuationHeader("Promesa de Compra-Venta · " + data.Reference)
		}
	})
	pdf.SetFooterFunc(func() {
		renderer.drawFooter("FINTERA · Documento generado electrónicamente")
	})

	pdf.AddPage()
	renderer.header(data)
	renderer.infoCards(data)
	renderer.financialSummary(data.Financial)
	renderer.schedule(data.Payments)
	renderer.clauses(data)
	renderer.signatures(data)

	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to render contract pdf: %w", err)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to write contract pdf: %w", err)
	}
	return &buf, nil
}

func (r *reportPDFRenderer) header(data ContractPDFData) {
	pdf := r.pdf
	pageWidth := r.pageWidth

	r.setFillColor(pdfGreen)
	pdf.Rect(0, 0, pageWidth, 3, "F")

	pdf.SetXY(reportMarginX, 14)
	pdf.SetFont("Helvetica", "B", 19)
	r.setTextColor(pdfGreen)
	pdf.CellFormat(60, 8, "FINTERA", "", 0, "L", false, 0, "")

	pdf.SetXY(reportMarginX, 22)
	pdf.SetFont("Helvetica", "", 7.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(85, 5, r.tr(data.LegalName), "", 0, "L", false, 0, "")

	rightWidth := 92.0
	rightX := pageWidth - reportMarginX - rightWidth
	pdf.SetXY(rightX, 14)
	pdf.SetFont("Helvetica", "B", 13)
	r.setTextColor(pdfText)
	pdf.CellFormat(rightWidth, 7, r.tr("PROMESA DE COMPRA-VENTA"), "", 0, "R", false, 0, "")

	pdf.SetXY(rightX, 21)
	pdf.SetFont("Helvetica", "", 8.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(rightWidth, 5, r.tr("Contrato "+data.Reference), "", 0, "R", false, 0, "")

	r.statusBadge(data.Status, data.StatusKey, pageWidth-reportMarginX, 28)

	dividerY := 38.0
	r.setDrawColor(pdfBorder)
	pdf.SetLineWidth(0.3)
	pdf.Line(reportMarginX, dividerY, pageWidth-reportMarginX, dividerY)

	pdf.SetXY(reportMarginX, dividerY+4)
	pdf.SetFont("Helvetica", "", 8.5)
	r.setTextColor(pdfMuted)
	pdf.CellFormat(100, 5, r.tr("Fecha de emisión: "+data.IssuedAt), "", 0, "L", false, 0, "")

	meta := fmt.Sprintf("%s · %s", data.Financial.Currency, data.Financial.FinancingType)
	pdf.SetXY(pageWidth-reportMarginX-rightWidth, dividerY+4)
	pdf.CellFormat(rightWidth, 5, r.tr(meta), "", 0, "R", false, 0, "")
	pdf.SetY(dividerY + 12)
}

func (r *reportPDFRenderer) infoCards(data ContractPDFData) {
	clientRows := []pdfKeyValue{
		{Label: "Nombre", Value: data.Client.Name},
		{Label: "Identidad", Value: data.Client.Identity},
		{Label: "Teléfono", Value: data.Client.Phone},
		{Label: "Correo", Value: data.Client.Email},
		{Label: "Dirección", Value: data.Client.Address},
	}
	propertyRows := []pdfKeyValue{
		{Label: "Proyecto", Value: data.Property.ProjectName},
		{Label: "Lote", Value: data.Property.LotName},
		{Label: "Área", Value: data.Property.Area},
		{Label: "Dimensiones", Value: data.Property.Dimensions},
		{Label: "Dirección", Value: data.Property.LotAddress},
		{Label: "Matrícula", Value: data.Property.Registration},
	}

	const gap = 5.0
	cardWidth := (r.contentWidth - gap) / 2
	leftHeight := r.infoCardHeight(cardWidth, clientRows)
	rightHeight := r.infoCardHeight(cardWidth, propertyRows)
	cardHeight := math.Max(leftHeight, rightHeight)

	r.ensureSpace(cardHeight + 6)
	y := r.pdf.GetY()
	r.drawInfoCard(reportMarginX, y, cardWidth, cardHeight, "DATOS DEL CLIENTE", clientRows)
	r.drawInfoCard(reportMarginX+cardWidth+gap, y, cardWidth, cardHeight, "DATOS DE LA PROPIEDAD", propertyRows)
	r.pdf.SetY(y + cardHeight + 8)
}

func (r *reportPDFRenderer) financialSummary(financial ContractPDFFinancial) {
	stats := []pdfStat{
		{Label: "VALOR TOTAL", Value: financial.Amount},
		{Label: "RESERVA", Value: financial.Reserve},
		{Label: "PRIMA", Value: financial.DownPayment},
		{Label: "MONTO FINANCIADO", Value: financial.Financed},
		{Label: "CUOTA MENSUAL", Value: financial.Monthly, Hint: financial.PaymentTerm},
		{Label: "TOTAL PAGADO", Value: financial.TotalPaid},
		{Label: financial.BalanceLabel, Value: financial.Balance, Highlight: true},
		{Label: "PRÓXIMO PAGO", Value: financial.NextPayment, Hint: financial.NextPaymentDate},
	}

	r.drawStatGrid(stats, 4)
}

func (r *reportPDFRenderer) schedule(payments []ContractPDFPaymentRow) {
	if len(payments) == 0 {
		r.pdf.SetFont("Helvetica", "I", 8.5)
		r.setTextColor(pdfMuted)
		r.pdf.MultiCell(r.contentWidth, 5, r.tr("No hay pagos registrados para este contrato."), "", "L", false)
		r.pdf.Ln(4)
		return
	}

	widths := []float64{12, 30, 56, 42, 38}
	headers := []string{"#", "Vencimiento", "Concepto", "Monto", "Estado"}
	aligns := []string{"C", "L", "L", "R", "L"}
	bottomY := reportPageHeight - reportMarginBottom

	r.tableHeader(widths, headers, aligns)
	for index, payment := range payments {
		if r.pdf.GetY()+pdfTableRowHeight > bottomY {
			r.pdf.AddPage()
			r.tableHeader(widths, headers, aligns)
		}

		colors := []pdfRGB{pdfText, pdfText, pdfText, pdfText, pdfStatusColor(payment.StatusKey)}
		r.tableRow(widths, aligns, []string{
			fmt.Sprintf("%d", payment.Index),
			payment.DueDate,
			r.fit(payment.Concept, widths[2]-3),
			payment.Amount,
			payment.Status,
		}, index%2 == 1, colors)
	}
	r.pdf.Ln(7)
}

func (r *reportPDFRenderer) clauses(data ContractPDFData) {
	r.sectionTitle("CLÁUSULAS Y CONDICIONES")

	r.paragraph(fmt.Sprintf(
		"Nosotros: %s, en su condición de vendedor, y %s, mayor de edad, con Número de Documento Nacional de Identificación %s, con domicilio en %s.",
		data.LegalName,
		data.Client.Name,
		data.Client.Identity,
		data.Client.Address,
	))
	r.paragraph(fmt.Sprintf(
		"HACEMOS CONSTAR: Que hemos convenido celebrar el presente CONTRATO PRIVADO DE PROMESA DE COMPRA-VENTA DE UN BIEN INMUEBLE, consistente en la compra por parte de %s de un lote de terreno ubicado en %s, %s.",
		data.Client.Name,
		data.Property.ProjectAddress,
		data.Property.ProjectName,
	))

	description := fmt.Sprintf(
		"El inmueble se describe como %s, con un área de %s y dimensiones de %s.",
		data.Property.LotName,
		data.Property.Area,
		data.Property.Dimensions,
	)
	if data.Property.LotAddress != "" && data.Property.LotAddress != "—" {
		description += " Dirección: " + data.Property.LotAddress + "."
	}
	description += " Matrícula: " + data.Property.Registration + "."
	if boundaries := contractBoundaries(data.Property); boundaries != "" {
		description += " Colindancias: " + boundaries + "."
	}
	r.paragraph(description)

	financial := data.Financial
	r.clause("PRIMERA. DERECHOS, PRECIO, RESERVACIÓN Y PAGO DE PRIMA.",
		fmt.Sprintf(
			"El vendedor manifiesta que es dueño y legítimo poseedor del lote de terreno antes descrito, el cual cuenta con DOMINIO PLENO y está ubicado en %s, teniendo convenido con el cliente dárselo en venta por el precio base de %s, monto que será recibido de la siguiente manera:",
			data.Property.ProjectAddress,
			financial.Amount,
		))
	accountPhrase := bankAccountPhrase(data.BankAccount)
	r.clause("INCISO A) PAGO DE RESERVACIÓN.",
		fmt.Sprintf(
			"En fecha %s se deposita la cantidad de %s %s.",
			data.IssuedAt,
			financial.Reserve,
			accountPhrase,
		))
	if financial.RawDownPayment > 0 {
		r.clause("INCISO B) PAGO/COMPLEMENTO DE PRIMA.",
			fmt.Sprintf(
				"Hasta la fecha %s se deposita la cantidad de %s %s.",
				data.IssuedAt,
				financial.DownPayment,
				accountPhrase,
			))
	}
	if data.Financial.FinancingType == translateFinancingType(models.FinancingTypeDirect) {
		r.clause("INCISO C) MONTO A FINANCIAR.",
			fmt.Sprintf(
				"La cantidad de %s será financiada por medio de %s a un plazo de %s al %s%% de interés. La primera cuota será el %s y la última el %s, haciendo un total de %d pagos conforme al cronograma adjunto. Los pagos se realizarán %s.",
				financial.Financed,
				data.LegalName,
				financial.PaymentTerm,
				financial.InterestRate,
				financial.FirstPaymentDate,
				financial.LastPaymentDate,
				financial.InstallmentCount,
				accountPhrase,
			))
	} else {
		r.clause("INCISO C) PAGO TOTAL.",
			fmt.Sprintf(
				"El saldo restante de %s será cancelado en un único pago a más tardar el %s, depositado %s.",
				financial.Financed,
				financial.MaxPaymentDate,
				accountPhrase,
			))
	}

	r.clause("PRIMERO.",
		fmt.Sprintf(
			"En caso de que %s se atrase con el pago en la fecha acordada, se aplicará una tasa de interés moratorio del %s%% mensual sobre el monto adeudado, a partir del día siguiente.",
			data.Client.Name,
			financial.MoratoryRate,
		))
	r.clause("SEGUNDO.",
		fmt.Sprintf(
			"En caso de que %s falte a dos cuotas consecutivas, se considerarán las siguientes alternativas:",
			data.Client.Name,
		))
	r.clause("UNO. FINANCIAMIENTO BANCARIO.",
		"Se otorgará un plazo de 15 días para que una entidad bancaria financie la cantidad restante del terreno.")
	r.clause("DOS.",
		fmt.Sprintf(
			"Si no se obtiene financiamiento, se dará un plazo adicional de 15 días para que el agente de ventas de %s gestione la venta del derecho adquirido.",
			data.LegalName,
		))
	r.clause("TRES.",
		fmt.Sprintf(
			"En un total de 30 días, %s podrá vender su derecho a un amigo o familiar.",
			data.Client.Name,
		))
	r.clause("CUATRO.",
		fmt.Sprintf(
			"Si se agotan estas opciones, %s perderá el derecho sobre el lote, junto con lo pagado hasta la fecha, sin derecho a devolución.",
			data.Client.Name,
		))
	r.clause("TERCERA. MEJORAS Y ENTREGA.",
		fmt.Sprintf(
			"El vendedor se compromete a entregar el proyecto denominado %s con las mejoras descritas en el expediente del proyecto: parques de recreación equipados, senderos, muro perimetral, calles pavimentadas, casa club, cámaras de vigilancia, parqueo vehicular, seguridad privada 24/7, caseta de control de acceso, agua potable y energía eléctrica.",
			data.Property.ProjectName,
		))
	r.clause("CUARTA.",
		fmt.Sprintf(
			"En caso de incumplimiento de las fechas de entrega, se procederá a la devolución de lo pagado por %s en un plazo de 40 días hábiles, contados a partir de la solicitud de devolución.",
			data.Client.Name,
		))
	r.clause("QUINTA.",
		fmt.Sprintf(
			"Los costos de escrituración correrán por cuenta del cliente, %s, quien podrá escriturar una vez cancelado en su totalidad el valor del terreno.",
			data.Client.Name,
		))
	r.clause("SEXTA.",
		"El vendedor se compromete a entregar toda la documentación necesaria para el proceso de escrituración a favor del cliente. Ambas partes aceptan y se comprometen a cumplir fielmente los términos de esta promesa de compra-venta.")

}

func contractBoundaries(property ContractPDFProperty) string {
	parts := make([]string, 0, 4)
	appendBoundary := func(label, value string) {
		value = strings.TrimSpace(value)
		if value != "" && value != "—" {
			parts = append(parts, label+": "+value)
		}
	}
	appendBoundary("NORTE", property.North)
	appendBoundary("SUR", property.South)
	appendBoundary("ESTE", property.East)
	appendBoundary("OESTE", property.West)
	return strings.Join(parts, "; ")
}

func (r *reportPDFRenderer) signatures(data ContractPDFData) {
	r.ensureSpace(84)
	r.paragraph(fmt.Sprintf(
		"En fe de lo cual, y para los efectos legales correspondientes, se firma el presente documento en la ciudad de Puerto Cortés, Departamento de Cortés, en fecha %s.",
		data.IssuedAt,
	))
	r.sectionTitle("FIRMAS")

	r.pdf.SetFont("Helvetica", "", 8.5)
	r.setTextColor(pdfMuted)
	r.pdf.MultiCell(r.contentWidth, 4.4, r.tr("En señal de aceptación, las partes suscriben el presente documento en la fecha indicada."), "", "L", false)
	r.pdf.Ln(6)

	y := r.pdf.GetY()
	const lineWidth = 62.0
	leftX := reportMarginX + 6
	rightX := r.pageWidth - reportMarginX - lineWidth - 6
	r.signatureBlock(leftX, y, lineWidth, "EL VENDEDOR", data.SignatoryName, data.SignatoryIdentity)
	r.signatureBlock(rightX, y, lineWidth, "EL CLIENTE", data.Client.Name, data.Client.Identity)
}

// prepareContractPDFData assembles the PDF view model from the contract. All
// money/date formatting reuses the report service helpers so the PDF matches
// the statement and API conventions.
func (s *ReportService) prepareContractPDFData(ctx context.Context, contract *models.Contract) ContractPDFData {
	client := ContractPDFClient{
		Name:     "EL CLIENTE",
		Identity: "—",
		Phone:    "No registrado",
		Email:    "No registrado",
		Address:  "—",
	}
	if contract.ApplicantUser.ID != 0 {
		client.Name = firstNonEmpty(contract.ApplicantUser.FullName, client.Name)
		client.Identity = firstNonEmpty(contract.ApplicantUser.Identity, client.Identity)
		client.Phone = firstNonEmpty(contract.ApplicantUser.Phone, client.Phone)
		client.Email = firstNonEmpty(contract.ApplicantUser.Email, client.Email)
		client.Address = firstNonEmpty(pointerValue(contract.ApplicantUser.Address), client.Address)
	}

	property := ContractPDFProperty{
		ProjectName:     "—",
		ProjectAddress:  "—",
		LotName:         "—",
		LotAddress:      "—",
		Dimensions:      "—",
		Area:            "—",
		AreaM2:          "0.00",
		MeasurementUnit: "V2",
		Registration:    "—",
	}
	projectInterestRate := 0.0
	installmentCount := 0

	if contract.Lot.ID != 0 {
		property.LotName = firstNonEmpty(contract.Lot.Name, "—")
		property.LotAddress = firstNonEmpty(pointerValue(contract.Lot.Address), "—")
		property.Registration = firstNonEmpty(pointerValue(contract.Lot.RegistrationNumber), "—")
		property.MeasurementUnit = models.ResolveMeasurementUnit(contract.Lot.MeasurementUnit, contract.Lot.Project.MeasurementUnit)
		property.AreaM2 = fmt.Sprintf("%.2f", contract.Lot.Area())
		property.Area = fmt.Sprintf("%.2f %s", contract.Lot.Area(), property.MeasurementUnit)
		if contract.Lot.Length > 0 && contract.Lot.Width > 0 {
			property.Dimensions = fmt.Sprintf("%.2f m × %.2f m", contract.Lot.Width, contract.Lot.Length)
		}
		property.North = pointerValue(contract.Lot.North)
		property.South = pointerValue(contract.Lot.South)
		property.East = pointerValue(contract.Lot.East)
		property.West = pointerValue(contract.Lot.West)

		if contract.Lot.Project.ID != 0 {
			property.ProjectName = firstNonEmpty(contract.Lot.Project.Name, "—")
			property.ProjectAddress = firstNonEmpty(contract.Lot.Project.Address, "—")
			projectInterestRate = contract.Lot.Project.InterestRate
		}
	}

	amount := floatValue(contract.Amount)
	reserve := floatValue(contract.ReserveAmount)
	downPayment := floatValue(contract.DownPayment)
	financed := amount - reserve - downPayment
	if financed < 0 {
		financed = 0
	}

	currency := firstNonEmpty(contract.Currency, "HNL")
	paymentTerm := "—"
	if contract.PaymentTerm > 0 {
		paymentTerm = fmt.Sprintf("%d meses", contract.PaymentTerm)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	payments := make([]models.Payment, len(contract.Payments))
	copy(payments, contract.Payments)
	sort.SliceStable(payments, func(i, j int) bool {
		if payments[i].DueDate.Equal(payments[j].DueDate) {
			return payments[i].ID < payments[j].ID
		}
		return payments[i].DueDate.Before(payments[j].DueDate)
	})

	totalPaid := 0.0
	nextPayment := (*models.Payment)(nil)
	rows := make([]ContractPDFPaymentRow, 0, len(payments))
	for index, payment := range payments {
		if payment.PaymentType == models.PaymentTypeInstallment {
			installmentCount++
		}
		statusLabel, statusKey := translatePaymentStatus(payment.Status)
		if statusKey == "pending" && payment.DueDate.Before(today) {
			statusLabel = "Vencida"
			statusKey = "overdue"
		}

		if isPaidPaymentStatus(payment.Status) {
			totalPaid += paidAmountOf(payment)
		} else if nextPayment == nil {
			nextPayment = &payments[index]
		}

		rows = append(rows, ContractPDFPaymentRow{
			Index:     index + 1,
			DueDate:   s.formatDateShort(payment.DueDate),
			Concept:   translatePaymentType(payment.PaymentType),
			Amount:    s.formatMoneyCurrency(currency, payment.Amount),
			Status:    statusLabel,
			StatusKey: statusKey,
		})
	}

	firstPaymentDate := "—"
	lastPaymentDate := "—"
	monthlyPayment := 0.0
	for _, payment := range payments {
		if payment.PaymentType != models.PaymentTypeInstallment {
			continue
		}
		if monthlyPayment == 0 {
			monthlyPayment = payment.Amount
			firstPaymentDate = s.formatDateLong(payment.DueDate)
		}
		lastPaymentDate = s.formatDateLong(payment.DueDate)
	}

	maxPaymentDate := "—"
	if contract.MaxPaymentDate != nil {
		maxPaymentDate = s.formatDateLong(*contract.MaxPaymentDate)
	}

	nextPaymentAmount := "—"
	nextPaymentDate := "Sin pagos pendientes"
	if nextPayment != nil {
		nextPaymentAmount = s.formatMoneyCurrency(currency, nextPayment.Amount)
		nextPaymentDate = s.formatDateShort(nextPayment.DueDate)
		if nextPayment.DueDate.Before(today) {
			nextPaymentDate += " · vencida"
		}
	}

	statusLabel, statusKey := translateContractStatus(contract.Status)

	balanceValue := balanceOf(contract)
	balanceLabel := "SALDO PENDIENTE"
	if balanceValue > 0 {
		balanceLabel = "SALDO A FAVOR"
	}

	return ContractPDFData{
		Reference:         shortContractReference(contract),
		Status:            statusLabel,
		StatusKey:         statusKey,
		IssuedAt:          s.formatDateLong(now),
		Brand:             "FINTERA",
		LegalName:         "INVERSIONES FAMA, S.A. DE C.V.",
		SignatoryName:     reportSignatoryName,
		SignatoryIdentity: reportSignatoryIdentity,
		Client:            client,
		Property:          property,
		Financial: ContractPDFFinancial{
			Amount:           s.formatMoneyCurrency(currency, amount),
			Reserve:          s.formatMoneyCurrency(currency, reserve),
			DownPayment:      s.formatMoneyCurrency(currency, downPayment),
			Financed:         s.formatMoneyCurrency(currency, financed),
			Monthly:          s.formatMoneyCurrency(currency, monthlyPayment),
			TotalPaid:        s.formatMoneyCurrency(currency, totalPaid),
			Balance:          s.formatMoneyCurrency(currency, math.Abs(balanceValue)),
			BalanceLabel:     balanceLabel,
			NextPayment:      nextPaymentAmount,
			NextPaymentDate:  nextPaymentDate,
			PaymentTerm:      paymentTerm,
			FirstPaymentDate: firstPaymentDate,
			LastPaymentDate:  lastPaymentDate,
			MaxPaymentDate:   maxPaymentDate,
			InstallmentCount: installmentCount,
			// Contracts do not persist their own interest rate today; the project
			// rate is the single source for the ordinary financing rate. The same
			// project-level rate is used as the moratory rate until a dedicated
			// moratory field exists (business/legal must confirm the distinction).
			InterestRate:   fmt.Sprintf("%.2f", projectInterestRate),
			MoratoryRate:   fmt.Sprintf("%.2f", projectInterestRate),
			FinancingType:  translateFinancingType(contract.FinancingType),
			Currency:       currency,
			RawDownPayment: downPayment,
		},
		BankAccount: s.projectBankAccount(ctx, contract),
		Payments:    rows,
	}
}

// projectBankAccount loads the project's default payment instruction. It falls
// back to the first instruction ordered by position/id, and to an empty account
// when the project has none (the clause then uses generic wording).
func (s *ReportService) projectBankAccount(ctx context.Context, contract *models.Contract) ContractPDFBankAccount {
	projectID := contract.Lot.ProjectID
	if projectID == 0 {
		projectID = contract.Lot.Project.ID
	}
	if projectID == 0 || s.instructionRepo == nil {
		return ContractPDFBankAccount{}
	}

	instructions, err := s.instructionRepo.ListByProject(ctx, projectID)
	if err != nil {
		logger.Error("failed to load project payment instructions",
			"project_id", projectID,
			"contract_id", contract.ID,
			"error", err,
		)
		return ContractPDFBankAccount{}
	}
	if len(instructions) == 0 {
		return ContractPDFBankAccount{}
	}

	// The repository orders by position/id, but keep the selection deterministic
	// for any implementation.
	sort.SliceStable(instructions, func(i, j int) bool {
		if instructions[i].Position == instructions[j].Position {
			return instructions[i].ID < instructions[j].ID
		}
		return instructions[i].Position < instructions[j].Position
	})

	selected := instructions[0]
	for _, instruction := range instructions {
		if instruction.IsDefault {
			selected = instruction
			break
		}
	}

	return ContractPDFBankAccount{
		BankName:      selected.BankName,
		Beneficiary:   selected.BeneficiaryName,
		AccountNumber: selected.AccountNumber,
		AccountType:   bankAccountTypeLabel(selected.AccountType),
		Currency:      selected.Currency,
	}
}

func bankAccountTypeLabel(accountType string) string {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case models.PaymentAccountTypeSavings, "ahorros":
		return "Ahorros"
	case models.PaymentAccountTypeChecking, "cheque":
		return "Cheque"
	default:
		return firstNonEmpty(accountType, "—")
	}
}

func bankAccountPhrase(account ContractPDFBankAccount) string {
	if !account.HasAccount() {
		return "a la cuenta indicada en las instrucciones de pago del proyecto"
	}

	phrase := "a la cuenta " + account.AccountNumber
	details := make([]string, 0, 2)
	if account.AccountType != "" {
		details = append(details, account.AccountType)
	}
	if account.Currency != "" {
		details = append(details, account.Currency)
	}
	if len(details) > 0 {
		phrase += " (" + strings.Join(details, ", ") + ")"
	}
	if account.BankName != "" {
		phrase += " de " + account.BankName
	}
	if account.Beneficiary != "" {
		phrase += " a nombre de " + account.Beneficiary
	}
	return phrase
}
