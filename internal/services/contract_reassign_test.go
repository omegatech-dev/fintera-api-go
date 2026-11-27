package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func reassignContractFixture(currentSellerID uint) *models.Contract {
	amount := 100000.0
	contract := &models.Contract{
		ID:               10,
		GUID:             "contract-1",
		Status:           models.ContractStatusPending,
		FinancingType:    models.FinancingTypeDirect,
		Amount:           &amount,
		AssignmentOrigin: models.AssignmentOriginAutomatic,
		AssignmentStatus: models.AssignmentStatusAccepted,
	}
	if currentSellerID > 0 {
		contract.SellerID = &currentSellerID
	}
	return contract
}

func configureReassignTransaction(service *ContractService) {
	repos := &repository.Repositories{
		Contract:   service.repo,
		Assignment: service.assignmentSvc.repo,
		User:       service.assignmentSvc.userRepo,
	}
	if service.commissionSvc != nil {
		repos.Commission = service.commissionSvc.repo
	}
	service.SetTransactionRunner(func(ctx context.Context, fn func(*repository.Repositories) error) error {
		return fn(repos)
	})
}

func configureSnapshotReassignTransaction(
	service *ContractService,
	contract *models.Contract,
	commissionRepo *fakeCommissionRepository,
	assignmentRepo *fakeAssignmentRepo,
	repos *repository.Repositories,
) {
	service.SetTransactionRunner(func(_ context.Context, fn func(*repository.Repositories) error) error {
		contractBefore := *contract
		creditsBefore := make(map[uint]models.SellerCommissionCredit, len(commissionRepo.credits))
		for id, row := range commissionRepo.credits {
			creditsBefore[id] = *row
		}
		applicationsBefore := make(map[uint][]models.CreditApplication, len(commissionRepo.applications))
		for id, rows := range commissionRepo.applications {
			applicationsBefore[id] = append([]models.CreditApplication(nil), rows...)
		}
		nextCreditIDBefore := commissionRepo.nextCreditID
		assignmentsBefore := make(map[uint]models.Assignment, len(assignmentRepo.assignments))
		for id, row := range assignmentRepo.assignments {
			assignmentsBefore[id] = *row
		}
		nextAssignmentIDBefore := assignmentRepo.nextID

		err := fn(repos)
		if err == nil {
			return nil
		}
		*contract = contractBefore
		commissionRepo.credits = make(map[uint]*models.SellerCommissionCredit, len(creditsBefore))
		for id, row := range creditsBefore {
			copy := row
			commissionRepo.credits[id] = &copy
		}
		commissionRepo.applications = applicationsBefore
		commissionRepo.nextCreditID = nextCreditIDBefore
		assignmentRepo.assignments = make(map[uint]*models.Assignment, len(assignmentsBefore))
		for id, row := range assignmentsBefore {
			copy := row
			assignmentRepo.assignments[id] = &copy
		}
		assignmentRepo.nextID = nextAssignmentIDBefore
		return err
	})
}

func TestReassignAutomaticAvoidsCurrentSeller(t *testing.T) {
	first := testSeller(7, models.ScoreLevelHigh)
	second := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()
	assignmentRepo.assignable = []models.User{*first, *second}

	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(first.ID)
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, first, second),
	}
	configureReassignTransaction(service)

	updated, assignment, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode:           ReassignModeAutomatic,
		ExcludeCurrent: true,
		ActorID:        99,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, assignment)
	require.NotNil(t, updated.SellerID)
	require.Equal(t, second.ID, *updated.SellerID)
	require.Equal(t, models.AssignmentOriginAutomatic, updated.AssignmentOrigin)
	require.Equal(t, models.AssignmentStatusPending, updated.AssignmentStatus)
	contractRepo.AssertExpectations(t)
}

func TestReassignRecoversPaidCommissionFromPreviousSeller(t *testing.T) {
	previous := testSeller(7, models.ScoreLevelHigh)
	next := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()

	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(previous.ID)
	paidAt := time.Now()
	contract.CommissionAmount = 1500
	contract.CommissionPaidAt = &paidAt
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	commissionRepo := newFakeCommissionRepository()
	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, previous, next),
		commissionSvc: NewCommissionService(commissionRepo, nil, nil),
	}
	configureReassignTransaction(service)

	updated, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode:     ReassignModeSeller,
		SellerID: &next.ID,
		Accept:   true,
		ActorID:  99,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.SellerID)
	require.Equal(t, next.ID, *updated.SellerID)
	require.Nil(t, updated.CommissionPaidAt)
	require.Nil(t, updated.CommissionPaidBy)
	require.Len(t, commissionRepo.credits, 1)

	var credit *models.SellerCommissionCredit
	for _, item := range commissionRepo.credits {
		credit = item
	}
	require.NotNil(t, credit)
	require.Equal(t, previous.ID, credit.SellerID)
	require.Equal(t, 1500.0, credit.Amount)
	require.Equal(t, models.CommissionCreditStatusPending, credit.Status)
	contractRepo.AssertExpectations(t)
}

func TestReassignFullyNettedCommissionRestoresCreditWithoutCreatingDuplicate(t *testing.T) {
	previous := testSeller(7, models.ScoreLevelHigh)
	next := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()

	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(previous.ID)
	paidAt := time.Now()
	contract.CommissionAmount = 1500
	contract.CommissionPaidAt = &paidAt
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	commissionRepo := newFakeCommissionRepository()
	credit := &models.SellerCommissionCredit{
		SellerID: previous.ID, ContractID: 3, Amount: 1500,
		AppliedAmount: 1500, Status: models.CommissionCreditStatusSettled,
	}
	require.NoError(t, commissionRepo.CreateCredit(context.Background(), credit))
	commissionRepo.applications[credit.ID] = []models.CreditApplication{{
		ID: 1, CreditID: credit.ID, ContractID: contract.ID, Amount: 1500, CreatedAt: time.Now(),
	}}

	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, previous, next),
		commissionSvc: NewCommissionService(commissionRepo, nil, nil),
	}
	configureReassignTransaction(service)

	updated, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode:     ReassignModeSeller,
		SellerID: &next.ID,
		Accept:   true,
		ActorID:  99,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.SellerID)
	require.Equal(t, next.ID, *updated.SellerID)
	require.Nil(t, updated.CommissionPaidAt)
	require.Nil(t, updated.CommissionPaidBy)
	require.Len(t, commissionRepo.credits, 1, "fully netted payout must not create a zero-value credit")
	require.Equal(t, models.CommissionCreditStatusPending, commissionRepo.credits[credit.ID].Status)
	require.Equal(t, 0.0, commissionRepo.credits[credit.ID].AppliedAmount)
	require.NotNil(t, commissionRepo.applications[credit.ID][0].ReversedAt)
}

func TestReassignPartiallyNettedCommissionCreatesCashRecoveryCredit(t *testing.T) {
	previous := testSeller(7, models.ScoreLevelHigh)
	next := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()
	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(previous.ID)
	paidAt := time.Now()
	contract.CommissionAmount = 1500
	contract.CommissionPaidAt = &paidAt
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	commissionRepo := newFakeCommissionRepository()
	credit := &models.SellerCommissionCredit{
		SellerID: previous.ID, ContractID: 3, Amount: 1000,
		AppliedAmount: 1000, Status: models.CommissionCreditStatusSettled,
	}
	require.NoError(t, commissionRepo.CreateCredit(context.Background(), credit))
	commissionRepo.applications[credit.ID] = []models.CreditApplication{{
		ID: 1, CreditID: credit.ID, ContractID: contract.ID, Amount: 1000, CreatedAt: time.Now(),
	}}
	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, previous, next),
		commissionSvc: NewCommissionService(commissionRepo, nil, nil),
	}
	configureReassignTransaction(service)

	updated, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode: ReassignModeSeller, SellerID: &next.ID, Accept: true, ActorID: 99,
	})
	require.NoError(t, err)
	require.Nil(t, updated.CommissionPaidAt)
	require.Equal(t, next.ID, *updated.SellerID)
	require.Equal(t, 0.0, commissionRepo.credits[credit.ID].AppliedAmount)
	require.Equal(t, models.CommissionCreditStatusPending, commissionRepo.credits[credit.ID].Status)
	require.Len(t, commissionRepo.credits, 2)
	for id, recovery := range commissionRepo.credits {
		if id == credit.ID {
			continue
		}
		require.Equal(t, previous.ID, recovery.SellerID)
		require.Equal(t, 500.0, recovery.Amount)
		require.Equal(t, models.CommissionCreditSourceReassignment, recovery.Source)
	}
	require.NotNil(t, commissionRepo.applications[credit.ID][0].ReversedAt)
}

func TestReassignUnpaidCommissionDoesNotCreateRecoveryCredit(t *testing.T) {
	previous := testSeller(7, models.ScoreLevelHigh)
	next := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()
	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(previous.ID)
	contract.CommissionAmount = 1500
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	commissionRepo := newFakeCommissionRepository()
	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, previous, next),
		commissionSvc: NewCommissionService(commissionRepo, nil, nil),
	}
	configureReassignTransaction(service)

	updated, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode: ReassignModeSeller, SellerID: &next.ID, Accept: true, ActorID: 99,
	})
	require.NoError(t, err)
	require.Nil(t, updated.CommissionPaidAt)
	require.Equal(t, next.ID, *updated.SellerID)
	require.Empty(t, commissionRepo.credits)
}

func TestReassignRollbackRestoresCommissionAndAssignmentState(t *testing.T) {
	previous := testSeller(7, models.ScoreLevelHigh)
	next := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()

	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(previous.ID)
	paidAt := time.Now()
	contract.CommissionAmount = 1500
	contract.CommissionPaidAt = &paidAt
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	commissionRepo := newFakeCommissionRepository()
	credit := &models.SellerCommissionCredit{
		SellerID: previous.ID, ContractID: 3, Amount: 1000,
		AppliedAmount: 1000, Status: models.CommissionCreditStatusSettled,
	}
	require.NoError(t, commissionRepo.CreateCredit(context.Background(), credit))
	commissionRepo.applications[credit.ID] = []models.CreditApplication{{
		ID: 1, CreditID: credit.ID, ContractID: contract.ID,
		Amount: 1000, CreatedAt: time.Now(),
	}}

	assignmentSvc := newTestAssignmentService(assignmentRepo, previous, next)
	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: assignmentSvc,
		commissionSvc: NewCommissionService(commissionRepo, nil, nil),
	}
	repos := &repository.Repositories{
		Contract:   contractRepo,
		Assignment: assignmentRepo,
		User:       assignmentSvc.userRepo,
		Commission: commissionRepo,
	}
	service.SetTransactionRunner(func(ctx context.Context, fn func(*repository.Repositories) error) error {
		contractBefore := *contract
		creditsBefore := make(map[uint]models.SellerCommissionCredit, len(commissionRepo.credits))
		for id, row := range commissionRepo.credits {
			creditsBefore[id] = *row
		}
		applicationsBefore := make(map[uint][]models.CreditApplication, len(commissionRepo.applications))
		for id, rows := range commissionRepo.applications {
			applicationsBefore[id] = append([]models.CreditApplication(nil), rows...)
		}
		nextCreditIDBefore := commissionRepo.nextCreditID
		assignmentBefore := make(map[uint]models.Assignment, len(assignmentRepo.assignments))
		for id, row := range assignmentRepo.assignments {
			assignmentBefore[id] = *row
		}
		nextAssignmentIDBefore := assignmentRepo.nextID

		err := fn(repos)
		if err != nil {
			*contract = contractBefore
			commissionRepo.credits = make(map[uint]*models.SellerCommissionCredit, len(creditsBefore))
			for id, row := range creditsBefore {
				copy := row
				commissionRepo.credits[id] = &copy
			}
			commissionRepo.applications = applicationsBefore
			commissionRepo.nextCreditID = nextCreditIDBefore
			assignmentRepo.assignments = make(map[uint]*models.Assignment, len(assignmentBefore))
			for id, row := range assignmentBefore {
				copy := row
				assignmentRepo.assignments[id] = &copy
			}
			assignmentRepo.nextID = nextAssignmentIDBefore
		}
		return err
	})

	assignmentRepo.createErr = errors.New("injected assignment insert failure")
	_, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode: ReassignModeSeller, SellerID: &next.ID, Accept: true, ActorID: 99,
	})
	require.ErrorContains(t, err, "injected assignment insert failure")
	require.Len(t, commissionRepo.credits, 1)
	require.Equal(t, 1000.0, commissionRepo.credits[credit.ID].AppliedAmount)
	require.Equal(t, models.CommissionCreditStatusSettled, commissionRepo.credits[credit.ID].Status)
	require.Nil(t, commissionRepo.applications[credit.ID][0].ReversedAt)
	require.Equal(t, previous.ID, *contract.SellerID)
	require.Equal(t, &paidAt, contract.CommissionPaidAt)
	require.Empty(t, assignmentRepo.assignments)
}

func TestReassignFailuresRollbackAfterCommissionCompensation(t *testing.T) {
	for _, stage := range []string{"supersede", "offer", "contract-save"} {
		t.Run(stage, func(t *testing.T) {
			previous := testSeller(7, models.ScoreLevelHigh)
			next := testSeller(8, models.ScoreLevelHigh)
			assignmentRepo := newFakeAssignmentRepo()
			contractRepo := &mockContractRepo{}
			contract := reassignContractFixture(previous.ID)
			contract.Status = models.ContractStatusPending
			if stage == "contract-save" {
				contract.Status = models.ContractStatusApproved
			}
			paidAt := time.Now()
			contract.CommissionAmount = 1500
			contract.CommissionPaidAt = &paidAt
			contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
			contractRepo.On("Update", mock.Anything, mock.Anything).Return(errors.New("injected contract save failure"))

			commissionRepo := newFakeCommissionRepository()
			credit := &models.SellerCommissionCredit{
				SellerID: previous.ID, ContractID: 3, Amount: 1000,
				AppliedAmount: 1000, Status: models.CommissionCreditStatusSettled,
			}
			require.NoError(t, commissionRepo.CreateCredit(context.Background(), credit))
			commissionRepo.applications[credit.ID] = []models.CreditApplication{{
				ID: 1, CreditID: credit.ID, ContractID: contract.ID, Amount: 1000, CreatedAt: time.Now(),
			}}

			assignmentSvc := newTestAssignmentService(assignmentRepo, previous, next)
			service := &ContractService{
				repo: contractRepo, assignmentSvc: assignmentSvc,
				commissionSvc: NewCommissionService(commissionRepo, nil, nil),
			}
			repos := &repository.Repositories{
				Contract: contractRepo, Assignment: assignmentRepo,
				User: assignmentSvc.userRepo, Commission: commissionRepo,
			}
			configureSnapshotReassignTransaction(service, contract, commissionRepo, assignmentRepo, repos)

			switch stage {
			case "supersede":
				assignmentRepo.pendingErr = errors.New("injected supersede lookup failure")
			case "offer":
				assignmentRepo.createErr = errors.New("injected offer insert failure")
			case "contract-save":
				// Approved contracts are assigned immediately; the final contract
				// save fails after compensation and assignment-side audit writes.
			}

			accept := stage != "offer"
			_, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
				Mode: ReassignModeSeller, SellerID: &next.ID, Accept: accept, ActorID: 99,
			})
			require.Error(t, err)
			require.Len(t, commissionRepo.credits, 1)
			require.Equal(t, 1000.0, commissionRepo.credits[credit.ID].AppliedAmount)
			require.Equal(t, models.CommissionCreditStatusSettled, commissionRepo.credits[credit.ID].Status)
			require.Nil(t, commissionRepo.applications[credit.ID][0].ReversedAt)
			require.Equal(t, previous.ID, *contract.SellerID)
			require.Equal(t, &paidAt, contract.CommissionPaidAt)
			require.Empty(t, assignmentRepo.assignments)
		})
	}
}

func TestReassignCancelledContractDoesNotDuplicateCredit(t *testing.T) {
	previous := testSeller(7, models.ScoreLevelHigh)
	next := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()

	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(previous.ID)
	contract.Status = models.ContractStatusCancelled
	paidAt := time.Now()
	contract.CommissionAmount = 1500
	contract.CommissionPaidAt = &paidAt
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	commissionRepo := newFakeCommissionRepository()
	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, previous, next),
		commissionSvc: NewCommissionService(commissionRepo, nil, nil),
	}
	configureReassignTransaction(service)

	updated, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode:     ReassignModeSeller,
		SellerID: &next.ID,
		Accept:   true,
		ActorID:  99,
	})
	require.NoError(t, err)
	require.Empty(t, commissionRepo.credits, "cancellation already created the recoverable credit")
	require.NotNil(t, updated.CommissionPaidAt)
	require.Equal(t, next.ID, *updated.SellerID)
}
