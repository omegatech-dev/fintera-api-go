package models

import (
	"testing"
	"time"
)

func TestScoreLevelThresholds(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{100, ScoreLevelHigh},
		{85, ScoreLevelHigh},
		{84, ScoreLevelMedium},
		{60, ScoreLevelMedium},
		{59, ScoreLevelLow},
		{0, ScoreLevelLow},
	}
	for _, tc := range cases {
		if got := ScoreLevelFor(tc.score); got != tc.want {
			t.Errorf("ScoreLevelFor(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestComputeAcceptanceScore(t *testing.T) {
	cases := []struct {
		name      string
		accepted  int64
		expired   int64
		wantScore int
		wantLevel string
	}{
		{name: "no history", accepted: 0, expired: 0, wantScore: 100, wantLevel: ScoreLevelHigh},
		{name: "twelve of thirteen", accepted: 12, expired: 1, wantScore: 92, wantLevel: ScoreLevelHigh},
		{name: "exactly eighty five", accepted: 17, expired: 3, wantScore: 85, wantLevel: ScoreLevelHigh},
		{name: "exactly sixty", accepted: 3, expired: 2, wantScore: 60, wantLevel: ScoreLevelMedium},
		{name: "below sixty", accepted: 59, expired: 41, wantScore: 59, wantLevel: ScoreLevelLow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score, level := ComputeAcceptanceScore(tc.accepted, tc.expired)
			if score != tc.wantScore {
				t.Fatalf("score = %d, want %d", score, tc.wantScore)
			}
			if level != tc.wantLevel {
				t.Fatalf("level = %q, want %q", level, tc.wantLevel)
			}
		})
	}
}

func TestAssignmentDeadlineAndOverdue(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	deadline := AssignmentDeadlineFrom(now)
	if got := deadline.Sub(now); got != DefaultAssignmentAcceptWindow {
		t.Fatalf("window = %v, want %v", got, DefaultAssignmentAcceptWindow)
	}

	assignment := Assignment{Status: AssignmentStatusPending, AcceptDeadlineAt: &deadline}
	if !assignment.IsOverdue(deadline.Add(time.Second)) {
		t.Fatal("expected the assignment to be overdue")
	}
	if assignment.IsOverdue(now) {
		t.Fatal("did not expect the assignment to be overdue before the deadline")
	}

	assignment.Status = AssignmentStatusAccepted
	if assignment.IsOverdue(deadline.Add(time.Hour)) {
		t.Fatal("answered assignments are never overdue")
	}
}

func TestAssignmentResponse(t *testing.T) {
	amount := 250000.0
	contract := Contract{
		ID:            9,
		GUID:          "contract-guid",
		Status:        ContractStatusPending,
		FinancingType: FinancingTypeDirect,
		Amount:        &amount,
	}
	contract.Lot.Name = "Lote 7"
	contract.Lot.Project.Name = "Ciudad Jaguar"
	contract.ApplicantUser.FullName = "Ana Pérez"
	contract.ApplicantUser.Phone = "9999-1111"
	contract.ApplicantUser.Email = "ana@example.com"

	assignment := Assignment{
		ID:         1,
		ContractID: 9,
		SellerID:   3,
		Origin:     AssignmentOriginAutomatic,
		Status:     AssignmentStatusPending,
		Contract:   contract,
		Seller:     &User{ID: 3, FullName: "Carlos Ruiz"},
	}
	resp := assignment.ToResponse()
	if resp.Seller == nil || resp.Seller.FullName != "Carlos Ruiz" {
		t.Fatalf("seller = %+v", resp.Seller)
	}
	if resp.Contract == nil || resp.Contract.ProjectName != "Ciudad Jaguar" || resp.Contract.LotName != "Lote 7" {
		t.Fatalf("contract summary = %+v", resp.Contract)
	}
}
