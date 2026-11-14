package models

import "testing"

func TestUserScoreResponse(t *testing.T) {
	guid := "8f14e45f-ceea-4a67-9c0f-1f2d3e4a5b6c"
	override := 70
	reason := "Acuerdo comercial"

	user := User{
		ID:                  4,
		Role:                RoleSeller,
		FullName:            "Carlos Ruiz",
		ReferralGUID:        &guid,
		Score:               70,
		ScoreLevel:          "",
		ScoreOverride:       &override,
		ScoreOverrideReason: &reason,
	}
	resp := user.ToResponse()
	if resp.ReferralGUID == nil || *resp.ReferralGUID != guid {
		t.Fatalf("referral_guid = %v", resp.ReferralGUID)
	}
	if resp.Score != 70 || resp.ScoreLevel != ScoreLevelMedium {
		t.Fatalf("score = %d/%q", resp.Score, resp.ScoreLevel)
	}
	if resp.ScoreOverride == nil || *resp.ScoreOverride != 70 {
		t.Fatalf("score_override = %v", resp.ScoreOverride)
	}
	if resp.ScoreOverrideReason == nil || *resp.ScoreOverrideReason != reason {
		t.Fatalf("score_override_reason = %v", resp.ScoreOverrideReason)
	}
}

func TestUserIsAssignable(t *testing.T) {
	seller := &User{Role: RoleSeller, Status: StatusActive, ScoreLevel: ScoreLevelHigh}
	if !seller.IsAssignable() {
		t.Fatal("active high seller should be assignable")
	}
	seller.ScoreLevel = ScoreLevelMedium
	if !seller.IsAssignable() {
		t.Fatal("active medium seller should be assignable")
	}
	seller.ScoreLevel = ScoreLevelLow
	if seller.IsAssignable() {
		t.Fatal("low seller must be excluded")
	}
	seller.ScoreLevel = ScoreLevelHigh
	seller.Status = StatusInactive
	if seller.IsAssignable() {
		t.Fatal("inactive seller must be excluded")
	}
}

func TestAssignmentConstants(t *testing.T) {
	if AssignmentOriginReferral == AssignmentOriginManual {
		t.Fatal("origin constants must be distinct")
	}
	if AssignmentStatusPending == AssignmentStatusExpired {
		t.Fatal("status constants must be distinct")
	}
}
