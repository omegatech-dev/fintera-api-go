package models

import "time"

// HistoricalSale is a sale migrated from before the system. It never creates a
// reservation, never touches lots or stock and never affects seller scores.
type HistoricalSale struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	ClientName       string     `gorm:"column:client_name;size:255;not null" json:"client_name"`
	ClientIdentity   *string    `gorm:"column:client_identity;size:255" json:"client_identity"`
	ClientPhone      *string    `gorm:"column:client_phone;size:50" json:"client_phone"`
	ClientEmail      *string    `gorm:"column:client_email;size:255" json:"client_email"`
	ProjectName      string     `gorm:"column:project_name;size:255;not null" json:"project_name"`
	LotLabel         *string    `gorm:"column:lot_label;size:255" json:"lot_label"`
	AreaM2           *float64   `gorm:"column:area_m2;type:decimal(10,2)" json:"area_m2"`
	SalePrice        float64    `gorm:"column:sale_price;type:decimal(15,2);not null" json:"sale_price"`
	SaleDate         time.Time  `gorm:"column:sale_date;type:date;not null" json:"sale_date"`
	PaymentMode      *string    `gorm:"column:payment_mode;size:50" json:"payment_mode"`
	SaleStatus       *string    `gorm:"column:sale_status;size:50" json:"sale_status"`
	InstallmentsPaid int        `gorm:"column:installments_paid;not null;default:0" json:"installments_paid"`
	Rate             *float64   `gorm:"type:decimal(5,2)" json:"rate"`
	CommissionAmount float64    `gorm:"column:commission_amount;type:decimal(15,2);not null;default:0" json:"commission_amount"`
	CommissionStatus string     `gorm:"column:commission_status;size:50;not null;default:pending" json:"commission_status"`
	EvidenceFile     *string    `gorm:"column:evidence_file;size:500" json:"evidence_file"`
	ValidationStatus string     `gorm:"column:validation_status;size:30;not null;default:pending" json:"validation_status"`
	RejectionReason  *string    `gorm:"column:rejection_reason;type:text" json:"rejection_reason"`
	CreatedBy        uint       `gorm:"column:created_by;not null;index" json:"created_by"`
	ValidatedBy      *uint      `gorm:"column:validated_by" json:"validated_by"`
	ValidatedAt      *time.Time `gorm:"column:validated_at" json:"validated_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	Creator   *User `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Validator *User `gorm:"foreignKey:ValidatedBy" json:"validator,omitempty"`
}

// TableName specifies the table name for HistoricalSale.
func (HistoricalSale) TableName() string {
	return "historical_sales"
}

// Historical sale validation statuses.
const (
	HistoricalSaleStatusPending   = "pending"
	HistoricalSaleStatusValidated = "validated"
	HistoricalSaleStatusRejected  = "rejected"
)

// Historical sale payment modes.
const (
	HistoricalPaymentModeCash   = "cash"
	HistoricalPaymentModeBank   = "bank"
	HistoricalPaymentModeDirect = "direct"
)

// HistoricalSaleDateLayout is the date-only format used by the API and CSV.
const HistoricalSaleDateLayout = "2006-01-02"

// HistoricalSaleResponse is the JSON representation of a historical sale. It
// exposes both the persisted client_* names and the shorter aliases used by the
// seller UI (full_name, identity, phone, email) so either consumer works.
type HistoricalSaleResponse struct {
	ID               uint     `json:"id"`
	ClientName       string   `json:"client_name"`
	FullName         string   `json:"full_name"`
	ClientIdentity   *string  `json:"client_identity"`
	Identity         *string  `json:"identity"`
	ClientPhone      *string  `json:"client_phone"`
	Phone            *string  `json:"phone"`
	ClientEmail      *string  `json:"client_email"`
	Email            *string  `json:"email"`
	ProjectName      string   `json:"project_name"`
	LotLabel         *string  `json:"lot_label"`
	AreaM2           *float64 `json:"area_m2"`
	SalePrice        float64  `json:"sale_price"`
	SaleDate         string   `json:"sale_date"`
	PaymentMode      *string  `json:"payment_mode"`
	SaleStatus       *string  `json:"sale_status"`
	InstallmentsPaid int      `json:"installments_paid"`
	Rate             *float64 `json:"rate"`
	CommissionAmount float64  `json:"commission_amount"`
	CommissionStatus string   `json:"commission_status"`
	// CommissionOrigin is always "historical"; the accrual date is the sale date.
	CommissionOrigin      string     `json:"commission_origin"`
	CommissionAccrualDate string     `json:"commission_accrual_date"`
	EvidenceFile          *string    `json:"evidence_file"`
	IsHistorical          bool       `json:"is_historical"`
	ValidationStatus      string     `json:"validation_status"`
	RejectionReason       *string    `json:"rejection_reason"`
	CreatedBy             uint       `json:"created_by"`
	CreatorName           string     `json:"creator_name,omitempty"`
	ValidatedBy           *uint      `json:"validated_by"`
	ValidatedByName       string     `json:"validated_by_name,omitempty"`
	ValidatedAt           *time.Time `json:"validated_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// ToResponse converts a HistoricalSale to its API representation.
func (h *HistoricalSale) ToResponse() HistoricalSaleResponse {
	saleDate := ""
	if !h.SaleDate.IsZero() {
		saleDate = h.SaleDate.Format(HistoricalSaleDateLayout)
	}
	creatorName := ""
	if h.Creator != nil {
		creatorName = h.Creator.FullName
	}
	validatorName := ""
	if h.Validator != nil {
		validatorName = h.Validator.FullName
	}
	validationStatus := h.ValidationStatus
	if validationStatus == "" {
		validationStatus = HistoricalSaleStatusPending
	}
	commissionStatus := h.CommissionStatus
	if commissionStatus == "" {
		commissionStatus = "pending"
	}
	return HistoricalSaleResponse{
		ID:                    h.ID,
		ClientName:            h.ClientName,
		FullName:              h.ClientName,
		ClientIdentity:        h.ClientIdentity,
		Identity:              h.ClientIdentity,
		ClientPhone:           h.ClientPhone,
		Phone:                 h.ClientPhone,
		ClientEmail:           h.ClientEmail,
		Email:                 h.ClientEmail,
		ProjectName:           h.ProjectName,
		LotLabel:              h.LotLabel,
		AreaM2:                h.AreaM2,
		SalePrice:             h.SalePrice,
		SaleDate:              saleDate,
		PaymentMode:           h.PaymentMode,
		SaleStatus:            h.SaleStatus,
		InstallmentsPaid:      h.InstallmentsPaid,
		Rate:                  h.Rate,
		CommissionAmount:      h.CommissionAmount,
		CommissionStatus:      commissionStatus,
		CommissionOrigin:      CommissionOriginHistorical,
		CommissionAccrualDate: saleDate,
		EvidenceFile:          h.EvidenceFile,
		IsHistorical:          true,
		ValidationStatus:      validationStatus,
		RejectionReason:       h.RejectionReason,
		CreatedBy:             h.CreatedBy,
		CreatorName:           creatorName,
		ValidatedBy:           h.ValidatedBy,
		ValidatedByName:       validatorName,
		ValidatedAt:           h.ValidatedAt,
		CreatedAt:             h.CreatedAt,
		UpdatedAt:             h.UpdatedAt,
	}
}
