package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// SellerService owns seller referral GUIDs and the seller picker payload.
type SellerService struct {
	repo           repository.SellerRepository
	assignmentRepo repository.AssignmentRepository
}

// NewSellerService creates the seller service.
func NewSellerService(repo repository.SellerRepository, assignmentRepo repository.AssignmentRepository) *SellerService {
	return &SellerService{repo: repo, assignmentRepo: assignmentRepo}
}

// EnsureReferralGUID assigns an opaque referral GUID to a seller when it is
// missing.
func (s *SellerService) EnsureReferralGUID(ctx context.Context, user *models.User) error {
	if s == nil || s.repo == nil || user == nil || !user.IsSeller() {
		return nil
	}
	if user.ReferralGUID != nil && strings.TrimSpace(*user.ReferralGUID) != "" {
		return nil
	}
	if user.ID == 0 {
		return fmt.Errorf("cannot assign a referral GUID before the user is persisted")
	}

	guid := uuid.NewString()
	if err := s.repo.UpdateReferralGUID(ctx, user.ID, guid); err != nil {
		return err
	}
	user.ReferralGUID = &guid
	return nil
}

// List returns active sellers with their sales count and score for pickers.
func (s *SellerService) List(ctx context.Context, search string) ([]models.SellerSummary, error) {
	sellers, err := s.repo.ListActive(ctx, strings.TrimSpace(search))
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(sellers))
	for _, seller := range sellers {
		ids = append(ids, seller.ID)
	}
	sales, err := s.assignmentRepo.CountSalesBySellerIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	summaries := make([]models.SellerSummary, 0, len(sellers))
	for _, seller := range sellers {
		level := seller.ScoreLevel
		if level == "" {
			level = models.ScoreLevelFor(seller.Score)
		}
		summaries = append(summaries, models.SellerSummary{
			ID:         seller.ID,
			FullName:   seller.FullName,
			Address:    seller.Address,
			SalesCount: sales[seller.ID],
			Score:      seller.Score,
			ScoreLevel: level,
		})
	}
	return summaries, nil
}
