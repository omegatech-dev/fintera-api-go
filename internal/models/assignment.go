package models

import "time"

// Assignment is one offer of a reservation to a seller. Every attempt is kept
// as a separate row so the acceptance score can count accepted and expired
// offers over a rolling window. The contract keeps a copy of the current
// seller/status for fast list reads.
type Assignment struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	ContractID       uint       `gorm:"not null;index" json:"contract_id"`
	SellerID         uint       `gorm:"not null;index" json:"seller_id"`
	Origin           string     `gorm:"size:20;not null;default:automatic" json:"origin"`
	Status           string     `gorm:"size:20;not null;default:pending" json:"status"`
	AssignedAt       time.Time  `gorm:"not null" json:"assigned_at"`
	AcceptDeadlineAt *time.Time `gorm:"index" json:"accept_deadline_at"`
	RespondedAt      *time.Time `json:"responded_at"`
	CreatedAt        time.Time  `json:"created_at"`

	Contract Contract `gorm:"foreignKey:ContractID" json:"contract,omitempty"`
	Seller   *User    `gorm:"foreignKey:SellerID" json:"seller,omitempty"`
}

// TableName specifies the table name for Assignment.
func (Assignment) TableName() string {
	return "assignments"
}

// Assignment origin constants. `none` and `historical` are contract-level
// values; assignment rows use referral, manual or automatic.
const (
	AssignmentOriginReferral   = "referral"
	AssignmentOriginManual     = "manual"
	AssignmentOriginAutomatic  = "automatic"
	AssignmentOriginNone       = "none"
	AssignmentOriginHistorical = "historical"
)

// Assignment status constants.
const (
	AssignmentStatusNone     = "none"
	AssignmentStatusPending  = "pending"
	AssignmentStatusAccepted = "accepted"
	AssignmentStatusExpired  = "expired"
)

// DefaultAssignmentAcceptWindow is how long a seller has to answer a manual or
// automatic offer.
const DefaultAssignmentAcceptWindow = 2 * time.Hour

// AssignmentAcceptWindow returns the configured acceptance window.
func AssignmentAcceptWindow() time.Duration {
	return DefaultAssignmentAcceptWindow
}

// AssignmentDeadlineFrom returns the acceptance deadline for an offer created
// at `now`.
func AssignmentDeadlineFrom(now time.Time) time.Time {
	return now.Add(AssignmentAcceptWindow())
}

// IsResponded reports whether the assignment already has a final answer.
func (a *Assignment) IsResponded() bool {
	return a.Status == AssignmentStatusAccepted || a.Status == AssignmentStatusExpired
}

// IsOverdue reports whether a pending assignment passed its deadline.
func (a *Assignment) IsOverdue(now time.Time) bool {
	if a.Status != AssignmentStatusPending || a.AcceptDeadlineAt == nil {
		return false
	}
	return now.After(*a.AcceptDeadlineAt)
}

// ScoreOutcome aggregates the accepted and expired offers of one seller over
// the score window.
type ScoreOutcome struct {
	SellerID uint
	Accepted int64
	Expired  int64
}

// AssignmentSellerSummary is the seller embedded in assignment responses.
type AssignmentSellerSummary struct {
	ID       uint    `json:"id"`
	FullName string  `json:"full_name"`
	Phone    string  `json:"phone"`
	Email    string  `json:"email"`
	Address  *string `json:"address"`
}

// AssignmentContractSummary is the reservation data shown with a pending
// assignment. It intentionally avoids the full contract payload.
type AssignmentContractSummary struct {
	ID               uint      `json:"id"`
	GUID             string    `json:"guid"`
	Status           string    `json:"status"`
	FinancingType    string    `json:"financing_type"`
	Amount           *float64  `json:"amount"`
	ReserveAmount    *float64  `json:"reserve_amount"`
	DownPayment      *float64  `json:"down_payment"`
	CommissionAmount float64   `json:"commission_amount"`
	CreatedAt        time.Time `json:"created_at"`
	ProjectName      string    `json:"project_name"`
	LotName          string    `json:"lot_name"`
	ApplicantName    string    `json:"applicant_name"`
	ApplicantPhone   string    `json:"applicant_phone"`
	ApplicantEmail   string    `json:"applicant_email"`
	ApplicantUserID  uint      `json:"applicant_user_id"`
}

// AssignmentResponse is the JSON representation of an assignment.
type AssignmentResponse struct {
	ID               uint                       `json:"id"`
	ContractID       uint                       `json:"contract_id"`
	SellerID         uint                       `json:"seller_id"`
	Origin           string                     `json:"origin"`
	Status           string                     `json:"status"`
	AssignedAt       time.Time                  `json:"assigned_at"`
	AcceptDeadlineAt *time.Time                 `json:"accept_deadline_at"`
	RespondedAt      *time.Time                 `json:"responded_at"`
	CreatedAt        time.Time                  `json:"created_at"`
	Seller           *AssignmentSellerSummary   `json:"seller,omitempty"`
	Contract         *AssignmentContractSummary `json:"contract,omitempty"`
}

// ToResponse converts an Assignment to its API representation.
func (a *Assignment) ToResponse() AssignmentResponse {
	resp := AssignmentResponse{
		ID:               a.ID,
		ContractID:       a.ContractID,
		SellerID:         a.SellerID,
		Origin:           a.Origin,
		Status:           a.Status,
		AssignedAt:       a.AssignedAt,
		AcceptDeadlineAt: a.AcceptDeadlineAt,
		RespondedAt:      a.RespondedAt,
		CreatedAt:        a.CreatedAt,
	}
	if a.Seller != nil {
		resp.Seller = &AssignmentSellerSummary{
			ID:       a.Seller.ID,
			FullName: a.Seller.FullName,
			Phone:    a.Seller.Phone,
			Email:    a.Seller.Email,
			Address:  a.Seller.Address,
		}
	}
	if a.Contract.ID != 0 {
		resp.Contract = &AssignmentContractSummary{
			ID:               a.Contract.ID,
			GUID:             a.Contract.GUID,
			Status:           a.Contract.Status,
			FinancingType:    a.Contract.FinancingType,
			Amount:           a.Contract.Amount,
			ReserveAmount:    a.Contract.ReserveAmount,
			DownPayment:      a.Contract.DownPayment,
			CommissionAmount: a.Contract.CommissionAmount,
			CreatedAt:        a.Contract.CreatedAt,
			ProjectName:      a.Contract.Lot.Project.Name,
			LotName:          a.Contract.Lot.Name,
			ApplicantName:    a.Contract.ApplicantUser.FullName,
			ApplicantPhone:   a.Contract.ApplicantUser.Phone,
			ApplicantEmail:   a.Contract.ApplicantUser.Email,
			ApplicantUserID:  a.Contract.ApplicantUserID,
		}
	}
	return resp
}
