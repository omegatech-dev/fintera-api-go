package services

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
)

func parseTemplateRecords(t *testing.T) []ProjectCSVRecord {
	t.Helper()
	records, err := ParseProjectCSV(strings.NewReader(ProjectCSVTemplate()))
	if err != nil {
		t.Fatalf("ParseProjectCSV(template): %v", err)
	}
	return records
}

func TestProjectCSVTemplateMatchesParser(t *testing.T) {
	template := ProjectCSVTemplate()
	if !strings.HasPrefix(template, "\uFEFF") {
		t.Fatal("template must start with a UTF-8 BOM")
	}

	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(template, "\uFEFF")))
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("template is not valid CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("template rows = %d, want header + one example row", len(rows))
	}
	if got, want := rows[0], ProjectCSVCanonicalHeaders; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("headers = %v, want %v", got, want)
	}

	records := parseTemplateRecords(t)
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	project, err := records[0].BuildProject()
	if err != nil {
		t.Fatalf("BuildProject(template): %v", err)
	}
	if project.Name != "Proyecto Ejemplo" || project.Code == nil || *project.Code != "EJ-001" {
		t.Fatalf("unexpected template example: %+v", project)
	}
	if project.InterestRate != 0 || project.CommissionRateDirect != 0 ||
		project.CommissionRateBank != 0 || project.CommissionRateCash != 0 {
		t.Fatalf("template rates = %v/%v/%v/%v, want zero",
			project.InterestRate, project.CommissionRateDirect, project.CommissionRateBank, project.CommissionRateCash)
	}
	if project.MeasurementUnit != models.MeasurementUnitM2 || project.ProjectType != "residential" {
		t.Fatalf("template defaults = %q/%q, want m2/residential", project.MeasurementUnit, project.ProjectType)
	}
}

func TestParseProjectCSVHandlesAliasesQuotesAndMultilineFields(t *testing.T) {
	body := "\uFEFFnombre,codigo,direccion,ciudad,precio_m2,lotes,unidad,tipo,fecha_entrega,descripcion,interes,comision_directa,comision_banco,comision_contado\n" +
		"\"Altos, Norte\",ASM-1,\"Calle 50, Ciudad\",Panamá,1500,8,m2,residential,2027-01-31,\"Primera línea\nSegunda línea\",3.5%,4,6,7\n\n"

	records, err := ParseProjectCSV(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseProjectCSV: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].Row != 2 {
		t.Fatalf("row = %d, want 2", records[0].Row)
	}

	project, err := records[0].BuildProject()
	if err != nil {
		t.Fatalf("BuildProject: %v", err)
	}
	if project.Name != "Altos, Norte" || project.Address != "Calle 50, Ciudad" {
		t.Fatalf("quoted fields parsed as %q / %q", project.Name, project.Address)
	}
	if project.Description != "Primera línea\nSegunda línea" {
		t.Fatalf("multiline description = %q", project.Description)
	}
	if project.Code == nil || *project.Code != "ASM-1" || project.City == nil || *project.City != "Panamá" {
		t.Fatalf("code/city = %v/%v", project.Code, project.City)
	}
	if project.LotCount != 8 || project.PricePerSquareUnit != 1500 {
		t.Fatalf("lot_count/price = %d/%v", project.LotCount, project.PricePerSquareUnit)
	}
	if project.InterestRate != 3.5 || project.CommissionRateDirect != 4 ||
		project.CommissionRateBank != 6 || project.CommissionRateCash != 7 {
		t.Fatalf("rates = %v/%v/%v/%v", project.InterestRate, project.CommissionRateDirect, project.CommissionRateBank, project.CommissionRateCash)
	}
	if project.DeliveryDate == nil || *project.DeliveryDate != "2027-01-31" {
		t.Fatalf("delivery date = %v", project.DeliveryDate)
	}
}

func TestParseProjectCSVRequiresRequiredHeaders(t *testing.T) {
	_, err := ParseProjectCSV(strings.NewReader("name,address\nAltos,Calle 1\n"))
	if err == nil {
		t.Fatal("expected missing required column error")
	}
	if !strings.Contains(err.Error(), "price_per_square_unit") {
		t.Fatalf("error = %q, want price_per_square_unit", err)
	}
}

func TestParseProjectCSVRejectsEmptyAndMalformedCSV(t *testing.T) {
	if _, err := ParseProjectCSV(strings.NewReader("")); err == nil {
		t.Fatal("expected empty CSV error")
	}
	if _, err := ParseProjectCSV(strings.NewReader("name,address,price_per_square_unit\n\"unterminated,x,1\n")); err == nil {
		t.Fatal("expected malformed CSV error")
	}
}

func TestProjectCSVRecordBuildProjectValidatesRows(t *testing.T) {
	base := func() ProjectCSVRecord {
		return ProjectCSVRecord{
			Row: 2,
			Values: map[string]string{
				"name":                  "Altos",
				"address":               "Calle 1",
				"price_per_square_unit": "100",
			},
			Present: map[string]bool{"name": true, "address": true, "price_per_square_unit": true},
		}
	}

	tests := []struct {
		name   string
		mutate func(*ProjectCSVRecord)
		want   string
	}{
		{"missing name", func(r *ProjectCSVRecord) { r.Values["name"] = " " }, "name es obligatorio"},
		{"missing address", func(r *ProjectCSVRecord) { r.Values["address"] = "" }, "address es obligatorio"},
		{"missing price", func(r *ProjectCSVRecord) { r.Values["price_per_square_unit"] = "" }, "price_per_square_unit es obligatorio"},
		{"bad price", func(r *ProjectCSVRecord) { r.Values["price_per_square_unit"] = "abc" }, "price_per_square_unit debe ser un número"},
		{"negative price", func(r *ProjectCSVRecord) { r.Values["price_per_square_unit"] = "-1" }, "no puede ser negativo"},
		{"bad lots", func(r *ProjectCSVRecord) { r.Values["lot_count"] = "2.5" }, "lot_count debe ser un número entero"},
		{"bad unit", func(r *ProjectCSVRecord) { r.Values["measurement_unit"] = "acres" }, "measurement_unit debe ser"},
		{"bad date", func(r *ProjectCSVRecord) { r.Values["delivery_date"] = "31/01/2027" }, "YYYY-MM-DD"},
		{"bad rate", func(r *ProjectCSVRecord) { r.Values["interest_rate"] = "high" }, "interest_rate debe ser un número"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := base()
			test.mutate(&record)
			_, err := record.BuildProject()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want contains %q", err, test.want)
			}
		})
	}
}

func TestProjectCSVRecordMergeIntoPreservesAbsentAndLotCount(t *testing.T) {
	existing := &models.Project{
		ID:                 7,
		GUID:               "keep-guid",
		Name:               "Viejo",
		Address:            "Vieja",
		Description:        "Descripción previa",
		Code:               stringPtr("OLD"),
		City:               stringPtr("Tegucigalpa"),
		LotCount:           12,
		PricePerSquareUnit: 100,
		InterestRate:       2,
		MeasurementUnit:    models.MeasurementUnitM2,
		ProjectType:        "residential",
		IsPublished:        true,
		IsActive:           true,
	}
	record := ProjectCSVRecord{
		Row: 2,
		Values: map[string]string{
			"name":                  "Nuevo",
			"address":               "Nueva",
			"price_per_square_unit": "250",
			"interest_rate":         "5",
		},
		Present: map[string]bool{
			"name":                  true,
			"address":               true,
			"price_per_square_unit": true,
			"interest_rate":         true,
		},
	}

	merged, err := record.MergeInto(existing)
	if err != nil {
		t.Fatalf("MergeInto: %v", err)
	}
	if merged.ID != 7 || merged.GUID != "keep-guid" {
		t.Fatalf("identity = %d/%q", merged.ID, merged.GUID)
	}
	if merged.Name != "Nuevo" || merged.Address != "Nueva" || merged.PricePerSquareUnit != 250 || merged.InterestRate != 5 {
		t.Fatalf("required/present fields not merged: %+v", merged)
	}
	if merged.Description != "Descripción previa" || merged.City == nil || *merged.City != "Tegucigalpa" ||
		merged.Code == nil || *merged.Code != "OLD" || merged.ProjectType != "residential" {
		t.Fatalf("absent columns must be preserved: %+v", merged)
	}
	if merged.LotCount != 12 {
		t.Fatalf("lot_count = %d, want existing 12", merged.LotCount)
	}
	if existing.Name != "Viejo" {
		t.Fatal("MergeInto must not mutate the existing project")
	}
}
