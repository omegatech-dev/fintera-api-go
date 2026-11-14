package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// ScoreService recomputes seller acceptance scores and applies audited admin
// overrides.
type ScoreService struct {
	sellers     repository.SellerRepository
	assignments repository.AssignmentRepository
	auditSvc    *AuditService
	now         func() time.Time
}

// NewScoreService creates the score service.
func NewScoreService(sellers repository.SellerRepository, assignments repository.AssignmentRepository, auditSvc *AuditService) *ScoreService {
	return &ScoreService{
		sellers:     sellers,
		assignments: assignments,
		auditSvc:    auditSvc,
		now:         time.Now,
	}
}

// RecomputeAll refreshes every seller score from the 90-day assignment window.
// Sellers with an active override keep their overridden value.
func (s *ScoreService) RecomputeAll(ctx context.Context) error {
	since := s.now().AddDate(0, 0, -models.SellerScoreWindowDays)
	outcomes, err := s.assignments.ScoreOutcomesSince(ctx, since)
	if err != nil {
		return err
	}
	bySeller := make(map[uint]models.ScoreOutcome, len(outcomes))
	for _, outcome := range outcomes {
		bySeller[outcome.SellerID] = outcome
	}

	sellers, err := s.sellers.FindAllSellers(ctx)
	if err != nil {
		return err
	}
	for _, seller := range sellers {
		if seller.ScoreOverride != nil {
			continue
		}
		outcome := bySeller[seller.ID]
		score, level := models.ComputeAcceptanceScore(outcome.Accepted, outcome.Expired)
		if score == seller.Score && level == seller.ScoreLevel {
			continue
		}
		if err := s.sellers.UpdateScore(ctx, seller.ID, score, level); err != nil {
			return err
		}
	}
	return nil
}

// RecomputeSeller refreshes one seller, respecting an active override.
func (s *ScoreService) RecomputeSeller(ctx context.Context, sellerID uint) (*models.User, error) {
	seller, err := s.sellers.FindByID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if seller.ScoreOverride != nil || !seller.IsSeller() {
		return seller, nil
	}

	since := s.now().AddDate(0, 0, -models.SellerScoreWindowDays)
	outcomes, err := s.assignments.ScoreOutcomesSince(ctx, since)
	if err != nil {
		return nil, err
	}
	var outcome models.ScoreOutcome
	for _, item := range outcomes {
		if item.SellerID == sellerID {
			outcome = item
			break
		}
	}
	score, level := models.ComputeAcceptanceScore(outcome.Accepted, outcome.Expired)
	if score != seller.Score || level != seller.ScoreLevel {
		if err := s.sellers.UpdateScore(ctx, sellerID, score, level); err != nil {
			return nil, err
		}
		seller.Score = score
		seller.ScoreLevel = level
	}
	return seller, nil
}

// MetricsForUsers returns the accepted/expired breakdown of the 90-day score
// window for each requested user. Non-sellers and users without history get a
// zeroed breakdown, which the UI renders as "nuevo".
func (s *ScoreService) MetricsForUsers(ctx context.Context, userIDs []uint) (map[uint]models.ScoreMetrics, error) {
	metrics := make(map[uint]models.ScoreMetrics, len(userIDs))
	if len(userIDs) == 0 {
		return metrics, nil
	}
	for _, id := range userIDs {
		metrics[id] = models.EmptyScoreMetrics()
	}
	if s == nil || s.assignments == nil {
		return metrics, nil
	}
	since := s.now().AddDate(0, 0, -models.SellerScoreWindowDays)
	outcomes, err := s.assignments.ScoreOutcomesSince(ctx, since)
	if err != nil {
		return nil, err
	}
	wanted := make(map[uint]bool, len(userIDs))
	for _, id := range userIDs {
		wanted[id] = true
	}
	for _, outcome := range outcomes {
		if !wanted[outcome.SellerID] {
			continue
		}
		metrics[outcome.SellerID] = models.ScoreMetrics{
			AcceptedCount: outcome.Accepted,
			ExpiredCount:  outcome.Expired,
			WindowDays:    models.SellerScoreWindowDays,
		}
	}
	return metrics, nil
}

// MetricsForUser returns the breakdown for one user.
func (s *ScoreService) MetricsForUser(ctx context.Context, userID uint) (models.ScoreMetrics, error) {
	metrics, err := s.MetricsForUsers(ctx, []uint{userID})
	if err != nil {
		return models.EmptyScoreMetrics(), err
	}
	return metrics[userID], nil
}

// Override stores an audited manual score for a seller.
func (s *ScoreService) Override(ctx context.Context, sellerID uint, score int, reason string, actorID uint, ip, userAgent string) (*models.User, error) {
	reason = strings.TrimSpace(reason)
	if score < 0 || score > 100 || reason == "" {
		return nil, ErrScoreOverrideInvalid
	}
	seller, err := s.sellers.FindByID(ctx, sellerID)
	if err != nil {
		return nil, ErrNotFound
	}
	if !seller.IsSeller() {
		return nil, ErrScoreOverrideInvalid
	}

	level := models.ScoreLevelFor(score)
	at := s.now()
	if err := s.sellers.SetScoreOverride(ctx, sellerID, score, level, reason, actorID, at); err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Puntaje del vendedor %s sobrescrito de %d a %d. Motivo: %s", seller.FullName, seller.Score, score, reason)
	if s.auditSvc != nil {
		logIgnoredError("write score override audit log",
			s.auditSvc.Log(ctx, actorID, "SCORE_OVERRIDE", "User", sellerID, details, ip, userAgent))
	}

	seller.Score = score
	seller.ScoreLevel = level
	seller.ScoreOverride = &score
	seller.ScoreOverrideReason = &reason
	seller.ScoreOverrideBy = &actorID
	seller.ScoreOverrideAt = &at
	return seller, nil
}

// ClearOverride removes an override and recomputes the score from history.
func (s *ScoreService) ClearOverride(ctx context.Context, sellerID, actorID uint, ip, userAgent string) (*models.User, error) {
	seller, err := s.sellers.FindByID(ctx, sellerID)
	if err != nil {
		return nil, ErrNotFound
	}
	if !seller.IsSeller() {
		return nil, ErrScoreOverrideInvalid
	}
	if err := s.sellers.ClearScoreOverride(ctx, sellerID); err != nil {
		return nil, err
	}
	if s.auditSvc != nil {
		logIgnoredError("write score override clear audit log",
			s.auditSvc.Log(ctx, actorID, "SCORE_OVERRIDE_CLEARED", "User", sellerID,
				fmt.Sprintf("Sobrescritura de puntaje eliminada para %s", seller.FullName), ip, userAgent))
	}
	return s.RecomputeSeller(ctx, sellerID)
}
