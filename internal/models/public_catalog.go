package models

import "time"

// PublicProjectStats is the aggregate lot information exposed publicly.
type PublicProjectStats struct {
	TotalLots     int `json:"total_lots"`
	AvailableLots int `json:"available_lots"`
	ReservedLots  int `json:"reserved_lots"`
	FinancedLots  int `json:"financed_lots"`
	PaidLots      int `json:"paid_lots"`
	SoldLots      int `json:"sold_lots"`
}

// PublicProjectResponse is the privacy-safe project payload for the customer
// portal. Admin-only fields (rates, commissions, notes) never appear here.
type PublicProjectResponse struct {
	GUID               string             `json:"guid"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	ProjectType        string             `json:"project_type"`
	Address            string             `json:"address"`
	City               *string            `json:"city"`
	Code               *string            `json:"code"`
	DeliveryDate       *string            `json:"delivery_date"`
	CreatedAt          time.Time          `json:"created_at"`
	PricePerSquareUnit float64            `json:"price_per_square_unit"`
	MeasurementUnit    string             `json:"measurement_unit"`
	PriceFrom          *float64           `json:"price_from"`
	PriceTo            *float64           `json:"price_to"`
	DrawingCount       int                `json:"drawing_count"`
	Stats              PublicProjectStats `json:"stats"`
	Photos             []PhotoResponse    `json:"photos"`
	PhotoCount         int                `json:"photo_count"`
}

// PublicLotBoundaries is the N/S/E/W boundary text exposed for a lot.
type PublicLotBoundaries struct {
	North *string `json:"north"`
	South *string `json:"south"`
	East  *string `json:"east"`
	West  *string `json:"west"`
}

// PublicLotResponse is the privacy-safe lot payload for the customer portal.
type PublicLotResponse struct {
	GUID            string              `json:"guid"`
	Name            string              `json:"name"`
	Status          string              `json:"status"`
	Length          float64             `json:"length"`
	Width           float64             `json:"width"`
	Area            float64             `json:"area"`
	OverrideArea    *float64            `json:"override_area"`
	EffectivePrice  float64             `json:"effective_price"`
	HasSpecialPrice bool                `json:"has_special_price"`
	Boundaries      PublicLotBoundaries `json:"boundaries"`
	MeasurementUnit *string             `json:"measurement_unit"`
	Photos          []PhotoResponse     `json:"photos"`
}
