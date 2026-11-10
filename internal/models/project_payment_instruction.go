package models

import "time"

// ProjectPaymentInstruction describes a bank account where customers can pay a
// reservation for a given project.
type ProjectPaymentInstruction struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ProjectID       uint      `gorm:"not null;index" json:"project_id"`
	BankName        string    `gorm:"not null" json:"bank_name"`
	BeneficiaryName string    `gorm:"not null" json:"beneficiary_name"`
	AccountNumber   string    `gorm:"not null" json:"account_number"`
	AccountType     string    `gorm:"not null;default:savings" json:"account_type"`
	Notes           *string   `gorm:"type:text" json:"notes"`
	Position        int       `gorm:"not null;default:0" json:"position"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName specifies the table name for ProjectPaymentInstruction.
func (ProjectPaymentInstruction) TableName() string {
	return "project_payment_instructions"
}

// Account type constants.
const (
	PaymentAccountTypeSavings  = "savings"
	PaymentAccountTypeChecking = "checking"

	// AccountTypeSavings and AccountTypeChecking are the canonical names used
	// by the payment instruction service/handlers.
	AccountTypeSavings  = PaymentAccountTypeSavings
	AccountTypeChecking = PaymentAccountTypeChecking
)

// ProjectPaymentInstructionResponse is the admin-facing JSON representation.
type ProjectPaymentInstructionResponse struct {
	ID              uint      `json:"id"`
	ProjectID       uint      `json:"project_id"`
	BankName        string    `json:"bank_name"`
	BeneficiaryName string    `json:"beneficiary_name"`
	AccountNumber   string    `json:"account_number"`
	AccountType     string    `json:"account_type"`
	Notes           *string   `json:"notes"`
	Position        int       `json:"position"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PaymentInstructionResponse is kept as an alias for callers that prefer the
// shorter name.
type PaymentInstructionResponse = ProjectPaymentInstructionResponse

// ToResponse converts a ProjectPaymentInstruction to its admin response.
func (p *ProjectPaymentInstruction) ToResponse() PaymentInstructionResponse {
	return PaymentInstructionResponse{
		ID:              p.ID,
		ProjectID:       p.ProjectID,
		BankName:        p.BankName,
		BeneficiaryName: p.BeneficiaryName,
		AccountNumber:   p.AccountNumber,
		AccountType:     p.AccountType,
		Notes:           p.Notes,
		Position:        p.Position,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

// PublicPaymentInstructionResponse exposes only the fields the customer portal
// needs to display "where to pay".
type PublicPaymentInstructionResponse struct {
	BankName        string  `json:"bank_name"`
	BeneficiaryName string  `json:"beneficiary_name"`
	AccountNumber   string  `json:"account_number"`
	AccountType     string  `json:"account_type"`
	Notes           *string `json:"notes"`
}

// ToPublicResponse converts a ProjectPaymentInstruction to the public shape.
func (p *ProjectPaymentInstruction) ToPublicResponse() PublicPaymentInstructionResponse {
	return PublicPaymentInstructionResponse{
		BankName:        p.BankName,
		BeneficiaryName: p.BeneficiaryName,
		AccountNumber:   p.AccountNumber,
		AccountType:     p.AccountType,
		Notes:           p.Notes,
	}
}
