package services

import (
	"bytes"
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var reportPDFPagePattern = regexp.MustCompile(`/Type\s*/Page\b`)

func reportPDFPageCount(content []byte) int {
	return len(reportPDFPagePattern.FindAllIndex(content, -1))
}

func contractPDFTestContract() *models.Contract {
	amount := 1125000.0
	reserve := 5000.0
	downPayment := 120000.0
	balance := -1000000.0
	measureUnit := "V2"
	address := "Barrio El Centro, Puerto Cortés"
	lotAddress := "Etapa 2, manzana C"
	registration := "ABC-123"
	north := "Lote 4"
	south := "Calle principal"
	east := "Lote 6"
	west := "Área verde"
	now := time.Now()

	return &models.Contract{
		ID:            101,
		GUID:          "a1b2c3d4-9b3a-4a1f-8f2e-1234567890ab",
		Status:        models.ContractStatusApproved,
		FinancingType: models.FinancingTypeDirect,
		PaymentTerm:   84,
		Currency:      "HNL",
		Amount:        &amount,
		ReserveAmount: &reserve,
		DownPayment:   &downPayment,
		Balance:       &balance,
		CreatedAt:     now,
		ApplicantUser: models.User{
			ID:       10,
			FullName: "José Ramón Núñez",
			Identity: "0801-1990-12345",
			Phone:    "9999-9999",
			Email:    "jose.nunez@example.com",
			Address:  &address,
		},
		Lot: models.Lot{
			ID:                 50,
			Name:               "Lote 5",
			Length:             20,
			Width:              10,
			Price:              1125000,
			Address:            &lotAddress,
			RegistrationNumber: &registration,
			MeasurementUnit:    &measureUnit,
			North:              &north,
			South:              &south,
			East:               &east,
			West:               &west,
			Project: models.Project{
				ID:              5,
				Name:            "Residencial Los Álamos",
				Address:         "Puerto Cortés, Cortés",
				MeasurementUnit: "V2",
				InterestRate:    12.5,
			},
		},
		Payments: []models.Payment{
			{
				ID:          1,
				PaymentType: models.PaymentTypeReservation,
				Status:      models.PaymentStatusPaid,
				Amount:      reserve,
				PaidAmount:  &reserve,
				DueDate:     now.AddDate(0, -3, 0),
			},
			{
				ID:          2,
				PaymentType: models.PaymentTypeInstallment,
				Status:      models.PaymentStatusPaid,
				Amount:      11358,
				PaidAmount:  func() *float64 { v := 11358.0; return &v }(),
				DueDate:     now.AddDate(0, -1, 0),
			},
			{
				ID:          3,
				PaymentType: models.PaymentTypeInstallment,
				Status:      models.PaymentStatusPending,
				Amount:      11358,
				DueDate:     now.AddDate(0, 0, -5),
			},
			{
				ID:          4,
				PaymentType: models.PaymentTypeInstallment,
				Status:      models.PaymentStatusPending,
				Amount:      11358,
				DueDate:     now.AddDate(0, 1, 0),
			},
			{
				ID:          5,
				PaymentType: models.PaymentTypeInstallment,
				Status:      models.PaymentStatusPending,
				Amount:      11358,
				DueDate:     now.AddDate(0, 2, 0),
			},
		},
	}
}

func TestPrepareContractPDFData(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)

	data := service.prepareContractPDFData(context.Background(), contractPDFTestContract())

	assert.Equal(t, "#101", data.Reference)
	assert.Equal(t, "Aprobado", data.Status)
	assert.Equal(t, "paid", data.StatusKey)
	assert.Equal(t, "José Ramón Núñez", data.Client.Name)
	assert.Equal(t, "0801-1990-12345", data.Client.Identity)
	assert.Equal(t, "jose.nunez@example.com", data.Client.Email)
	assert.Equal(t, "Residencial Los Álamos", data.Property.ProjectName)
	assert.Equal(t, "Lote 5", data.Property.LotName)
	assert.Equal(t, "200.00 v2", data.Property.Area)
	assert.Equal(t, "10.00 m × 20.00 m", data.Property.Dimensions)
	assert.Equal(t, "ABC-123", data.Property.Registration)
	assert.Equal(t, "L 1,125,000.00", data.Financial.Amount)
	assert.Equal(t, "L 5,000.00", data.Financial.Reserve)
	assert.Equal(t, "L 120,000.00", data.Financial.DownPayment)
	assert.Equal(t, "L 1,000,000.00", data.Financial.Financed)
	assert.Equal(t, "L 1,000,000.00", data.Financial.Balance)
	assert.Equal(t, "L 11,358.00", data.Financial.Monthly)
	assert.Equal(t, "84 meses", data.Financial.PaymentTerm)
	assert.Equal(t, 4, data.Financial.InstallmentCount)
	assert.Equal(t, "12.50", data.Financial.InterestRate)
	assert.Equal(t, "12.50", data.Financial.MoratoryRate)
	assert.Equal(t, "SALDO PENDIENTE", data.Financial.BalanceLabel)
	assert.Len(t, data.Payments, 5)
	assert.Equal(t, "Reservación", data.Payments[0].Concept)
	assert.Equal(t, "Pagado", data.Payments[0].Status)
	assert.Equal(t, "Vencida", data.Payments[2].Status)
	assert.Equal(t, "overdue", data.Payments[2].StatusKey)
	assert.Equal(t, "L 11,358.00", data.Financial.NextPayment)
	assert.Contains(t, data.Financial.NextPaymentDate, "vencida")
}

func TestBuildContractPDFGeneratesValidPDF(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	data := service.prepareContractPDFData(context.Background(), contractPDFTestContract())

	buf, err := buildContractPDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)

	content := buf.Bytes()
	assert.True(t, bytes.HasPrefix(content, []byte("%PDF-")), "output should be a PDF")
	assert.Greater(t, len(content), 1024, "PDF should not be tiny")
	assert.GreaterOrEqual(t, reportPDFPageCount(content), 1, "PDF should contain at least one page")
}

func TestBuildContractPDFLongScheduleUsesMultiplePages(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	contract := contractPDFTestContract()
	contract.Payments = nil

	now := time.Now()
	for index := 0; index < 80; index++ {
		contract.Payments = append(contract.Payments, models.Payment{
			ID:          uint(index + 1),
			PaymentType: models.PaymentTypeInstallment,
			Status:      models.PaymentStatusPending,
			Amount:      11358,
			DueDate:     now.AddDate(0, index, 0),
		})
	}

	data := service.prepareContractPDFData(context.Background(), contract)
	buf, err := buildContractPDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)

	content := buf.Bytes()
	assert.True(t, bytes.HasPrefix(content, []byte("%PDF-")))
	assert.Greater(t, reportPDFPageCount(content), 1, "long schedules should span multiple pages")
}

func TestGenerateContractPDFWithoutExternalBinary(t *testing.T) {
	contractRepo := &mockContractRepository{
		mockFindByIDWithDetails: func(ctx context.Context, id uint) (*models.Contract, error) {
			return contractPDFTestContract(), nil
		},
	}
	service := NewReportService(nil, contractRepo, nil, nil)

	buf, err := service.GenerateContractPDF(context.Background(), 101)
	require.NoError(t, err)
	require.NotNil(t, buf)
	assert.True(t, bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")))
	assert.Greater(t, buf.Len(), 1024)
}

func TestBuildContractPDFHandlesEmptyContract(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	data := service.prepareContractPDFData(context.Background(), &models.Contract{})

	buf, err := buildContractPDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)
	assert.True(t, bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")))
	assert.Greater(t, buf.Len(), 1024)
}

func TestPrepareContractPDFDataMarksCreditBalance(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	contract := contractPDFTestContract()
	credit := 2500.0
	contract.Balance = &credit

	data := service.prepareContractPDFData(context.Background(), contract)

	assert.Equal(t, "SALDO A FAVOR", data.Financial.BalanceLabel)
	assert.Equal(t, "L 2,500.00", data.Financial.Balance)
}

type stubInstructionRepository struct {
	repository.ProjectPaymentInstructionRepository
	instructions  []models.ProjectPaymentInstruction
	err           error
	lastProjectID uint
}

func (s *stubInstructionRepository) ListByProject(ctx context.Context, projectID uint) ([]models.ProjectPaymentInstruction, error) {
	s.lastProjectID = projectID
	return s.instructions, s.err
}

func TestPrepareContractPDFDataUsesDefaultProjectAccount(t *testing.T) {
	instructionRepo := &stubInstructionRepository{
		instructions: []models.ProjectPaymentInstruction{
			{
				ID:              1,
				ProjectID:       5,
				BankName:        "Banco Uno",
				BeneficiaryName: "Inversiones FAMA",
				AccountNumber:   "111-111",
				AccountType:     models.PaymentAccountTypeSavings,
				Currency:        "HNL",
				Position:        1,
			},
			{
				ID:              2,
				ProjectID:       5,
				BankName:        "Banco Dos",
				BeneficiaryName: "Inversiones FAMA",
				AccountNumber:   "222-222",
				AccountType:     models.PaymentAccountTypeChecking,
				Currency:        "HNL",
				Position:        2,
				IsDefault:       true,
			},
		},
	}
	service := NewReportService(nil, nil, nil, instructionRepo)

	data := service.prepareContractPDFData(context.Background(), contractPDFTestContract())

	assert.Equal(t, uint(5), instructionRepo.lastProjectID)
	assert.Equal(t, "222-222", data.BankAccount.AccountNumber)
	assert.Equal(t, "Banco Dos", data.BankAccount.BankName)
	assert.Equal(t, "Cheque", data.BankAccount.AccountType)
	assert.Equal(t, "Inversiones FAMA", data.BankAccount.Beneficiary)
	assert.Equal(t, "HNL", data.BankAccount.Currency)
	assert.True(t, data.BankAccount.HasAccount())

	phrase := bankAccountPhrase(data.BankAccount)
	assert.Contains(t, phrase, "a la cuenta 222-222 (Cheque, HNL) de Banco Dos")
	assert.Contains(t, phrase, "a nombre de Inversiones FAMA")
}

func TestPrepareContractPDFDataFallsBackToFirstProjectAccount(t *testing.T) {
	instructionRepo := &stubInstructionRepository{
		instructions: []models.ProjectPaymentInstruction{
			{
				ID:              10,
				ProjectID:       5,
				BankName:        "Banco Primero",
				BeneficiaryName: "Inversiones FAMA",
				AccountNumber:   "000-001",
				AccountType:     models.PaymentAccountTypeSavings,
				Currency:        "HNL",
				Position:        1,
			},
			{
				ID:              11,
				ProjectID:       5,
				BankName:        "Banco Segundo",
				BeneficiaryName: "Inversiones FAMA",
				AccountNumber:   "000-002",
				AccountType:     models.PaymentAccountTypeSavings,
				Currency:        "HNL",
				Position:        2,
			},
		},
	}
	service := NewReportService(nil, nil, nil, instructionRepo)

	data := service.prepareContractPDFData(context.Background(), contractPDFTestContract())

	assert.Equal(t, "000-001", data.BankAccount.AccountNumber)
	assert.Equal(t, "Banco Primero", data.BankAccount.BankName)
}

func TestPrepareContractPDFDataWithoutProjectAccountsUsesFallback(t *testing.T) {
	service := NewReportService(nil, nil, nil, &stubInstructionRepository{})

	data := service.prepareContractPDFData(context.Background(), contractPDFTestContract())

	assert.False(t, data.BankAccount.HasAccount())
	assert.Equal(t, "", data.BankAccount.AccountNumber)
	assert.Equal(t, "a la cuenta indicada en las instrucciones de pago del proyecto", bankAccountPhrase(data.BankAccount))
}
