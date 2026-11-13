package services

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/sjperalta/fintera-api/internal/models"
)

// ProjectCSVOptions controls how duplicate projects are handled during an
// import. SkipDuplicates wins over UpdateExisting: when both are enabled,
// duplicate rows are reported as skipped and never updated.
type ProjectCSVOptions struct {
	UpdateExisting bool
	SkipDuplicates bool
}

// ProjectImportRowError reports a row that could not be imported.
type ProjectImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ProjectImportResult is the summary returned by the CSV import endpoint.
type ProjectImportResult struct {
	Imported int                     `json:"imported"`
	Updated  int                     `json:"updated"`
	Skipped  int                     `json:"skipped"`
	Errors   []ProjectImportRowError `json:"errors"`
}

// ProjectCSVRecord holds the canonical values of one CSV data row together
// with the columns that were actually present in the file.
type ProjectCSVRecord struct {
	Row     int
	Values  map[string]string
	Present map[string]bool
}

type projectCSVField struct {
	key      string
	required bool
	aliases  []string
}

// Keep this list in sync with src/features/projects/csvImport.js in the UI.
var projectCSVFields = []projectCSVField{
	{key: "name", required: true, aliases: []string{"name", "nombre", "proyecto", "project", "project_name", "nombre_proyecto"}},
	{key: "code", aliases: []string{"code", "codigo", "clave", "project_code", "codigo_proyecto"}},
	{key: "guid", aliases: []string{"guid", "id_publico", "public_id"}},
	{key: "address", required: true, aliases: []string{"address", "direccion", "ubicacion", "location"}},
	{key: "city", aliases: []string{"city", "ciudad", "zona", "zone", "municipio"}},
	{key: "price_per_square_unit", required: true, aliases: []string{"price_per_square_unit", "precio", "precio_m2", "precio_por_m2", "precio_por_metro_cuadrado", "precio_unidad", "precio_por_unidad", "price_per_unit"}},
	{key: "lot_count", aliases: []string{"lot_count", "lotes", "lots", "lots_count", "cantidad_de_lotes", "cantidad_lotes", "numero_de_lotes", "num_lotes", "total_lotes"}},
	{key: "measurement_unit", aliases: []string{"measurement_unit", "unidad", "unidad_medida", "unidad_de_medida"}},
	{key: "project_type", aliases: []string{"project_type", "tipo", "tipo_proyecto", "tipo_de_proyecto"}},
	{key: "delivery_date", aliases: []string{"delivery_date", "entrega", "fecha", "fecha_entrega", "fecha_de_entrega"}},
	{key: "description", aliases: []string{"description", "descripcion", "detalles", "notas"}},
	{key: "interest_rate", aliases: []string{"interest_rate", "tasa_interes", "tasa_de_interes", "interes", "tasa"}},
	{key: "commission_rate", aliases: []string{"commission_rate", "comision", "tasa_comision"}},
	{key: "commission_rate_direct", aliases: []string{"commission_rate_direct", "comision_directa", "comision_directo", "comision_venta_directa"}},
	{key: "commission_rate_bank", aliases: []string{"commission_rate_bank", "comision_banco", "comision_bancaria"}},
	{key: "commission_rate_cash", aliases: []string{"commission_rate_cash", "comision_contado", "comision_efectivo"}},
}

// ProjectCSVCanonicalHeaders is the template header order. Required headers
// must always be present in an uploaded file.
var ProjectCSVCanonicalHeaders = []string{
	"name",
	"code",
	"address",
	"city",
	"price_per_square_unit",
	"lot_count",
	"measurement_unit",
	"project_type",
	"delivery_date",
	"description",
	"interest_rate",
	"commission_rate_direct",
	"commission_rate_bank",
	"commission_rate_cash",
}

var projectCSVRequiredHeaders = []string{"name", "address", "price_per_square_unit"}

var projectCSVHeaderToField = buildProjectCSVHeaderIndex()

func buildProjectCSVHeaderIndex() map[string]string {
	index := make(map[string]string)
	for _, field := range projectCSVFields {
		index[normalizeProjectCSVHeader(field.key)] = field.key
		for _, alias := range field.aliases {
			index[normalizeProjectCSVHeader(alias)] = field.key
		}
	}
	return index
}

func normalizeProjectCSVHeader(header string) string {
	header = strings.TrimSpace(strings.TrimPrefix(header, "\uFEFF"))
	header = strings.ToLower(header)
	header = strings.Map(func(r rune) rune {
		switch r {
		case 'á', 'à', 'ä', 'â':
			return 'a'
		case 'é', 'è', 'ë', 'ê':
			return 'e'
		case 'í', 'ì', 'ï', 'î':
			return 'i'
		case 'ó', 'ò', 'ö', 'ô':
			return 'o'
		case 'ú', 'ù', 'ü', 'û':
			return 'u'
		case 'ñ':
			return 'n'
		}
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, header)
	header = strings.ReplaceAll(header, "-", "_")
	header = strings.Join(strings.Fields(header), "_")
	return header
}

// ProjectCSVTemplate returns a UTF-8 CSV template with every canonical header
// and one example row. The values mirror the safe defaults used by the parser.
func ProjectCSVTemplate() string {
	var buffer bytes.Buffer
	buffer.WriteString("\uFEFF")

	writer := csv.NewWriter(&buffer)
	_ = writer.Write(ProjectCSVCanonicalHeaders)
	_ = writer.Write([]string{
		"Proyecto Ejemplo",
		"EJ-001",
		"Calle 50, Ciudad de Panamá",
		"Ciudad de Panamá",
		"1200",
		"10",
		"m2",
		"residential",
		"2027-06-30",
		"Proyecto de ejemplo; reemplaza o elimina esta fila.",
		"0",
		"0",
		"0",
		"0",
	})
	writer.Flush()
	return buffer.String()
}

// ParseProjectCSV parses a CSV file into canonical records. Request-level
// problems (unreadable file, malformed CSV, missing required headers) are
// returned as errors; row-level problems are reported later by BuildProject.
func ParseProjectCSV(reader io.Reader) ([]ProjectCSVRecord, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true
	csvReader.ReuseRecord = false

	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV inválido: %w", err)
	}
	if len(rows) == 0 {
		return nil, errors.New("el archivo CSV está vacío")
	}

	headerRow := rows[0]
	columnFields := make([]string, len(headerRow))
	presentFields := make(map[string]bool)
	for index, header := range headerRow {
		if index == 0 {
			header = strings.TrimPrefix(header, "\uFEFF")
		}
		key, ok := projectCSVHeaderToField[normalizeProjectCSVHeader(header)]
		if !ok {
			continue
		}
		columnFields[index] = key
		presentFields[key] = true
	}

	missing := make([]string, 0, len(projectCSVRequiredHeaders))
	for _, key := range projectCSVRequiredHeaders {
		if !presentFields[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("faltan columnas requeridas: %s", strings.Join(missing, ", "))
	}

	records := make([]ProjectCSVRecord, 0, len(rows)-1)
	for index, row := range rows[1:] {
		values := make(map[string]string, len(presentFields))
		empty := true
		for column, value := range row {
			if column >= len(columnFields) || columnFields[column] == "" {
				continue
			}
			value = strings.TrimSpace(value)
			if value != "" {
				empty = false
			}
			values[columnFields[column]] = value
		}
		if empty {
			continue
		}
		records = append(records, ProjectCSVRecord{
			Row:     index + 2,
			Values:  values,
			Present: presentFields,
		})
	}
	return records, nil
}

// BuildProject converts a CSV record into a project using safe defaults for
// omitted columns.
func (r ProjectCSVRecord) BuildProject() (*models.Project, error) {
	project := &models.Project{
		Name:               strings.TrimSpace(r.Values["name"]),
		Description:        strings.TrimSpace(r.Values["description"]),
		ProjectType:        defaultString(r.Values["project_type"], "residential"),
		Address:            strings.TrimSpace(r.Values["address"]),
		MeasurementUnit:    models.NormalizeMeasurementUnit(defaultString(r.Values["measurement_unit"], models.MeasurementUnitM2)),
		PricePerSquareUnit: 0,
		InterestRate:       0,
		GUID:               strings.TrimSpace(r.Values["guid"]),
	}

	if project.Name == "" {
		return nil, errors.New("name es obligatorio")
	}
	if project.Address == "" {
		return nil, errors.New("address es obligatorio")
	}
	if value := strings.TrimSpace(r.Values["price_per_square_unit"]); value == "" {
		return nil, errors.New("price_per_square_unit es obligatorio")
	}

	price, err := parseCSVFloat(r.Values["price_per_square_unit"])
	if err != nil {
		return nil, fmt.Errorf("price_per_square_unit debe ser un número")
	}
	if price < 0 {
		return nil, errors.New("price_per_square_unit no puede ser negativo")
	}
	project.PricePerSquareUnit = price

	if value := strings.TrimSpace(r.Values["lot_count"]); value != "" {
		lotCount, err := strconv.Atoi(value)
		if err != nil {
			return nil, errors.New("lot_count debe ser un número entero")
		}
		if lotCount < 0 {
			return nil, errors.New("lot_count no puede ser negativo")
		}
		project.LotCount = lotCount
	}

	switch project.MeasurementUnit {
	case models.MeasurementUnitM2, models.MeasurementUnitFT2, models.MeasurementUnitVara2:
	default:
		return nil, errors.New("measurement_unit debe ser m2, ft2 o vara2")
	}

	if date := strings.TrimSpace(r.Values["delivery_date"]); date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return nil, errors.New("delivery_date debe tener el formato YYYY-MM-DD")
		}
		project.DeliveryDate = &date
	}

	rates := []struct {
		key    string
		target *float64
	}{
		{"interest_rate", &project.InterestRate},
		{"commission_rate", &project.CommissionRate},
		{"commission_rate_direct", &project.CommissionRateDirect},
		{"commission_rate_bank", &project.CommissionRateBank},
		{"commission_rate_cash", &project.CommissionRateCash},
	}
	for _, rate := range rates {
		value := strings.TrimSpace(r.Values[rate.key])
		if value == "" {
			continue
		}
		parsed, err := parseCSVFloat(value)
		if err != nil {
			return nil, fmt.Errorf("%s debe ser un número", rate.key)
		}
		if parsed < 0 {
			return nil, fmt.Errorf("%s no puede ser negativo", rate.key)
		}
		*rate.target = parsed
	}

	if code := strings.TrimSpace(r.Values["code"]); code != "" {
		project.Code = &code
	}
	if city := strings.TrimSpace(r.Values["city"]); city != "" {
		project.City = &city
	}
	return project, nil
}

// MergeInto applies the CSV columns present in the file to an existing project.
// Required fields always overwrite; optional columns only overwrite when the
// header exists, and lot_count is preserved to avoid drifting from real lots.
func (r ProjectCSVRecord) MergeInto(existing *models.Project) (*models.Project, error) {
	incoming, err := r.BuildProject()
	if err != nil {
		return nil, err
	}

	merged := *existing
	merged.Name = incoming.Name
	merged.Address = incoming.Address
	merged.PricePerSquareUnit = incoming.PricePerSquareUnit

	if r.Present["code"] {
		merged.Code = incoming.Code
	}
	if r.Present["city"] {
		merged.City = incoming.City
	}
	if r.Present["description"] {
		merged.Description = incoming.Description
	}
	if r.Present["project_type"] && incoming.ProjectType != "" {
		merged.ProjectType = incoming.ProjectType
	}
	if r.Present["measurement_unit"] {
		merged.MeasurementUnit = incoming.MeasurementUnit
	}
	if r.Present["delivery_date"] {
		merged.DeliveryDate = incoming.DeliveryDate
	}
	if r.Present["interest_rate"] {
		merged.InterestRate = incoming.InterestRate
	}
	if r.Present["commission_rate"] {
		merged.CommissionRate = incoming.CommissionRate
	}
	if r.Present["commission_rate_direct"] {
		merged.CommissionRateDirect = incoming.CommissionRateDirect
	}
	if r.Present["commission_rate_bank"] {
		merged.CommissionRateBank = incoming.CommissionRateBank
	}
	if r.Present["commission_rate_cash"] {
		merged.CommissionRateCash = incoming.CommissionRateCash
	}
	return &merged, nil
}

func defaultString(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func parseCSVFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "%")
	value = strings.TrimSpace(value)
	return strconv.ParseFloat(value, 64)
}
