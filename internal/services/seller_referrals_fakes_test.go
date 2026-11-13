package services

import (
	"context"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

type fakeSellerRepo struct {
	repository.SellerRepository
	byID         map[uint]*models.User
	byGUID       map[string]*models.User
	active       []models.User
	all          []models.User
	updatedGUIDs map[uint]string
	scoreUpdates map[uint]struct {
		Score int
		Level string
	}
	overrides map[uint]struct {
		Score  int
		Level  string
		Reason string
		Actor  uint
	}
	cleared []uint
}

func newFakeSellerRepo(users ...*models.User) *fakeSellerRepo {
	repo := &fakeSellerRepo{
		byID:         make(map[uint]*models.User),
		byGUID:       make(map[string]*models.User),
		updatedGUIDs: make(map[uint]string),
		scoreUpdates: make(map[uint]struct {
			Score int
			Level string
		}),
		overrides: make(map[uint]struct {
			Score  int
			Level  string
			Reason string
			Actor  uint
		}),
	}
	for _, user := range users {
		repo.byID[user.ID] = user
		repo.all = append(repo.all, *user)
		repo.active = append(repo.active, *user)
		if user.ReferralGUID != nil {
			repo.byGUID[*user.ReferralGUID] = user
		}
	}
	return repo
}

func (r *fakeSellerRepo) FindByID(_ context.Context, id uint) (*models.User, error) {
	user, ok := r.byID[id]
	if !ok {
		return nil, repositoryNotFound()
	}
	return user, nil
}

func (r *fakeSellerRepo) FindActiveByReferralGUID(_ context.Context, guid string) (*models.User, error) {
	user, ok := r.byGUID[guid]
	if !ok || !user.IsActive() {
		return nil, repositoryNotFound()
	}
	return user, nil
}

func (r *fakeSellerRepo) ListActive(_ context.Context, search string) ([]models.User, error) {
	if search == "" {
		return r.active, nil
	}
	result := make([]models.User, 0, len(r.active))
	for _, user := range r.active {
		if strings.Contains(strings.ToLower(user.FullName), strings.ToLower(search)) {
			result = append(result, user)
		}
	}
	return result, nil
}

func (r *fakeSellerRepo) FindAllSellers(context.Context) ([]models.User, error) {
	return r.all, nil
}

func (r *fakeSellerRepo) UpdateReferralGUID(_ context.Context, id uint, guid string) error {
	r.updatedGUIDs[id] = guid
	if user, ok := r.byID[id]; ok {
		user.ReferralGUID = &guid
		r.byGUID[guid] = user
	}
	return nil
}

func (r *fakeSellerRepo) UpdateScore(_ context.Context, id uint, score int, level string) error {
	r.scoreUpdates[id] = struct {
		Score int
		Level string
	}{score, level}
	if user, ok := r.byID[id]; ok {
		user.Score = score
		user.ScoreLevel = level
	}
	return nil
}

func (r *fakeSellerRepo) SetScoreOverride(_ context.Context, id uint, score int, level, reason string, actorID uint, _ time.Time) error {
	r.overrides[id] = struct {
		Score  int
		Level  string
		Reason string
		Actor  uint
	}{score, level, reason, actorID}
	if user, ok := r.byID[id]; ok {
		user.Score = score
		user.ScoreLevel = level
		user.ScoreOverride = &score
		user.ScoreOverrideReason = &reason
		user.ScoreOverrideBy = &actorID
	}
	return nil
}

func (r *fakeSellerRepo) ClearScoreOverride(_ context.Context, id uint) error {
	r.cleared = append(r.cleared, id)
	if user, ok := r.byID[id]; ok {
		user.ScoreOverride = nil
		user.ScoreOverrideReason = nil
		user.ScoreOverrideBy = nil
		user.ScoreOverrideAt = nil
	}
	return nil
}

type fakeReferralRepo struct {
	repository.ReferralRepository
	visits       []models.ReferralVisit
	visitsCount  int64
	reservations int64
	commission   float64
	contracts    []models.Contract
}

func (r *fakeReferralRepo) RecordVisit(_ context.Context, visit *models.ReferralVisit) error {
	r.visits = append(r.visits, *visit)
	return nil
}

func (r *fakeReferralRepo) CountVisitsSince(context.Context, uint, time.Time) (int64, error) {
	return r.visitsCount, nil
}

func (r *fakeReferralRepo) CountAttributedReservationsSince(context.Context, uint, time.Time) (int64, error) {
	return r.reservations, nil
}

func (r *fakeReferralRepo) SumGeneratedCommissionSince(context.Context, uint, time.Time) (float64, error) {
	return r.commission, nil
}

func (r *fakeReferralRepo) ListAttributedReservations(context.Context, uint, int) ([]models.Contract, error) {
	return r.contracts, nil
}

type fakeUserRepoForAssignment struct {
	repository.UserRepository
	users map[uint]*models.User
}

func (r *fakeUserRepoForAssignment) FindByID(_ context.Context, id uint) (*models.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, repositoryNotFound()
	}
	return user, nil
}

type fakeContractRepoForAssignment struct {
	repository.ContractRepository
	contracts map[uint]*models.Contract
}

func (r *fakeContractRepoForAssignment) FindByIDWithDetails(_ context.Context, id uint) (*models.Contract, error) {
	contract, ok := r.contracts[id]
	if !ok {
		return nil, repositoryNotFound()
	}
	return contract, nil
}

type contractAssignmentUpdateRecord struct {
	ContractID uint
	Update     repository.ContractAssignmentUpdate
}

type fakeAssignmentRepo struct {
	repository.AssignmentRepository
	assignments     map[uint]*models.Assignment
	nextID          uint
	assignable      []models.User
	loads           map[uint]int64
	last            map[uint]*time.Time
	contractUpdates []contractAssignmentUpdateRecord
	statusUpdates   int
	outcomes        []models.ScoreOutcome
	sales           map[uint]int64
}

func newFakeAssignmentRepo() *fakeAssignmentRepo {
	return &fakeAssignmentRepo{
		assignments: make(map[uint]*models.Assignment),
		nextID:      1,
		loads:       make(map[uint]int64),
		last:        make(map[uint]*time.Time),
		sales:       make(map[uint]int64),
	}
}

func (r *fakeAssignmentRepo) Create(_ context.Context, assignment *models.Assignment) error {
	if assignment.ID == 0 {
		for {
			if _, exists := r.assignments[r.nextID]; !exists {
				break
			}
			r.nextID++
		}
		assignment.ID = r.nextID
		r.nextID++
	}
	if assignment.CreatedAt.IsZero() {
		assignment.CreatedAt = time.Now()
	}
	r.assignments[assignment.ID] = assignment
	return nil
}

func (r *fakeAssignmentRepo) FindByID(_ context.Context, id uint) (*models.Assignment, error) {
	assignment, ok := r.assignments[id]
	if !ok {
		return nil, repositoryNotFound()
	}
	return assignment, nil
}

func (r *fakeAssignmentRepo) FindByIDWithDetails(ctx context.Context, id uint) (*models.Assignment, error) {
	return r.FindByID(ctx, id)
}

func (r *fakeAssignmentRepo) FindPendingBySeller(_ context.Context, sellerID uint) ([]models.Assignment, error) {
	result := make([]models.Assignment, 0)
	for _, assignment := range r.assignments {
		if assignment.Status == models.AssignmentStatusPending && assignment.SellerID == sellerID {
			result = append(result, *assignment)
		}
	}
	return result, nil
}

func (r *fakeAssignmentRepo) FindPendingAll(context.Context) ([]models.Assignment, error) {
	result := make([]models.Assignment, 0)
	for _, assignment := range r.assignments {
		if assignment.Status == models.AssignmentStatusPending {
			result = append(result, *assignment)
		}
	}
	return result, nil
}

func (r *fakeAssignmentRepo) FindOverduePending(_ context.Context, now time.Time) ([]models.Assignment, error) {
	result := make([]models.Assignment, 0)
	for _, assignment := range r.assignments {
		if assignment.IsOverdue(now) {
			result = append(result, *assignment)
		}
	}
	return result, nil
}

func (r *fakeAssignmentRepo) UpdateStatusIfPending(_ context.Context, id uint, status string, respondedAt time.Time) (bool, error) {
	assignment, ok := r.assignments[id]
	if !ok || assignment.Status != models.AssignmentStatusPending {
		return false, nil
	}
	r.statusUpdates++
	assignment.Status = status
	assignment.RespondedAt = &respondedAt
	return true, nil
}

func (r *fakeAssignmentRepo) ExpiredSellerIDsForContract(_ context.Context, contractID uint) ([]uint, error) {
	ids := make([]uint, 0)
	for _, assignment := range r.assignments {
		if assignment.ContractID == contractID && assignment.Status == models.AssignmentStatusExpired {
			ids = append(ids, assignment.SellerID)
		}
	}
	return ids, nil
}

func (r *fakeAssignmentRepo) ListAssignableSellers(_ context.Context, excludeIDs []uint) ([]models.User, error) {
	excluded := make(map[uint]bool, len(excludeIDs))
	for _, id := range excludeIDs {
		excluded[id] = true
	}
	result := make([]models.User, 0, len(r.assignable))
	for _, seller := range r.assignable {
		if !excluded[seller.ID] {
			result = append(result, seller)
		}
	}
	return result, nil
}

func (r *fakeAssignmentRepo) PendingLoadBySellerIDs(_ context.Context, ids []uint) (map[uint]int64, error) {
	result := make(map[uint]int64, len(ids))
	for _, id := range ids {
		result[id] = r.loads[id]
	}
	return result, nil
}

func (r *fakeAssignmentRepo) LastAssignedAtBySellerIDs(_ context.Context, ids []uint) (map[uint]*time.Time, error) {
	result := make(map[uint]*time.Time, len(ids))
	for _, id := range ids {
		result[id] = r.last[id]
	}
	return result, nil
}

func (r *fakeAssignmentRepo) ScoreOutcomesSince(context.Context, time.Time) ([]models.ScoreOutcome, error) {
	return r.outcomes, nil
}

func (r *fakeAssignmentRepo) UpdateContractAssignment(_ context.Context, contractID uint, update repository.ContractAssignmentUpdate) error {
	r.contractUpdates = append(r.contractUpdates, contractAssignmentUpdateRecord{ContractID: contractID, Update: update})
	return nil
}

func (r *fakeAssignmentRepo) CountSalesBySellerIDs(_ context.Context, ids []uint) (map[uint]int64, error) {
	result := make(map[uint]int64, len(ids))
	for _, id := range ids {
		result[id] = r.sales[id]
	}
	return result, nil
}

func repositoryNotFound() error {
	return errRecordNotFound
}

var errRecordNotFound = &notFoundError{}

type notFoundError struct{}

func (*notFoundError) Error() string { return "record not found" }
