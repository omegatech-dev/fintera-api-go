package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// Payment instruction validation errors.
var (
	ErrInstructionInvalidType = errors.New("el tipo de cuenta debe ser savings o checking")
	ErrInstructionInvalidData = errors.New("banco, beneficiario y número de cuenta son requeridos")
)

// ProjectPaymentInstructionService manages project bank instructions.
type ProjectPaymentInstructionService struct {
	instructions repository.ProjectPaymentInstructionRepository
	projects     repository.ProjectRepository
	audit        *AuditService
}

// NewProjectPaymentInstructionService creates the service.
func NewProjectPaymentInstructionService(
	instructions repository.ProjectPaymentInstructionRepository,
	projects repository.ProjectRepository,
	audit *AuditService,
) *ProjectPaymentInstructionService {
	return &ProjectPaymentInstructionService{
		instructions: instructions,
		projects:     projects,
		audit:        audit,
	}
}

// List returns the instructions of a project for the admin view.
func (s *ProjectPaymentInstructionService) List(ctx context.Context, projectID uint) ([]models.ProjectPaymentInstruction, error) {
	return s.instructions.ListByProject(ctx, projectID)
}

// Create validates and stores an instruction.
func (s *ProjectPaymentInstructionService) Create(ctx context.Context, projectID, actorID uint, instruction *models.ProjectPaymentInstruction) error {
	if _, err := s.projects.FindByID(ctx, projectID); err != nil {
		return ErrNotFound
	}
	if err := validateInstruction(instruction); err != nil {
		return err
	}
	if instruction.Position == 0 {
		next, err := s.instructions.NextPosition(ctx, projectID)
		if err != nil {
			return err
		}
		instruction.Position = next
	}
	instruction.ProjectID = projectID
	if err := s.instructions.Create(ctx, instruction); err != nil {
		return err
	}
	logIgnoredError("audit instruction create", s.audit.Log(ctx, actorID, "CREATE", "ProjectPaymentInstruction", instruction.ID,
		fmt.Sprintf("Instrucción de pago creada para el proyecto %d", projectID), "", ""))
	return nil
}

// Update modifies an instruction, validating ownership and fields.
func (s *ProjectPaymentInstructionService) Update(ctx context.Context, projectID, id, actorID uint, instruction *models.ProjectPaymentInstruction) (*models.ProjectPaymentInstruction, error) {
	current, err := s.instructions.FindByID(ctx, id)
	if err != nil || current.ProjectID != projectID {
		return nil, ErrNotFound
	}
	if instruction.BankName != "" {
		current.BankName = instruction.BankName
	}
	if instruction.BeneficiaryName != "" {
		current.BeneficiaryName = instruction.BeneficiaryName
	}
	if instruction.AccountNumber != "" {
		current.AccountNumber = instruction.AccountNumber
	}
	if instruction.AccountType != "" {
		current.AccountType = instruction.AccountType
	}
	if instruction.Notes != nil {
		current.Notes = instruction.Notes
	}
	if instruction.Position >= 0 {
		current.Position = instruction.Position
	}
	if err := validateInstruction(current); err != nil {
		return nil, err
	}
	if err := s.instructions.Update(ctx, current); err != nil {
		return nil, err
	}
	logIgnoredError("audit instruction update", s.audit.Log(ctx, actorID, "UPDATE", "ProjectPaymentInstruction", current.ID,
		fmt.Sprintf("Instrucción de pago actualizada en el proyecto %d", projectID), "", ""))
	return current, nil
}

// Delete removes an instruction.
func (s *ProjectPaymentInstructionService) Delete(ctx context.Context, projectID, id, actorID uint) error {
	current, err := s.instructions.FindByID(ctx, id)
	if err != nil || current.ProjectID != projectID {
		return ErrNotFound
	}
	if err := s.instructions.Delete(ctx, id); err != nil {
		return err
	}
	logIgnoredError("audit instruction delete", s.audit.Log(ctx, actorID, "DELETE", "ProjectPaymentInstruction", id,
		fmt.Sprintf("Instrucción de pago eliminada del proyecto %d", projectID), "", ""))
	return nil
}

func validateInstruction(instruction *models.ProjectPaymentInstruction) error {
	instruction.BankName = strings.TrimSpace(instruction.BankName)
	instruction.BeneficiaryName = strings.TrimSpace(instruction.BeneficiaryName)
	instruction.AccountNumber = strings.TrimSpace(instruction.AccountNumber)
	if instruction.BankName == "" || instruction.BeneficiaryName == "" || instruction.AccountNumber == "" {
		return ErrInstructionInvalidData
	}
	if instruction.AccountType == "" {
		instruction.AccountType = models.PaymentAccountTypeSavings
	}
	if instruction.AccountType != models.PaymentAccountTypeSavings && instruction.AccountType != models.PaymentAccountTypeChecking {
		return ErrInstructionInvalidType
	}
	return nil
}
