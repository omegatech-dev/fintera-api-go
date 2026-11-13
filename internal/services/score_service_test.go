package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
)

func TestScoreServiceRecomputeAll(t *testing.T) {
	good := testSeller(1, models.ScoreLevelHigh)
	noHistory := &models.User{ID: 2, FullName: "New Seller", Role: models.RoleSeller, Status: models.StatusActive, Score: 40, ScoreLevel: models.ScoreLevelLow}
	overridden := 55
	overriddenSeller := &models.User{
		ID:            3,
		FullName:      "Overridden",
		Role:          models.RoleSeller,
		Status:        models.StatusActive,
		Score:         55,
		ScoreLevel:    models.ScoreLevelLow,
		ScoreOverride: &overridden,
	}
	sellers := newFakeSellerRepo(good, noHistory, overriddenSeller)
	assignments := newFakeAssignmentRepo()
	assignments.outcomes = []models.ScoreOutcome{
		{SellerID: good.ID, Accepted: 12, Expired: 1},
		{SellerID: overriddenSeller.ID, Accepted: 0, Expired: 10},
	}
	service := NewScoreService(sellers, assignments, nil)

	if err := service.RecomputeAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	if update, ok := sellers.scoreUpdates[good.ID]; !ok || update.Score != 92 || update.Level != models.ScoreLevelHigh {
		t.Fatalf("good seller update = %+v", update)
	}
	if update, ok := sellers.scoreUpdates[noHistory.ID]; !ok || update.Score != 100 || update.Level != models.ScoreLevelHigh {
		t.Fatalf("no-history seller update = %+v", update)
	}
	if _, ok := sellers.scoreUpdates[overriddenSeller.ID]; ok {
		t.Fatal("an active override must not be overwritten by the nightly job")
	}
	if overriddenSeller.Score != 55 {
		t.Fatalf("overridden score = %d", overriddenSeller.Score)
	}
}

func TestScoreServiceOverride(t *testing.T) {
	seller := testSeller(1, models.ScoreLevelHigh)
	sellers := newFakeSellerRepo(seller)
	service := NewScoreService(sellers, newFakeAssignmentRepo(), nil)
	service.now = func() time.Time { return assignmentTestNow }

	updated, err := service.Override(context.Background(), seller.ID, 70, "Acuerdo comercial", 9, "127.0.0.1", "agent")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Score != 70 || updated.ScoreLevel != models.ScoreLevelMedium {
		t.Fatalf("updated = %d/%q", updated.Score, updated.ScoreLevel)
	}
	record := sellers.overrides[seller.ID]
	if record.Score != 70 || record.Reason != "Acuerdo comercial" || record.Actor != 9 {
		t.Fatalf("override record = %+v", record)
	}
	if updated.ScoreOverrideAt == nil || !updated.ScoreOverrideAt.Equal(assignmentTestNow) {
		t.Fatalf("override at = %v", updated.ScoreOverrideAt)
	}
}

func TestScoreServiceOverrideValidation(t *testing.T) {
	seller := testSeller(1, models.ScoreLevelHigh)
	user := &models.User{ID: 2, FullName: "Client", Role: models.RoleUser, Status: models.StatusActive}
	sellers := newFakeSellerRepo(seller, user)
	service := NewScoreService(sellers, newFakeAssignmentRepo(), nil)

	if _, err := service.Override(context.Background(), seller.ID, 101, "reason", 9, "", ""); !errors.Is(err, ErrScoreOverrideInvalid) {
		t.Fatalf("out of range err = %v", err)
	}
	if _, err := service.Override(context.Background(), seller.ID, 80, "   ", 9, "", ""); !errors.Is(err, ErrScoreOverrideInvalid) {
		t.Fatalf("empty reason err = %v", err)
	}
	if _, err := service.Override(context.Background(), user.ID, 80, "reason", 9, "", ""); !errors.Is(err, ErrScoreOverrideInvalid) {
		t.Fatalf("non-seller err = %v", err)
	}
	if _, err := service.Override(context.Background(), 999, 80, "reason", 9, "", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing seller err = %v", err)
	}
}

func TestScoreServiceClearOverrideRecomputes(t *testing.T) {
	overridden := 60
	seller := testSeller(1, models.ScoreLevelMedium)
	seller.ScoreOverride = &overridden
	sellers := newFakeSellerRepo(seller)
	assignments := newFakeAssignmentRepo()
	assignments.outcomes = []models.ScoreOutcome{{SellerID: seller.ID, Accepted: 9, Expired: 1}}
	service := NewScoreService(sellers, assignments, nil)

	updated, err := service.ClearOverride(context.Background(), seller.ID, 9, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(sellers.cleared) != 1 || sellers.cleared[0] != seller.ID {
		t.Fatalf("cleared = %v", sellers.cleared)
	}
	if updated.Score != 90 || updated.ScoreLevel != models.ScoreLevelHigh {
		t.Fatalf("recomputed = %d/%q", updated.Score, updated.ScoreLevel)
	}
	if updated.ScoreOverride != nil {
		t.Fatal("override must be cleared")
	}
}

func TestScoreServiceMetricsForUsers(t *testing.T) {
	assignments := newFakeAssignmentRepo()
	assignments.outcomes = []models.ScoreOutcome{{SellerID: 7, Accepted: 12, Expired: 1}}
	service := NewScoreService(nil, assignments, nil)

	metrics, err := service.MetricsForUsers(context.Background(), []uint{7, 9})
	if err != nil {
		t.Fatal(err)
	}
	if got := metrics[7]; got.AcceptedCount != 12 || got.ExpiredCount != 1 || got.WindowDays != models.SellerScoreWindowDays {
		t.Fatalf("seller 7 metrics = %+v", got)
	}
	if got := metrics[9]; got.AcceptedCount != 0 || got.ExpiredCount != 0 || got.WindowDays != models.SellerScoreWindowDays {
		t.Fatalf("no-history metrics = %+v", got)
	}
}
