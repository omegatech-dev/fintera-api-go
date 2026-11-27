package models

import "time"

// CreditApplication records how much of a seller commission credit was applied
// to a specific contract when its commission was paid. Reversals are soft: the
// row stays for the audit trail and is excluded once reversed_at is set.
type CreditApplication struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	CreditID   uint       `gorm:"column:credit_id;not null;index" json:"credit_id"`
	ContractID uint       `gorm:"column:contract_id;not null;index" json:"contract_id"`
	Amount     float64    `gorm:"column:amount;type:decimal(15,2);not null" json:"amount"`
	ActorID    *uint      `gorm:"column:actor_id" json:"actor_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ReversedAt *time.Time `gorm:"column:reversed_at" json:"reversed_at,omitempty"`
	ReversedBy *uint      `gorm:"column:reversed_by" json:"reversed_by,omitempty"`
	// ApplicationsTotal is populated by the grouped listing query and is not a
	// persisted column.
	ApplicationsTotal int `gorm:"-" json:"-"`

	Contract *Contract `gorm:"foreignKey:ContractID" json:"contract,omitempty"`
}

// TableName specifies the table name for CreditApplication.
func (CreditApplication) TableName() string {
	return "credit_applications"
}

// CreditApplicationResponse is the JSON representation of one application. The
// contract fields are prefixed with "applied" to distinguish the destination
// contract from the credit's origin contract.
type CreditApplicationResponse struct {
	ID                  uint      `json:"id"`
	CreditID            uint      `json:"credit_id"`
	AppliedContractID   uint      `json:"applied_contract_id"`
	AppliedContractGUID string    `json:"applied_contract_guid"`
	Amount              float64   `json:"amount"`
	CreatedAt           time.Time `json:"created_at"`
}

// ToResponse converts an application to its API representation.
func (a *CreditApplication) ToResponse() CreditApplicationResponse {
	contractID := a.ContractID
	contractGUID := ""
	if a.Contract != nil {
		contractID = a.Contract.ID
		contractGUID = a.Contract.GUID
	}
	return CreditApplicationResponse{
		ID:                  a.ID,
		CreditID:            a.CreditID,
		AppliedContractID:   contractID,
		AppliedContractGUID: contractGUID,
		Amount:              a.Amount,
		CreatedAt:           a.CreatedAt,
	}
}

// CommissionPayout is the result of paying a commission. AppliedTotal plus
// NetCash equals Commission.
type CommissionPayout struct {
	Commission      float64                     `json:"commission"`
	AppliedTotal    float64                     `json:"applied_total"`
	NetCash         float64                     `json:"net_cash"`
	RemainingCredit float64                     `json:"remaining_credit"`
	Applications    []CreditApplicationResponse `json:"applications"`
	// SellerID is returned internally from the locked payout transaction so
	// concurrent reassignment cannot leave the service response on a stale
	// seller. It is not part of the public payout contract.
	SellerID *uint `json:"-"`
}

// CommissionPayoutPreview explains what would happen if the commission were
// paid right now. It never mutates credits.
type CommissionPayoutPreview struct {
	Commission      float64 `json:"commission"`
	AvailableCredit float64 `json:"available_credit"`
	Applicable      float64 `json:"applicable"`
	NetCash         float64 `json:"net_cash"`
	RemainingCredit float64 `json:"remaining_credit"`
}

// ContractCommissionPaidResponse is the body returned when an admin marks a
// contract commission paid. Payout carries the credit applications that were
// consumed, if any.
type ContractCommissionPaidResponse struct {
	Contract ContractResponse  `json:"contract"`
	Message  string            `json:"message"`
	Payout   *CommissionPayout `json:"payout,omitempty"`
}

// CommissionCompensation is the result of reversing the credit applications of
// a contract and creating the credit for the cash that remains recoverable.
type CommissionCompensation struct {
	Restored float64                 `json:"restored"`
	NetCash  float64                 `json:"net_cash"`
	Credit   *SellerCommissionCredit `json:"credit,omitempty"`
}
