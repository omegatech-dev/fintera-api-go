package models

import (
	"strings"
	"time"
)

// Contract represents a sales contract for a lot
type Contract struct {
	ID              uint     `gorm:"primaryKey" json:"id"`
	LotID           uint     `gorm:"not null;index" json:"lot_id"`
	CreatorID       *uint    `gorm:"index" json:"creator_id"`
	ApplicantUserID uint     `gorm:"not null;index" json:"applicant_user_id"`
	PaymentTerm     int      `gorm:"not null" json:"payment_term"`
	GUID            string   `gorm:"column:guid;not null" json:"guid"`
	FinancingType   string   `gorm:"not null" json:"financing_type"`
	Status          string   `gorm:"default:pending;index" json:"status"`
	Amount          *float64 `gorm:"type:decimal" json:"amount"`
	Balance         *float64 `gorm:"type:decimal" json:"balance"`
	// Commission
	CommissionAmount          float64    `json:"commission_amount" gorm:"type:decimal(15,2);default:0"`
	CommissionPaidAt          *time.Time `gorm:"index" json:"commission_paid_at"`
	CommissionPaidBy          *uint      `json:"commission_paid_by"`
	DownPayment               *float64   `gorm:"type:decimal" json:"down_payment"`
	ReserveAmount             *float64   `gorm:"type:decimal" json:"reserve_amount"`
	MaxPaymentDate            *time.Time `gorm:"type:date" json:"max_payment_date"` // for bank/cash: date by which customer will pay the rest
	BankName                  *string    `gorm:"size:120" json:"bank_name"`
	BankCreditStatus          *string    `gorm:"size:30" json:"bank_credit_status"`
	Currency                  string     `gorm:"default:HNL;not null" json:"currency"`
	ApprovedAt                *time.Time `gorm:"index" json:"approved_at"`
	SubmittedAt               *time.Time `json:"submitted_at"`
	RejectedAt                *time.Time `json:"rejected_at"`
	Active                    bool       `gorm:"default:false;index" json:"active"`
	Note                      *string    `gorm:"type:text" json:"note"`
	RejectionReason           *string    `gorm:"type:text" json:"rejection_reason"`
	TotalPaid                 float64    `gorm:"-" json:"total_paid"`             // Transient field for list view
	NextPayment               *Payment   `gorm:"-" json:"next_payment,omitempty"` // Transient, filled by list batching
	DocumentPaths             *string    `gorm:"type:text" json:"document_paths"` // JSON string of document paths
	ClosedAt                  *time.Time `json:"closed_at"`
	ReservationReminderSentAt *time.Time `gorm:"column:reservation_reminder_sent_at" json:"-"`
	CreatedAt                 time.Time  `gorm:"index" json:"created_at"` // index for list sort and date filters
	UpdatedAt                 time.Time  `json:"updated_at"`

	// Associations
	Lot           Lot                   `gorm:"foreignKey:LotID" json:"lot,omitempty"`
	Creator       *User                 `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	ApplicantUser User                  `gorm:"foreignKey:ApplicantUserID" json:"applicant_user,omitempty"`
	Payments      []Payment             `gorm:"foreignKey:ContractID" json:"payments,omitempty"`
	LedgerEntries []ContractLedgerEntry `gorm:"foreignKey:ContractID" json:"ledger_entries,omitempty"`
}

// TableName specifies the table name for Contract
func (Contract) TableName() string {
	return "contracts"
}

// Contract status constants
const (
	ContractStatusPending   = "pending"
	ContractStatusSubmitted = "submitted"
	ContractStatusApproved  = "approved"
	ContractStatusSigned    = "signed"
	ContractStatusRejected  = "rejected"
	ContractStatusCancelled = "cancelled"
	ContractStatusClosed    = "closed"
)

// Financing type constants
const (
	FinancingTypeDirect = "direct"
	FinancingTypeBank   = "bank"
	FinancingTypeCash   = "cash"
)

// Bank credit status constants. They describe where the customer's loan
// application stands at reservation time.
const (
	BankCreditStatusNotStarted   = "not_started"
	BankCreditStatusPreQualified = "pre_qualified"
	BankCreditStatusPreApproved  = "pre_approved"
	BankCreditStatusApproved     = "approved"
)

// NormalizeBankCreditStatus trims and lowercases a bank credit status. It
// returns ok=false when the value is not one of the supported stages. An empty
// value is valid and means "not provided".
func NormalizeBankCreditStatus(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "",
		BankCreditStatusNotStarted,
		BankCreditStatusPreQualified,
		BankCreditStatusPreApproved,
		BankCreditStatusApproved:
		return normalized, true
	default:
		return "", false
	}
}

// MaySubmit returns true if contract can transition to submitted
func (c *Contract) MaySubmit() bool {
	return c.Status == ContractStatusPending || c.Status == ContractStatusRejected
}

// MayApprove returns true if contract can be approved
func (c *Contract) MayApprove() bool {
	return c.Status == ContractStatusPending ||
		c.Status == ContractStatusSubmitted ||
		c.Status == ContractStatusRejected
}

// MayReject returns true if contract can be rejected
func (c *Contract) MayReject() bool {
	return c.Status == ContractStatusPending || c.Status == ContractStatusSubmitted
}

// MayCancel returns true if contract can be cancelled
func (c *Contract) MayCancel() bool {
	return c.Status == ContractStatusPending ||
		c.Status == ContractStatusSubmitted ||
		c.Status == ContractStatusRejected
}

// MayClose returns true if contract can be closed
func (c *Contract) MayClose() bool {
	if c.Status != ContractStatusApproved {
		return false
	}
	if c.Balance == nil {
		return false
	}
	return *c.Balance <= 0
}

// MayReopen returns true if contract can be reopened
func (c *Contract) MayReopen() bool {
	return c.Status == ContractStatusClosed
}

// CalculateBalance calculates the current balance from ledger entries
func (c *Contract) CalculateBalance() float64 {
	var total float64
	for _, entry := range c.LedgerEntries {
		total += entry.Amount
	}
	return total
}

// CalculateCommission calculates the commission based on the project's rates and financing type
func (c *Contract) CalculateCommission() float64 {
	// Need amount and project info to calculate
	if c.Amount == nil || c.Lot.Project.ID == 0 {
		return 0
	}

	rate := c.Lot.Project.CommissionRateDirect // Default
	switch c.FinancingType {
	case FinancingTypeDirect:
		rate = c.Lot.Project.CommissionRateDirect
	case FinancingTypeBank:
		rate = c.Lot.Project.CommissionRateBank
	case FinancingTypeCash:
		rate = c.Lot.Project.CommissionRateCash
	}

	return *c.Amount * (rate / 100)
}

// ContractResponse is the JSON response format for contracts
type ContractResponse struct {
	ID                     uint                          `json:"id"`
	ContractID             uint                          `json:"contract_id"`
	GUID                   string                        `json:"guid"`
	ProjectID              uint                          `json:"project_id"`
	ProjectGUID            string                        `json:"project_guid"`
	ProjectName            string                        `json:"project_name"`
	ProjectAddress         string                        `json:"project_address"`
	LotID                  uint                          `json:"lot_id"`
	LotName                string                        `json:"lot_name"`
	LotAddress             *string                       `json:"lot_address"`
	LotWidth               float64                       `json:"lot_width"`
	LotLength              float64                       `json:"lot_length"`
	LotArea                float64                       `json:"lot_area"`
	LotStatus              string                        `json:"lot_status"`
	LotMeasurementUnit     *string                       `json:"lot_measurement_unit"`
	LotPrice               float64                       `json:"lot_price"`
	LotOverridePrice       *float64                      `json:"lot_override_price"`
	ApplicantUserID        uint                          `json:"applicant_user_id"`
	ApplicantName          string                        `json:"applicant_name"`
	ApplicantPhone         string                        `json:"applicant_phone"`
	ApplicantIdentity      string                        `json:"applicant_identity"`
	ApplicantCreditScore   int                           `json:"applicant_credit_score"`
	CreatedBy              string                        `json:"created_by"`
	Amount                 *float64                      `json:"amount"`
	TotalAmount            *float64                      `json:"total_amount"`
	PaymentTerm            int                           `json:"payment_term"`
	FinancingType          string                        `json:"financing_type"`
	ReserveAmount          *float64                      `json:"reserve_amount"`
	DownPayment            *float64                      `json:"down_payment"`
	MaxPaymentDate         *time.Time                    `json:"max_payment_date"`
	BankName               *string                       `json:"bank_name"`
	BankCreditStatus       *string                       `json:"bank_credit_status"`
	Status                 string                        `json:"status"`
	Balance                float64                       `json:"balance"`
	RejectionReason        *string                       `json:"rejection_reason"`
	CancellationNotes      *string                       `json:"cancellation_notes"`
	TotalInterestCollected float64                       `json:"total_interest_collected"`
	TotalPaid              float64                       `json:"total_paid"`
	CommissionAmount       float64                       `json:"commission_amount"`
	CommissionPaidAt       *time.Time                    `json:"commission_paid_at"`
	CommissionPaidBy       *uint                         `json:"commission_paid_by"`
	ApprovedAt             *time.Time                    `json:"approved_at"`
	SubmittedAt            *time.Time                    `json:"submitted_at"`
	RejectedAt             *time.Time                    `json:"rejected_at"`
	HoldExpiresAt          *time.Time                    `json:"hold_expires_at"`
	CreatedAt              time.Time                     `json:"created_at"`
	UpdatedAt              time.Time                     `json:"updated_at"`
	Note                   *string                       `json:"note"`
	PaymentSchedule        []PaymentResponse             `json:"payment_schedule"`
	NextPayment            *NextPaymentResponse          `json:"next_payment,omitempty"`
	NextPaymentID          *uint                         `json:"next_payment_id,omitempty"`
	NextPaymentDate        *time.Time                    `json:"next_payment_date,omitempty"`
	NextPaymentAmount      *float64                      `json:"next_payment_amount,omitempty"`
	NextPaymentType        string                        `json:"next_payment_type,omitempty"`
	NextPaymentLabel       string                        `json:"next_payment_label,omitempty"`
	ReservationPayment     *PaymentResponse              `json:"reservation_payment,omitempty"`
	LedgerEntries          []ContractLedgerEntryResponse `json:"ledger_entries"`
}

// ToResponse converts Contract to ContractResponse
func (c *Contract) ToResponse() ContractResponse {
	resp := ContractResponse{
		ID:                c.ID,
		ContractID:        c.ID,
		GUID:              c.GUID,
		LotID:             c.LotID,
		ApplicantUserID:   c.ApplicantUserID,
		PaymentTerm:       c.PaymentTerm,
		FinancingType:     c.FinancingType,
		Amount:            c.Amount,
		TotalAmount:       c.Amount,
		ReserveAmount:     c.ReserveAmount,
		DownPayment:       c.DownPayment,
		MaxPaymentDate:    c.MaxPaymentDate,
		BankName:          c.BankName,
		BankCreditStatus:  c.BankCreditStatus,
		Status:            c.Status,
		RejectionReason:   c.RejectionReason,
		CancellationNotes: c.Note,
		ApprovedAt:        c.ApprovedAt,
		SubmittedAt:       c.SubmittedAt,
		RejectedAt:        c.RejectedAt,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
		Note:              c.Note,
	}

	// Add lot info
	resp.LotName = c.Lot.Name
	resp.LotAddress = c.Lot.Address
	resp.LotWidth = c.Lot.Width
	resp.LotLength = c.Lot.Length
	resp.LotArea = c.Lot.Area()
	resp.LotStatus = c.Lot.Status
	lotUnit := ResolveMeasurementUnit(c.Lot.MeasurementUnit, c.Lot.Project.MeasurementUnit)
	resp.LotMeasurementUnit = &lotUnit
	resp.LotPrice = c.Lot.Price
	resp.LotOverridePrice = c.Lot.OverridePrice

	// Add project info
	resp.ProjectID = c.Lot.ProjectID
	resp.ProjectGUID = c.Lot.Project.GUID
	resp.ProjectName = c.Lot.Project.Name
	resp.ProjectAddress = c.Lot.Project.Address

	// Add applicant info
	resp.ApplicantName = c.ApplicantUser.FullName
	resp.ApplicantPhone = c.ApplicantUser.Phone
	resp.ApplicantIdentity = maskIdentity(c.ApplicantUser.Identity)
	resp.ApplicantCreditScore = c.ApplicantUser.CreditScore

	// Add creator info
	if c.Creator != nil {
		resp.CreatedBy = c.Creator.FullName
	}

	// Calculate balance and totals
	// Use cached balance if available, otherwise calculate from ledger
	if c.Balance != nil {
		resp.Balance = *c.Balance
	} else {
		resp.Balance = c.CalculateBalance()
	}

	// Calculate totals from payments
	var totalInterest, totalPaid float64

	if c.TotalPaid > 0 {
		totalPaid = c.TotalPaid
	} else {
		for _, p := range c.Payments {
			if p.InterestAmount != nil {
				totalInterest += *p.InterestAmount
			}
			if p.Status == PaymentStatusPaid && p.PaidAmount != nil {
				totalPaid += *p.PaidAmount
			}
		}
	}
	resp.TotalInterestCollected = totalInterest
	resp.TotalPaid = totalPaid
	resp.CommissionAmount = c.CommissionAmount
	resp.CommissionPaidAt = c.CommissionPaidAt
	resp.CommissionPaidBy = c.CommissionPaidBy

	// Add payment schedule
	for _, payment := range c.Payments {
		paymentResp := payment.ToResponse()
		resp.PaymentSchedule = append(resp.PaymentSchedule, paymentResp)
		if payment.PaymentType == PaymentTypeReservation && resp.ReservationPayment == nil {
			entry := paymentResp
			resp.ReservationPayment = &entry
		}
	}

	// Expose the next installment a seller can collect. The list endpoint
	// batches this into c.NextPayment to avoid preloading every payment.
	nextPayment := c.NextPayment
	if nextPayment == nil {
		nextPayment = FindCollectablePayment(c.Payments)
	}
	if nextPayment != nil && c.collectsPayments() {
		nextPaymentResp := nextPayment.ToNextPaymentResponse()
		resp.NextPayment = &nextPaymentResp
		id := nextPayment.ID
		amount := nextPayment.Amount
		dueDate := nextPayment.DueDate
		resp.NextPaymentID = &id
		resp.NextPaymentDate = &dueDate
		resp.NextPaymentAmount = &amount
		resp.NextPaymentType = nextPayment.PaymentType
		resp.NextPaymentLabel = nextPaymentResp.Label
	}

	// The hold remains meaningful until the reservation is approved, rejected
	// or cancelled. Jobs remain the source of truth for releasing lots.
	if (c.Status == ContractStatusPending || c.Status == ContractStatusSubmitted) && !c.CreatedAt.IsZero() {
		hold := HoldExpiresAt(c.CreatedAt)
		resp.HoldExpiresAt = &hold
	}

	// Add ledger entries
	for _, entry := range c.LedgerEntries {
		resp.LedgerEntries = append(resp.LedgerEntries, entry.ToResponse())
	}

	return resp
}

// collectsPayments reports whether the contract lifecycle still expects
// collections. Cancelled, rejected and closed contracts keep their history but
// must not advertise a "next payment" to sellers.
func (c *Contract) collectsPayments() bool {
	switch c.Status {
	case ContractStatusPending, ContractStatusSubmitted, ContractStatusApproved, ContractStatusSigned:
		return true
	default:
		return false
	}
}

// FindCollectablePayment returns the earliest payment a seller can still
// collect: pending or submitted, without a receipt, sorted by due date. It
// matches the rule used by the seller collection dialog.
func FindCollectablePayment(payments []Payment) *Payment {
	var next *Payment
	for i := range payments {
		payment := &payments[i]
		if payment.Status != PaymentStatusPending && payment.Status != PaymentStatusSubmitted {
			continue
		}
		if payment.DocumentPath != nil && strings.TrimSpace(*payment.DocumentPath) != "" {
			continue
		}
		if next == nil || payment.DueDate.Before(next.DueDate) ||
			(payment.DueDate.Equal(next.DueDate) && payment.ID < next.ID) {
			next = payment
		}
	}
	return next
}

// maskIdentity masks an identity string for privacy
func maskIdentity(identity string) string {
	if len(identity) <= 4 {
		masked := ""
		for range identity {
			masked += "*"
		}
		return masked
	}
	masked := identity[:4]
	for i := 4; i < len(identity)-3; i++ {
		masked += "*"
	}
	if len(identity) > 4 {
		masked += identity[len(identity)-3:]
	}
	return masked
}
