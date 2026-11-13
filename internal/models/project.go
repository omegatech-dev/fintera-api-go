package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Project represents a real estate project
type Project struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	Name                 string    `gorm:"not null" json:"name"`
	Description          string    `gorm:"type:text;not null" json:"description"`
	ProjectType          string    `gorm:"default:residential" json:"project_type"`
	Address              string    `gorm:"not null" json:"address"`
	City                 *string   `gorm:"type:varchar(120)" json:"city"`
	Code                 *string   `gorm:"type:varchar(40)" json:"code"`
	LotCount             int       `gorm:"not null" json:"lot_count"`
	PricePerSquareUnit   float64   `gorm:"type:decimal(10,2);not null" json:"price_per_square_unit"`
	InterestRate         float64   `gorm:"type:decimal(5,2);not null" json:"interest_rate"`
	GUID                 string    `gorm:"column:guid;not null" json:"guid"`
	CommissionRate       float64   `gorm:"type:decimal(5,2);default:0" json:"commission_rate"`
	CommissionRateDirect float64   `gorm:"type:decimal(5,2);default:0" json:"commission_rate_direct"`
	CommissionRateBank   float64   `gorm:"type:decimal(5,2);default:0" json:"commission_rate_bank"`
	CommissionRateCash   float64   `gorm:"type:decimal(5,2);default:0" json:"commission_rate_cash"`
	MeasurementUnit      string    `gorm:"default:m2" json:"measurement_unit"`
	DeliveryDate         *string   `gorm:"type:date" json:"delivery_date"`
	IsPublished          bool      `gorm:"column:is_published;default:true" json:"is_published"`
	IsActive             bool      `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`

	// Associations
	Lots     []Lot            `gorm:"foreignKey:ProjectID" json:"lots,omitempty"`
	Photos   []ProjectPhoto   `gorm:"foreignKey:ProjectID" json:"-"`
	Drawings []ProjectDrawing `gorm:"foreignKey:ProjectID" json:"-"`
}

// TableName specifies the table name for Project
func (Project) TableName() string {
	return "projects"
}

// BeforeCreate assigns a public GUID when the caller did not provide one.
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.GUID == "" {
		p.GUID = uuid.New().String()
	}
	return nil
}

// ProjectResponse is the JSON response format for projects
type ProjectResponse struct {
	ID                   uint            `json:"id"`
	GUID                 string          `json:"guid"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	ProjectType          string          `json:"project_type"`
	Address              string          `json:"address"`
	City                 *string         `json:"city"`
	Code                 *string         `json:"code"`
	LotCount             int             `json:"lot_count"`
	PricePerSquareUnit   float64         `json:"price_per_square_unit"`
	InterestRate         float64         `json:"interest_rate"`
	CommissionRate       float64         `json:"commission_rate"`
	CommissionRateDirect float64         `json:"commission_rate_direct"`
	CommissionRateBank   float64         `json:"commission_rate_bank"`
	CommissionRateCash   float64         `json:"commission_rate_cash"`
	MeasurementUnit      string          `json:"measurement_unit"`
	DeliveryDate         *string         `json:"delivery_date"`
	IsPublished          bool            `json:"is_published"`
	IsActive             bool            `json:"is_active"`
	CoverPhotoURL        *string         `json:"cover_photo_url"`
	Photos               []PhotoResponse `json:"photos"`
	AvailableLots        int             `json:"available_lots"`
	ReservedLots         int             `json:"reserved_lots"`
	SoldLots             int             `json:"sold_lots"`
	CreatedAt            time.Time       `json:"created_at"`
}

// ToResponse converts Project to ProjectResponse
func (p *Project) ToResponse() ProjectResponse {
	var available, reserved, sold int
	for _, lot := range p.Lots {
		switch lot.Status {
		case LotStatusAvailable:
			available++
		case LotStatusReserved:
			reserved++
		case LotStatusFinanced, LotStatusFullyPaid:
			sold++
		}
	}

	return ProjectResponse{
		ID:                   p.ID,
		GUID:                 p.GUID,
		Name:                 p.Name,
		Description:          p.Description,
		ProjectType:          p.ProjectType,
		Address:              p.Address,
		City:                 p.City,
		Code:                 p.Code,
		LotCount:             p.LotCount,
		PricePerSquareUnit:   p.PricePerSquareUnit,
		InterestRate:         p.InterestRate,
		CommissionRate:       p.CommissionRate,
		CommissionRateDirect: p.CommissionRateDirect,
		CommissionRateBank:   p.CommissionRateBank,
		CommissionRateCash:   p.CommissionRateCash,
		MeasurementUnit:      p.MeasurementUnit,
		DeliveryDate:         p.DeliveryDate,
		IsPublished:          p.IsPublished,
		IsActive:             p.IsActive,
		AvailableLots:        available,
		ReservedLots:         reserved,
		SoldLots:             sold,
		CreatedAt:            p.CreatedAt,
	}
}
