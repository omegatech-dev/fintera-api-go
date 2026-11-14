package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sjperalta/fintera-api/internal/models"
)

func TestSellerServiceEnsureReferralGUID(t *testing.T) {
	seller := &models.User{
		ID:       5,
		FullName: "José Ángel Núñez",
		Email:    "jose@example.com",
		Role:     models.RoleSeller,
		Status:   models.StatusActive,
	}
	sellers := newFakeSellerRepo(seller)
	service := NewSellerService(sellers, newFakeAssignmentRepo())

	if err := service.EnsureReferralGUID(context.Background(), seller); err != nil {
		t.Fatal(err)
	}
	if seller.ReferralGUID == nil {
		t.Fatal("expected a referral GUID")
	}
	parsed, err := uuid.Parse(*seller.ReferralGUID)
	if err != nil {
		t.Fatalf("referral GUID %q is not a UUID: %v", *seller.ReferralGUID, err)
	}
	if parsed.Version() != 4 {
		t.Fatalf("referral GUID version = %d, want 4", parsed.Version())
	}
	first := *seller.ReferralGUID

	// Calling it again keeps the existing GUID.
	if err := service.EnsureReferralGUID(context.Background(), seller); err != nil {
		t.Fatal(err)
	}
	if got := sellers.updatedGUIDs[seller.ID]; got != first {
		t.Fatalf("updated GUID = %q, want %q", got, first)
	}
	if *seller.ReferralGUID != first {
		t.Fatalf("referral GUID changed to %q", *seller.ReferralGUID)
	}
}

func TestSellerServiceEnsureReferralGUIDSkipsNonSellers(t *testing.T) {
	client := &models.User{ID: 7, Role: models.RoleUser, Status: models.StatusActive}
	sellers := newFakeSellerRepo(client)
	service := NewSellerService(sellers, newFakeAssignmentRepo())

	if err := service.EnsureReferralGUID(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if client.ReferralGUID != nil {
		t.Fatalf("client GUID = %v", client.ReferralGUID)
	}
	if len(sellers.updatedGUIDs) != 0 {
		t.Fatal("clients must not receive referral GUIDs")
	}
}

func TestSellerServiceList(t *testing.T) {
	first := &models.User{ID: 1, FullName: "Ana", Role: models.RoleSeller, Status: models.StatusActive, Score: 90, ScoreLevel: models.ScoreLevelHigh}
	second := &models.User{ID: 2, FullName: "Beto", Role: models.RoleSeller, Status: models.StatusActive, Score: 70, ScoreLevel: ""}
	sellers := newFakeSellerRepo(first, second)
	assignments := newFakeAssignmentRepo()
	assignments.sales[first.ID] = 4
	service := NewSellerService(sellers, assignments)

	list, err := service.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("sellers = %d", len(list))
	}
	if list[0].SalesCount != 4 || list[0].ScoreLevel != models.ScoreLevelHigh {
		t.Fatalf("first seller = %+v", list[0])
	}
	if list[1].ScoreLevel != models.ScoreLevelMedium {
		t.Fatalf("fallback level = %q", list[1].ScoreLevel)
	}
}
