package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
)

const testReferralGUID = "8f14e45f-ceea-4a67-9c0f-1f2d3e4a5b6c"

func TestReferralAttributionSignature(t *testing.T) {
	service := NewReferralService(nil, nil, &config.Config{JWTSecret: "test-secret"})
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

	signed := service.SignAttribution(testReferralGUID, now)
	guid, ok := service.ParseAttribution(signed, now.Add(29*24*time.Hour))
	if !ok || guid != testReferralGUID {
		t.Fatalf("parse = %q, %v", guid, ok)
	}

	if _, ok := service.ParseAttribution(signed, now.Add(31*24*time.Hour)); ok {
		t.Fatal("expired attribution must be rejected")
	}

	tampered := strings.Replace(signed, testReferralGUID, "11111111-1111-4111-8111-111111111111", 1)
	if _, ok := service.ParseAttribution(tampered, now); ok {
		t.Fatal("tampered attribution must be rejected")
	}

	other := NewReferralService(nil, nil, &config.Config{JWTSecret: "other-secret"})
	if _, ok := other.ParseAttribution(signed, now); ok {
		t.Fatal("signature from another secret must be rejected")
	}
}

func TestReferralServiceResolveVisitAndAttribution(t *testing.T) {
	guid := testReferralGUID
	seller := &models.User{ID: 3, FullName: "Carlos Ruiz", Role: models.RoleSeller, Status: models.StatusActive, ReferralGUID: &guid}
	sellers := newFakeSellerRepo(seller)
	visits := &fakeReferralRepo{}
	service := NewReferralService(visits, sellers, &config.Config{JWTSecret: "secret"})

	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	resolved, cookieValue, err := service.ResolveVisit(context.Background(), guid, "/r/"+guid, "10.0.0.1", "agent", now)
	if err != nil {
		t.Fatalf("ResolveVisit: %v", err)
	}
	if resolved.ID != seller.ID {
		t.Fatalf("resolved seller = %d", resolved.ID)
	}
	if len(visits.visits) != 1 {
		t.Fatalf("visits = %d", len(visits.visits))
	}
	if visits.visits[0].ReferralGUID != guid {
		t.Fatalf("visit GUID = %q", visits.visits[0].ReferralGUID)
	}
	if visits.visits[0].SessionHash == nil || *visits.visits[0].SessionHash == "" {
		t.Fatal("expected a hashed session")
	}
	if _, ok := service.ParseAttribution(cookieValue, now); !ok {
		t.Fatal("expected a valid signed attribution")
	}

	// The most recent cookie wins; an explicit GUID still takes precedence.
	attributed, attributedGUID, err := service.ResolveAttribution(context.Background(), "", cookieValue, now)
	if err != nil || attributed == nil || attributedGUID != guid {
		t.Fatalf("cookie attribution = %v, %q, %v", attributed, attributedGUID, err)
	}
}

func TestReferralServiceUnknownGUIDDoesNotRecordVisit(t *testing.T) {
	service := NewReferralService(&fakeReferralRepo{}, newFakeSellerRepo(), &config.Config{})
	visits := service.repo.(*fakeReferralRepo)

	_, _, err := service.ResolveVisit(context.Background(), "missing", "/r/missing", "1.1.1.1", "agent", time.Now())
	if !errors.Is(err, ErrReferralNotFound) {
		t.Fatalf("err = %v", err)
	}
	if len(visits.visits) != 0 {
		t.Fatal("unknown GUID must not record a visit")
	}
}

func TestReferralAttributionExpiresAfterThirtyDays(t *testing.T) {
	guid := testReferralGUID
	seller := &models.User{ID: 3, FullName: "Carlos Ruiz", Role: models.RoleSeller, Status: models.StatusActive, ReferralGUID: &guid}
	service := NewReferralService(&fakeReferralRepo{}, newFakeSellerRepo(seller), &config.Config{})
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	cookie := service.SignAttribution(guid, now)

	attributed, _, _ := service.ResolveAttribution(context.Background(), "", cookie, now.Add(30*24*time.Hour))
	if attributed == nil {
		t.Fatal("attribution inside the 30-day window must resolve")
	}
	attributed, _, _ = service.ResolveAttribution(context.Background(), "", cookie, now.Add(31*24*time.Hour))
	if attributed != nil {
		t.Fatal("attribution after 30 days must be ignored")
	}
}

func TestReferralMetrics(t *testing.T) {
	repo := &fakeReferralRepo{visitsCount: 10, reservations: 2, commission: 1500}
	guid := testReferralGUID
	override := 6.0
	seller := &models.User{
		ID:                     3,
		FullName:               "Carlos Ruiz",
		Role:                   models.RoleSeller,
		Status:                 models.StatusActive,
		ReferralGUID:           &guid,
		CommissionRateOverride: &override,
	}
	service := NewReferralService(repo, newFakeSellerRepo(seller), &config.Config{})

	metrics, err := service.Metrics(context.Background(), 3, 30)
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	if metrics.Visits != 10 || metrics.AttributedReservations != 2 {
		t.Fatalf("metrics = %+v", metrics)
	}
	if metrics.ConversionRate != 20 {
		t.Fatalf("conversion = %v", metrics.ConversionRate)
	}
	if metrics.GeneratedCommission != 1500 {
		t.Fatalf("commission = %v", metrics.GeneratedCommission)
	}
	if metrics.ReferralGUID == nil || *metrics.ReferralGUID != guid {
		t.Fatalf("referral_guid = %v", metrics.ReferralGUID)
	}
	if metrics.SellerName != "Carlos Ruiz" {
		t.Fatalf("seller name = %q", metrics.SellerName)
	}
	if metrics.CommissionRate == nil || *metrics.CommissionRate != 6 {
		t.Fatalf("commission rate = %v", metrics.CommissionRate)
	}
}

func TestPortalRedirectURLKeepsExistingQuery(t *testing.T) {
	service := NewReferralService(nil, nil, &config.Config{PortalURL: "https://portal.example.com/?lang=es"})
	got := service.PortalRedirectURL(testReferralGUID)
	if !strings.Contains(got, "ref="+testReferralGUID) || !strings.Contains(got, "lang=es") {
		t.Fatalf("redirect = %q", got)
	}
}

func TestSessionHashIsDeterministicAndAnonymized(t *testing.T) {
	first := SessionHash("10.0.0.1", "agent")
	second := SessionHash("10.0.0.1", "agent")
	other := SessionHash("10.0.0.2", "agent")
	if first != second {
		t.Fatal("session hash must be deterministic")
	}
	if first == other {
		t.Fatal("different IPs must produce different hashes")
	}
	if strings.Contains(first, "10.0.0.1") {
		t.Fatal("session hash must not expose the IP")
	}
}
