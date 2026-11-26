package models

import "math"

// MoneyEpsilon is the tolerance used when comparing monetary amounts stored as
// float64. Values at or below this difference are treated as equal.
const MoneyEpsilon = 0.005

// RoundMoney rounds a monetary amount to two decimals, the precision used by
// the NUMERIC(15,2) columns.
func RoundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}
