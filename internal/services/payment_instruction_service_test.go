package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
)

func TestValidateInstruction(t *testing.T) {
	valid := func() *models.ProjectPaymentInstruction {
		return &models.ProjectPaymentInstruction{
			BankName:        "  BAC Credomatic ",
			BeneficiaryName: " Fintera S.A. ",
			AccountNumber:   " 741-209-4321 ",
		}
	}

	t.Run("trims values and defaults the account type", func(t *testing.T) {
		instruction := valid()
		if err := validateInstruction(instruction); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if instruction.BankName != "BAC Credomatic" {
			t.Errorf("bank = %q", instruction.BankName)
		}
		if instruction.BeneficiaryName != "Fintera S.A." {
			t.Errorf("beneficiary = %q", instruction.BeneficiaryName)
		}
		if instruction.AccountNumber != "741-209-4321" {
			t.Errorf("account = %q", instruction.AccountNumber)
		}
		if instruction.AccountType != models.PaymentAccountTypeSavings {
			t.Errorf("account type = %q", instruction.AccountType)
		}
	})

	t.Run("normalizes the account type", func(t *testing.T) {
		instruction := valid()
		instruction.AccountType = " CHECKING "
		if err := validateInstruction(instruction); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if instruction.AccountType != models.PaymentAccountTypeChecking {
			t.Errorf("account type = %q", instruction.AccountType)
		}
	})

	t.Run("requires bank, beneficiary and account number", func(t *testing.T) {
		cases := []func(*models.ProjectPaymentInstruction){
			func(i *models.ProjectPaymentInstruction) { i.BankName = "  " },
			func(i *models.ProjectPaymentInstruction) { i.BeneficiaryName = "" },
			func(i *models.ProjectPaymentInstruction) { i.AccountNumber = "" },
		}
		for _, mutate := range cases {
			instruction := valid()
			mutate(instruction)
			if err := validateInstruction(instruction); !errors.Is(err, ErrInstructionInvalidData) {
				t.Errorf("error = %v, want ErrInstructionInvalidData", err)
			}
		}
	})

	t.Run("rejects an unsupported account type", func(t *testing.T) {
		instruction := valid()
		instruction.AccountType = "credit"
		if err := validateInstruction(instruction); !errors.Is(err, ErrInstructionInvalidType) {
			t.Fatalf("error = %v, want ErrInstructionInvalidType", err)
		}
	})

	t.Run("enforces field lengths", func(t *testing.T) {
		tooLong := []func(*models.ProjectPaymentInstruction){
			func(i *models.ProjectPaymentInstruction) {
				i.BankName = strings.Repeat("a", MaxInstructionBankNameLength+1)
			},
			func(i *models.ProjectPaymentInstruction) {
				i.BeneficiaryName = strings.Repeat("a", MaxInstructionBeneficiaryNameLength+1)
			},
			func(i *models.ProjectPaymentInstruction) {
				i.AccountNumber = strings.Repeat("1", MaxInstructionAccountNumberLength+1)
			},
			func(i *models.ProjectPaymentInstruction) {
				notes := strings.Repeat("a", MaxInstructionNotesLength+1)
				i.Notes = &notes
			},
		}
		for _, mutate := range tooLong {
			instruction := valid()
			mutate(instruction)
			if err := validateInstruction(instruction); !errors.Is(err, ErrInstructionInvalidLength) {
				t.Errorf("error = %v, want ErrInstructionInvalidLength", err)
			}
		}
	})

	t.Run("accepts values at the limit", func(t *testing.T) {
		instruction := valid()
		instruction.BankName = strings.Repeat("a", MaxInstructionBankNameLength)
		instruction.BeneficiaryName = strings.Repeat("b", MaxInstructionBeneficiaryNameLength)
		instruction.AccountNumber = strings.Repeat("1", MaxInstructionAccountNumberLength)
		notes := strings.Repeat("c", MaxInstructionNotesLength)
		instruction.Notes = &notes
		if err := validateInstruction(instruction); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("trims notes", func(t *testing.T) {
		instruction := valid()
		notes := "  paga antes del viernes  "
		instruction.Notes = &notes
		if err := validateInstruction(instruction); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *instruction.Notes != "paga antes del viernes" {
			t.Errorf("notes = %q", *instruction.Notes)
		}
	})
}
