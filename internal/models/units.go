package models

import (
	"math"
	"strings"
)

// Supported measurement units. The API stores areas in these units and
// dimensions in the corresponding linear unit.
const (
	MeasurementUnitM2    = "m2"
	MeasurementUnitFT2   = "ft2"
	MeasurementUnitVara2 = "vara2"
)

var areaFactors = map[string]float64{
	MeasurementUnitM2:    1,
	MeasurementUnitFT2:   10.7639,
	MeasurementUnitVara2: 1.431,
}

// NormalizeMeasurementUnit maps aliases to the canonical API values.
func NormalizeMeasurementUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", MeasurementUnitM2, "m²":
		return MeasurementUnitM2
	case MeasurementUnitFT2, "ft²":
		return MeasurementUnitFT2
	case MeasurementUnitVara2, "varas2", "varas²":
		return MeasurementUnitVara2
	default:
		return strings.ToLower(strings.TrimSpace(unit))
	}
}

// ResolveMeasurementUnit prefers the lot unit and falls back to the project.
func ResolveMeasurementUnit(lotUnit *string, projectUnit string) string {
	if lotUnit != nil && strings.TrimSpace(*lotUnit) != "" {
		return NormalizeMeasurementUnit(*lotUnit)
	}
	return NormalizeMeasurementUnit(projectUnit)
}

func areaFactor(unit string) (float64, bool) {
	factor, ok := areaFactors[NormalizeMeasurementUnit(unit)]
	return factor, ok
}

// ConvertArea converts an area value between supported measurement units.
func ConvertArea(value float64, from, to string) float64 {
	from = NormalizeMeasurementUnit(from)
	to = NormalizeMeasurementUnit(to)
	if from == to {
		return value
	}
	fromFactor, fromOK := areaFactor(from)
	toFactor, toOK := areaFactor(to)
	if !fromOK || !toOK || fromFactor == 0 {
		return value
	}
	return round2(value / fromFactor * toFactor)
}

// ConvertLength converts a linear dimension between supported units.
func ConvertLength(value float64, from, to string) float64 {
	from = NormalizeMeasurementUnit(from)
	to = NormalizeMeasurementUnit(to)
	if from == to {
		return value
	}
	fromFactor, fromOK := areaFactor(from)
	toFactor, toOK := areaFactor(to)
	if !fromOK || !toOK || fromFactor == 0 {
		return value
	}
	return round2(value / math.Sqrt(fromFactor) * math.Sqrt(toFactor))
}

// ConvertPricePerArea converts a price per square unit so a lot's total price
// stays economically equivalent after a unit change.
func ConvertPricePerArea(value float64, from, to string) float64 {
	from = NormalizeMeasurementUnit(from)
	to = NormalizeMeasurementUnit(to)
	if from == to {
		return value
	}
	fromFactor, fromOK := areaFactor(from)
	toFactor, toOK := areaFactor(to)
	if !fromOK || !toOK || toFactor == 0 {
		return value
	}
	return round2(value * fromFactor / toFactor)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
