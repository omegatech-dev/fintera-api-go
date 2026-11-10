package models

import (
	"testing"
)

func TestNormalizeMeasurementUnit(t *testing.T) {
	cases := map[string]string{
		"":        MeasurementUnitM2,
		"m2":      MeasurementUnitM2,
		"m²":      MeasurementUnitM2,
		"ft2":     MeasurementUnitFT2,
		"ft²":     MeasurementUnitFT2,
		"vara2":   MeasurementUnitVara2,
		"varas2":  MeasurementUnitVara2,
		"varas²":  MeasurementUnitVara2,
		"unknown": "unknown",
	}
	for input, want := range cases {
		if got := NormalizeMeasurementUnit(input); got != want {
			t.Errorf("NormalizeMeasurementUnit(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveMeasurementUnit(t *testing.T) {
	ft2 := "ft2"
	if got := ResolveMeasurementUnit(&ft2, "m2"); got != MeasurementUnitFT2 {
		t.Errorf("lot unit should win, got %q", got)
	}
	if got := ResolveMeasurementUnit(nil, "vara2"); got != MeasurementUnitVara2 {
		t.Errorf("project unit fallback = %q, want vara2", got)
	}
	empty := ""
	if got := ResolveMeasurementUnit(&empty, "ft2"); got != MeasurementUnitFT2 {
		t.Errorf("empty lot unit should use project, got %q", got)
	}
}

func TestConvertLength(t *testing.T) {
	if got := ConvertLength(20, MeasurementUnitM2, MeasurementUnitFT2); got != 65.62 {
		t.Errorf("20 m -> ft = %v, want 65.62", got)
	}
	if got := ConvertLength(20, MeasurementUnitM2, MeasurementUnitVara2); got != 23.92 {
		t.Errorf("20 m -> varas = %v, want 23.92", got)
	}
	if got := ConvertLength(20, MeasurementUnitFT2, MeasurementUnitM2); got != 6.1 {
		t.Errorf("20 ft -> m = %v, want 6.1", got)
	}
	if got := ConvertLength(20, MeasurementUnitM2, MeasurementUnitM2); got != 20 {
		t.Errorf("same unit = %v, want 20", got)
	}
}

func TestConvertArea(t *testing.T) {
	if got := ConvertArea(200, MeasurementUnitM2, MeasurementUnitFT2); got != 2152.78 {
		t.Errorf("200 m2 -> ft2 = %v, want 2152.78", got)
	}
	if got := ConvertArea(200, MeasurementUnitM2, MeasurementUnitVara2); got != 286.2 {
		t.Errorf("200 m2 -> vara2 = %v, want 286.2", got)
	}
}

func TestConvertPricePerArea(t *testing.T) {
	if got := ConvertPricePerArea(1000, MeasurementUnitM2, MeasurementUnitFT2); got != 92.9 {
		t.Errorf("1000/m2 -> /ft2 = %v, want 92.9", got)
	}
	if got := ConvertPricePerArea(1000, MeasurementUnitM2, MeasurementUnitVara2); got != 698.81 {
		t.Errorf("1000/m2 -> /vara2 = %v, want 698.81", got)
	}
}
