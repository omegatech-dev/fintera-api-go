package models

import "time"

// Seller commission credit statuses.
const (
	CommissionCreditStatusPending = "pending"
	CommissionCreditStatusSettled = "settled"
)

// Seller commission credit sources.
const (
	CommissionCreditSourceCancellation = "cancellation"
	CommissionCreditSourceReassignment = "reassignment"
	CommissionCreditSourceLegacy       = "legacy"
)

// SellerCommissionCredit is a recoverable credit created when a contract whose
// commission was already paid to the seller is cancelled or reassigned. It
// stays pending until the seller's next commission consumes it.
type SellerCommissionCredit struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	SellerID      uint       `gorm:"column:seller_id;not null;index" json:"seller_id"`
	ContractID    uint       `gorm:"column:contract_id;not null" json:"contract_id"`
	Amount        float64    `gorm:"column:amount;type:decimal(15,2);not null" json:"amount"`
	AppliedAmount float64    `gorm:"column:applied_amount;type:decimal(15,2);not null;default:0" json:"applied_amount"`
	Status        string     `gorm:"column:status;size:20;not null;default:pending" json:"status"`
	Source        string     `gorm:"column:source;size:20;not null;default:legacy" json:"source"`
	CreatedAt     time.Time  `json:"created_at"`
	SettledAt     *time.Time `gorm:"column:settled_at" json:"settled_at"`
	SettledBy     *uint      `gorm:"column:settled_by" json:"settled_by,omitempty"`

	Seller   *User     `gorm:"foreignKey:SellerID" json:"seller,omitempty"`
	Contract *Contract `gorm:"foreignKey:ContractID" json:"contract,omitempty"`
}

// TableName specifies the table name for SellerCommissionCredit.
func (SellerCommissionCredit) TableName() string {
	return "seller_commission_credits"
}

// RemainingAmount is the part of the credit that has not been applied yet.
func (c *SellerCommissionCredit) RemainingAmount() float64 {
	remaining := RoundMoney(c.Amount - c.AppliedAmount)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// SellerCommissionCreditResponse is the JSON representation consumed by the
// commissions report and the admin credit actions. The seller is exposed by
// name so the UI can render it read-only.
type SellerCommissionCreditResponse struct {
	ID                uint                        `json:"id"`
	SellerID          uint                        `json:"seller_id"`
	Seller            string                      `json:"seller"`
	ContractID        uint                        `json:"contract_id"`
	ContractGUID      string                      `json:"contract_guid"`
	Amount            float64                     `json:"amount"`
	AppliedAmount     float64                     `json:"applied_amount"`
	RemainingAmount   float64                     `json:"remaining_amount"`
	Status            string                      `json:"status"`
	Source            string                      `json:"source"`
	CreatedAt         time.Time                   `json:"created_at"`
	SettledAt         *time.Time                  `json:"settled_at"`
	SettledBy         *uint                       `json:"settled_by,omitempty"`
	ApplicationsTotal int                         `json:"applications_total"`
	Applications      []CreditApplicationResponse `json:"applications"`
}

// ToResponse converts a credit to its API representation.
func (c *SellerCommissionCredit) ToResponse() SellerCommissionCreditResponse {
	response := c.baseResponse()
	response.Applications = []CreditApplicationResponse{}
	return response
}

// ToResponseWithApplications converts a credit and its active applications to
// the API representation. total is the number of active applications before the
// display cap is applied.
func (c *SellerCommissionCredit) ToResponseWithApplications(applications []CreditApplication, total int) SellerCommissionCreditResponse {
	response := c.baseResponse()
	response.ApplicationsTotal = total
	response.Applications = make([]CreditApplicationResponse, 0, len(applications))
	for i := range applications {
		response.Applications = append(response.Applications, applications[i].ToResponse())
	}
	return response
}

func (c *SellerCommissionCredit) baseResponse() SellerCommissionCreditResponse {
	sellerName := ""
	if c.Seller != nil {
		sellerName = c.Seller.FullName
	}
	contractGUID := ""
	if c.Contract != nil {
		contractGUID = c.Contract.GUID
	}
	status := c.Status
	if status == "" {
		status = CommissionCreditStatusPending
	}
	source := c.Source
	if source == "" {
		// Credits created before sources were tracked cannot claim a concrete
		// origin; legacy is the honest label.
		source = CommissionCreditSourceLegacy
	}
	return SellerCommissionCreditResponse{
		ID:              c.ID,
		SellerID:        c.SellerID,
		Seller:          sellerName,
		ContractID:      c.ContractID,
		ContractGUID:    contractGUID,
		Amount:          c.Amount,
		AppliedAmount:   RoundMoney(c.AppliedAmount),
		RemainingAmount: c.RemainingAmount(),
		Status:          status,
		Source:          source,
		CreatedAt:       c.CreatedAt,
		SettledAt:       c.SettledAt,
		SettledBy:       c.SettledBy,
		Applications:    []CreditApplicationResponse{},
	}
}
