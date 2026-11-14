package models

import "time"

// Commission origins stored on contracts and exposed by the commission report.
const (
	CommissionOriginReferral   = "referral"
	CommissionOriginDirect     = "direct"
	CommissionOriginHistorical = "historical"
)

// Fallback commission rates used when the settings row has no explicit value.
const (
	DefaultCommissionRate           = 5.0
	DefaultUnassignedCommissionRate = 3.0
)

// CommissionPaymentRule is the only supported payout trigger: the commission is
// committed when the contract is approved.
const CommissionPaymentRule = "contract_approval"

// SellerCommissionRate is a per-seller commission exception. It takes priority
// over the project rate and the global default.
type SellerCommissionRate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SellerID  uint      `gorm:"column:seller_id;not null;uniqueIndex" json:"seller_id"`
	Rate      float64   `gorm:"type:decimal(5,2);not null" json:"rate"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Seller *User `gorm:"foreignKey:SellerID" json:"seller,omitempty"`
}

// TableName specifies the table name for SellerCommissionRate.
func (SellerCommissionRate) TableName() string {
	return "seller_commission_rates"
}

// CommissionSettings is the commission-rate slice of the general_settings
// singleton. Keeping it separate from GeneralSettings lets the commission
// module update its two columns without rewriting the whole settings row.
type CommissionSettings struct {
	ID                              uint      `gorm:"primaryKey" json:"id"`
	CommissionDefaultRate           float64   `gorm:"column:commission_default_rate;type:decimal(5,2);not null;default:5" json:"commission_default_rate"`
	CommissionUnassignedRate        float64   `gorm:"column:commission_unassigned_rate;type:decimal(5,2);not null;default:3" json:"commission_unassigned_rate"`
	CommissionAcceptanceWindowHours int       `gorm:"column:commission_acceptance_window_hours;not null;default:2" json:"commission_acceptance_window_hours"`
	CreatedAt                       time.Time `json:"-"`
	UpdatedAt                       time.Time `json:"-"`
}

// TableName specifies the table name for CommissionSettings.
func (CommissionSettings) TableName() string {
	return "general_settings"
}

// DefaultCommissionSettings returns the fallback rates when the singleton row
// does not exist yet.
func DefaultCommissionSettings() CommissionSettings {
	return CommissionSettings{
		ID:                              1,
		CommissionDefaultRate:           DefaultCommissionRate,
		CommissionUnassignedRate:        DefaultUnassignedCommissionRate,
		CommissionAcceptanceWindowHours: 2,
	}
}

// CommissionRulesResponse is the admin/seller view of the global rules.
type CommissionRulesResponse struct {
	DefaultRate           float64 `json:"default_rate"`
	UnassignedRate        float64 `json:"unassigned_rate"`
	PaymentRule           string  `json:"payment_rule"`
	AcceptanceWindowHours int     `json:"acceptance_window_hours"`
}

// CommissionExceptionResponse is one per-seller rate exception.
type CommissionExceptionResponse struct {
	ID         uint    `json:"id"`
	SellerID   uint    `json:"seller_id"`
	SellerName string  `json:"seller_name"`
	Rate       float64 `json:"rate"`
}

// CommissionRateForFinancing returns the project rate configured for a
// financing type. A zero value means the project did not configure one, so the
// caller must continue with the global default.
func (p *Project) CommissionRateForFinancing(financingType string) float64 {
	if p == nil {
		return 0
	}
	switch financingType {
	case FinancingTypeBank:
		return p.CommissionRateBank
	case FinancingTypeCash:
		return p.CommissionRateCash
	case FinancingTypeDirect:
		return p.CommissionRateDirect
	default:
		if p.CommissionRateDirect > 0 {
			return p.CommissionRateDirect
		}
		return p.CommissionRate
	}
}

// ResolveCommissionRate applies the precedence defined by the commissions
// capability: seller exception, project rate for the financing type, global
// default. The caller resolves the unassigned case before calling this helper.
func ResolveCommissionRate(sellerException *float64, project *Project, financingType string, defaults CommissionSettings) float64 {
	if sellerException != nil {
		return *sellerException
	}
	if rate := project.CommissionRateForFinancing(financingType); rate > 0 {
		return rate
	}
	return normalizeCommissionSettings(defaults).CommissionDefaultRate
}

// UnassignedCommissionRate returns the configured rate for contracts without a
// seller, falling back to 3% when the settings row is absent.
func UnassignedCommissionRate(defaults CommissionSettings) float64 {
	return normalizeCommissionSettings(defaults).CommissionUnassignedRate
}

func normalizeCommissionSettings(settings CommissionSettings) CommissionSettings {
	if settings.ID == 0 {
		return DefaultCommissionSettings()
	}
	return settings
}

// CommissionOriginForAssignment maps the reservation assignment origin to the
// commission origin stored on the contract.
func CommissionOriginForAssignment(assignmentOrigin string) string {
	if assignmentOrigin == AssignmentOriginReferral {
		return CommissionOriginReferral
	}
	return CommissionOriginDirect
}
