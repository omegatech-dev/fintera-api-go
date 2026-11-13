package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
)

var assignmentTestNow = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

func newTestAssignmentService(repo *fakeAssignmentRepo, sellers ...*models.User) *AssignmentService {
	users := make(map[uint]*models.User, len(sellers))
	for _, seller := range sellers {
		users[seller.ID] = seller
	}
	service := NewAssignmentService(
		repo,
		&fakeContractRepoForAssignment{contracts: map[uint]*models.Contract{}},
		&fakeUserRepoForAssignment{users: users},
		nil,
		nil,
	)
	service.now = func() time.Time { return assignmentTestNow }
	service.randIntn = func(int) int { return 0 }
	return service
}

func testSeller(id uint, level string) *models.User {
	return &models.User{
		ID:         id,
		FullName:   "Seller",
		Role:       models.RoleSeller,
		Status:     models.StatusActive,
		Score:      100,
		ScoreLevel: level,
	}
}

func testContract(id uint) models.Contract {
	return models.Contract{ID: id, Status: models.ContractStatusPending}
}

func TestSelectSellerPrefersHighBeforeMedium(t *testing.T) {
	high := testSeller(1, models.ScoreLevelHigh)
	medium := testSeller(2, models.ScoreLevelMedium)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*medium, *high}
	service := newTestAssignmentService(repo)

	selected, err := service.SelectSeller(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected == nil || selected.ID != high.ID {
		t.Fatalf("selected = %+v", selected)
	}
}

func TestSelectSellerExcludesLowPerformers(t *testing.T) {
	low := testSeller(1, models.ScoreLevelLow)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*low}
	service := newTestAssignmentService(repo)

	selected, err := service.SelectSeller(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected != nil {
		t.Fatalf("low seller must be excluded, got %+v", selected)
	}
}

func TestSelectSellerPrefersLowestPendingLoad(t *testing.T) {
	busy := testSeller(1, models.ScoreLevelHigh)
	free := testSeller(2, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*busy, *free}
	repo.loads[busy.ID] = 3
	repo.loads[free.ID] = 0
	service := newTestAssignmentService(repo)

	selected, err := service.SelectSeller(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected == nil || selected.ID != free.ID {
		t.Fatalf("selected = %+v", selected)
	}
}

func TestSelectSellerRotatesByLastAssignedAt(t *testing.T) {
	recent := testSeller(1, models.ScoreLevelHigh)
	never := testSeller(2, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*recent, *never}
	earlier := assignmentTestNow.Add(-time.Hour)
	repo.last[recent.ID] = &earlier
	service := newTestAssignmentService(repo)

	selected, err := service.SelectSeller(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected == nil || selected.ID != never.ID {
		t.Fatalf("never-assigned seller should win, got %+v", selected)
	}

	// After the newer seller also receives an offer, the earlier one rotates in.
	repo.last[never.ID] = &assignmentTestNow
	selected, err = service.SelectSeller(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected == nil || selected.ID != recent.ID {
		t.Fatalf("rotation should pick the least recently assigned seller, got %+v", selected)
	}
}

func TestSelectSellerExcludesExpiredForContract(t *testing.T) {
	expiredSeller := testSeller(1, models.ScoreLevelHigh)
	other := testSeller(2, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*expiredSeller, *other}
	repo.assignments[99] = &models.Assignment{
		ID:         99,
		ContractID: 10,
		SellerID:   expiredSeller.ID,
		Status:     models.AssignmentStatusExpired,
	}
	service := newTestAssignmentService(repo)

	selected, err := service.SelectSeller(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected == nil || selected.ID != other.ID {
		t.Fatalf("expired seller must be skipped, got %+v", selected)
	}
}

func TestSelectSellerRandomTiebreak(t *testing.T) {
	first := testSeller(1, models.ScoreLevelHigh)
	second := testSeller(2, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*first, *second}
	service := newTestAssignmentService(repo)
	service.randIntn = func(n int) int {
		if n != 2 {
			t.Fatalf("tie group size = %d, want 2", n)
		}
		return 1
	}

	selected, err := service.SelectSeller(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected == nil || selected.ID != second.ID {
		t.Fatalf("random tiebreak should pick the second candidate, got %+v", selected)
	}
}

func TestApplyOnCreateAutomaticAssignsPendingOffer(t *testing.T) {
	seller := testSeller(1, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*seller}
	service := newTestAssignmentService(repo, seller)

	contract := testContract(10)
	contract.AssignmentOrigin = models.AssignmentOriginAutomatic
	contract.AssignmentStatus = models.AssignmentStatusNone

	if err := service.ApplyOnCreate(context.Background(), &contract, assignmentTestNow); err != nil {
		t.Fatal(err)
	}
	if len(repo.assignments) != 1 {
		t.Fatalf("assignments = %d", len(repo.assignments))
	}
	assignment := repo.assignments[1]
	if assignment.SellerID != seller.ID || assignment.Status != models.AssignmentStatusPending {
		t.Fatalf("assignment = %+v", assignment)
	}
	wantDeadline := assignmentTestNow.Add(models.DefaultAssignmentAcceptWindow)
	if assignment.AcceptDeadlineAt == nil || !assignment.AcceptDeadlineAt.Equal(wantDeadline) {
		t.Fatalf("deadline = %v", assignment.AcceptDeadlineAt)
	}
}

func TestApplyOnCreateLeavesReservationWithoutSellerWhenPoolEmpty(t *testing.T) {
	repo := newFakeAssignmentRepo()
	service := newTestAssignmentService(repo)

	contract := testContract(10)
	contract.AssignmentOrigin = models.AssignmentOriginAutomatic
	contract.AssignmentStatus = models.AssignmentStatusNone

	if err := service.ApplyOnCreate(context.Background(), &contract, assignmentTestNow); err != nil {
		t.Fatal(err)
	}
	if len(repo.assignments) != 0 {
		t.Fatalf("assignments = %d", len(repo.assignments))
	}
	if len(repo.contractUpdates) != 1 {
		t.Fatalf("contract updates = %d", len(repo.contractUpdates))
	}
	update := repo.contractUpdates[0].Update
	if update.SellerID != nil || update.Status != models.AssignmentStatusNone {
		t.Fatalf("update = %+v", update)
	}
}

func TestApplyOnCreateReferralCreatesAcceptedAssignment(t *testing.T) {
	seller := testSeller(1, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	service := newTestAssignmentService(repo, seller)

	now := assignmentTestNow
	contract := testContract(10)
	contract.SellerID = &seller.ID
	contract.AssignmentOrigin = models.AssignmentOriginReferral
	contract.AssignmentStatus = models.AssignmentStatusAccepted
	contract.AssignedAt = &now
	contract.RespondedAt = &now

	if err := service.ApplyOnCreate(context.Background(), &contract, now); err != nil {
		t.Fatal(err)
	}
	assignment := repo.assignments[1]
	if assignment == nil {
		t.Fatal("expected an accepted assignment row")
	}
	if assignment.Status != models.AssignmentStatusAccepted || assignment.AcceptDeadlineAt != nil {
		t.Fatalf("assignment = %+v", assignment)
	}
}

func TestAcceptIsIdempotentAndRejectsOtherSellers(t *testing.T) {
	seller := testSeller(1, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	contract := testContract(10)
	repo.assignments[1] = &models.Assignment{
		ID:         1,
		ContractID: 10,
		SellerID:   seller.ID,
		Origin:     models.AssignmentOriginAutomatic,
		Status:     models.AssignmentStatusPending,
		AssignedAt: assignmentTestNow.Add(-time.Minute),
		Contract:   contract,
	}
	service := newTestAssignmentService(repo, seller)

	if _, err := service.Accept(context.Background(), 1, 99, false); !errors.Is(err, ErrAssignmentNotOwned) {
		t.Fatalf("wrong seller err = %v", err)
	}
	if repo.assignments[1].Status != models.AssignmentStatusPending {
		t.Fatal("wrong seller must not change the state")
	}

	accepted, err := service.Accept(context.Background(), 1, seller.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != models.AssignmentStatusAccepted {
		t.Fatalf("status = %q", accepted.Status)
	}
	if _, err := service.Accept(context.Background(), 1, seller.ID, false); err != nil {
		t.Fatalf("idempotent accept: %v", err)
	}
	if repo.statusUpdates != 1 {
		t.Fatalf("status updates = %d, want 1", repo.statusUpdates)
	}
	lastUpdate := repo.contractUpdates[len(repo.contractUpdates)-1].Update
	if lastUpdate.Status != models.AssignmentStatusAccepted || lastUpdate.SellerID == nil || *lastUpdate.SellerID != seller.ID {
		t.Fatalf("contract update = %+v", lastUpdate)
	}
}

func TestAcceptExpiredAssignmentIsRejected(t *testing.T) {
	seller := testSeller(1, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	deadline := assignmentTestNow.Add(-time.Minute)
	repo.assignments[1] = &models.Assignment{
		ID:               1,
		ContractID:       10,
		SellerID:         seller.ID,
		Status:           models.AssignmentStatusPending,
		AcceptDeadlineAt: &deadline,
		Contract:         testContract(10),
	}
	service := newTestAssignmentService(repo, seller)

	if _, err := service.Accept(context.Background(), 1, seller.ID, false); !errors.Is(err, ErrAssignmentExpired) {
		t.Fatalf("err = %v", err)
	}
	if repo.assignments[1].Status != models.AssignmentStatusExpired {
		t.Fatalf("status = %q", repo.assignments[1].Status)
	}
}

func TestRejectExpiresAndReassignsToNextSeller(t *testing.T) {
	first := testSeller(1, models.ScoreLevelHigh)
	second := testSeller(2, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*first, *second}
	repo.assignments[1] = &models.Assignment{
		ID:         1,
		ContractID: 10,
		SellerID:   first.ID,
		Origin:     models.AssignmentOriginManual,
		Status:     models.AssignmentStatusPending,
		AssignedAt: assignmentTestNow.Add(-time.Minute),
		Contract:   testContract(10),
	}
	service := newTestAssignmentService(repo, first, second)

	rejected, err := service.Reject(context.Background(), 1, first.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != models.AssignmentStatusExpired {
		t.Fatalf("status = %q", rejected.Status)
	}
	if len(repo.assignments) != 2 {
		t.Fatalf("assignments = %d", len(repo.assignments))
	}
	next := repo.assignments[2]
	if next.SellerID != second.ID || next.Status != models.AssignmentStatusPending {
		t.Fatalf("next assignment = %+v", next)
	}

	// Repeating the rejection is a no-op.
	if _, err := service.Reject(context.Background(), 1, first.ID, false); err != nil {
		t.Fatalf("idempotent reject: %v", err)
	}
	if len(repo.assignments) != 2 {
		t.Fatalf("repeated reject created extra assignments")
	}
}

func TestExpireOverdueReassignsAndStopsWhenPoolIsExhausted(t *testing.T) {
	expiredSeller := testSeller(1, models.ScoreLevelHigh)
	nextSeller := testSeller(2, models.ScoreLevelHigh)
	repo := newFakeAssignmentRepo()
	repo.assignable = []models.User{*expiredSeller}
	deadline := assignmentTestNow.Add(-time.Minute)
	repo.assignments[1] = &models.Assignment{
		ID:               1,
		ContractID:       10,
		SellerID:         expiredSeller.ID,
		Origin:           models.AssignmentOriginAutomatic,
		Status:           models.AssignmentStatusPending,
		AcceptDeadlineAt: &deadline,
		Contract:         testContract(10),
	}
	service := newTestAssignmentService(repo, expiredSeller, nextSeller)

	if err := service.ExpireOverdue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.assignments[1].Status != models.AssignmentStatusExpired {
		t.Fatalf("status = %q", repo.assignments[1].Status)
	}
	if len(repo.assignments) != 1 {
		t.Fatalf("pool should be exhausted, got %d assignments", len(repo.assignments))
	}
	last := repo.contractUpdates[len(repo.contractUpdates)-1].Update
	if last.SellerID != nil || last.Status != models.AssignmentStatusNone {
		t.Fatalf("contract must stay without a seller, update = %+v", last)
	}
	if repo.assignments[1].Contract.Status == models.ContractStatusCancelled {
		t.Fatal("the reservation must never be cancelled")
	}

	// With a second candidate the job creates the next pending offer.
	repo.assignable = []models.User{*expiredSeller, *nextSeller}
	repo.assignments[2] = &models.Assignment{
		ID:               2,
		ContractID:       11,
		SellerID:         expiredSeller.ID,
		Status:           models.AssignmentStatusPending,
		AcceptDeadlineAt: &deadline,
		Contract:         testContract(11),
	}
	if err := service.ExpireOverdue(context.Background()); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, assignment := range repo.assignments {
		if assignment.ContractID == 11 && assignment.Status == models.AssignmentStatusPending && assignment.SellerID == nextSeller.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected the reservation to be reassigned to the next seller")
	}
}

func TestValidateSeller(t *testing.T) {
	seller := testSeller(1, models.ScoreLevelHigh)
	inactive := testSeller(2, models.ScoreLevelHigh)
	inactive.Status = models.StatusInactive
	service := newTestAssignmentService(newFakeAssignmentRepo(), seller, inactive)

	if _, err := service.ValidateSeller(context.Background(), seller.ID); err != nil {
		t.Fatalf("active seller: %v", err)
	}
	if _, err := service.ValidateSeller(context.Background(), inactive.ID); !errors.Is(err, ErrAssignmentSellerInactive) {
		t.Fatalf("inactive seller err = %v", err)
	}
	if _, err := service.ValidateSeller(context.Background(), 999); !errors.Is(err, ErrAssignmentSellerInactive) {
		t.Fatalf("missing seller err = %v", err)
	}
}

func TestContractAssignmentInputOrigins(t *testing.T) {
	if normalizeAssignmentOrigin("MANUAL") != models.AssignmentOriginManual {
		t.Fatal("manual origin must be normalized")
	}
	if normalizeAssignmentOrigin("referral") != models.AssignmentOriginReferral {
		t.Fatal("referral origin must be kept")
	}
	if normalizeAssignmentOrigin("bogus") != "" {
		t.Fatal("unknown origin must be discarded")
	}
}
