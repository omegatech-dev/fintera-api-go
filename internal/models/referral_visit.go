package models

import "time"

// ReferralVisit records one visit to a seller's public referral link. The
// session is stored hashed; raw IP/user agent never leave the request.
type ReferralVisit struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SellerID     uint      `gorm:"not null;index" json:"seller_id"`
	ReferralGUID string    `gorm:"column:slug;size:80;not null;index" json:"referral_guid"`
	VisitedAt    time.Time `gorm:"not null;index" json:"visited_at"`
	Path         *string   `gorm:"size:500" json:"path,omitempty"`
	SessionHash  *string   `gorm:"size:128" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName specifies the table name for ReferralVisit.
func (ReferralVisit) TableName() string {
	return "referral_visits"
}

// ReferralMetrics is the seller-facing referral summary.
type ReferralMetrics struct {
	PeriodDays             int      `json:"period_days"`
	Visits                 int64    `json:"visits"`
	AttributedReservations int64    `json:"attributed_reservations"`
	ConversionRate         float64  `json:"conversion_rate"`
	GeneratedCommission    float64  `json:"generated_commission"`
	ReferralGUID           *string  `json:"referral_guid,omitempty"`
	SellerName             string   `json:"seller_name,omitempty"`
	CommissionRate         *float64 `json:"commission_rate,omitempty"`
}

// ReferralReservation is the reservation row shown in the seller referral
// dashboard. It exposes the aliases the portal reads (client_name, lot_label)
// on top of the contract data.
type ReferralReservation struct {
	ID               uint      `json:"id"`
	ContractID       uint      `json:"contract_id"`
	GUID             string    `json:"guid"`
	ClientName       string    `json:"client_name"`
	ApplicantName    string    `json:"applicant_name"`
	ApplicantPhone   string    `json:"applicant_phone"`
	LotLabel         string    `json:"lot_label"`
	LotName          string    `json:"lot_name"`
	ProjectName      string    `json:"project_name"`
	Amount           *float64  `json:"amount"`
	CommissionAmount float64   `json:"commission_amount"`
	Status           string    `json:"status"`
	AssignmentStatus string    `json:"assignment_status"`
	CreatedAt        time.Time `json:"created_at"`
}

// ToReferralReservation converts a contract to the referral dashboard row.
func (c *Contract) ToReferralReservation() ReferralReservation {
	return ReferralReservation{
		ID:               c.ID,
		ContractID:       c.ID,
		GUID:             c.GUID,
		ClientName:       c.ApplicantUser.FullName,
		ApplicantName:    c.ApplicantUser.FullName,
		ApplicantPhone:   c.ApplicantUser.Phone,
		LotLabel:         c.Lot.Name,
		LotName:          c.Lot.Name,
		ProjectName:      c.Lot.Project.Name,
		Amount:           c.Amount,
		CommissionAmount: c.CommissionAmount,
		Status:           c.Status,
		AssignmentStatus: c.AssignmentStatus,
		CreatedAt:        c.CreatedAt,
	}
}
