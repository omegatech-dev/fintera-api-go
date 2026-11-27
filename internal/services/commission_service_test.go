package services

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeCommissionRepository struct {
	repository.CommissionRepository
	settings          models.CommissionSettings
	exceptions        map[uint]*models.SellerCommissionRate
	credits           map[uint]*models.SellerCommissionCredit
	applications      map[uint][]models.CreditApplication
	contractSellers   map[uint]uint
	nextID            uint
	nextCreditID      uint
	nextApplicationID uint
	deleted           []uint
}

func newFakeCommissionRepository() *fakeCommissionRepository {
	settings := models.DefaultCommissionSettings()
	return &fakeCommissionRepository{
		settings:          settings,
		exceptions:        make(map[uint]*models.SellerCommissionRate),
		credits:           make(map[uint]*models.SellerCommissionCredit),
		applications:      make(map[uint][]models.CreditApplication),
		contractSellers:   make(map[uint]uint),
		nextID:            1,
		nextCreditID:      1,
		nextApplicationID: 1,
	}
}

func (r *fakeCommissionRepository) GetSettings(context.Context) (*models.CommissionSettings, error) {
	copy := r.settings
	return &copy, nil
}

func (r *fakeCommissionRepository) UpdateSettings(_ context.Context, settings *models.CommissionSettings) error {
	settings.ID = 1
	r.settings = *settings
	return nil
}

func (r *fakeCommissionRepository) FindSellerRate(_ context.Context, sellerID uint) (*models.SellerCommissionRate, error) {
	rate, ok := r.exceptions[sellerID]
	if !ok {
		return nil, gormRecordNotFound()
	}
	return rate, nil
}

func (r *fakeCommissionRepository) FindSellerRateByID(_ context.Context, id uint) (*models.SellerCommissionRate, error) {
	for _, rate := range r.exceptions {
		if rate.ID == id {
			return rate, nil
		}
	}
	return nil, gormRecordNotFound()
}

func (r *fakeCommissionRepository) ListSellerRates(context.Context) ([]models.SellerCommissionRate, error) {
	rates := make([]models.SellerCommissionRate, 0, len(r.exceptions))
	for _, rate := range r.exceptions {
		rates = append(rates, *rate)
	}
	return rates, nil
}

func (r *fakeCommissionRepository) UpsertSellerRate(_ context.Context, rate *models.SellerCommissionRate) error {
	if existing, ok := r.exceptions[rate.SellerID]; ok {
		existing.Rate = rate.Rate
		return nil
	}
	rate.ID = r.nextID
	r.nextID++
	copy := *rate
	r.exceptions[rate.SellerID] = &copy
	return nil
}

func (r *fakeCommissionRepository) DeleteSellerRate(_ context.Context, id uint) error {
	for sellerID, rate := range r.exceptions {
		if rate.ID == id {
			delete(r.exceptions, sellerID)
			r.deleted = append(r.deleted, id)
			return nil
		}
	}
	return gormRecordNotFound()
}

func (r *fakeCommissionRepository) CreateCredit(_ context.Context, credit *models.SellerCommissionCredit) error {
	credit.ID = r.nextCreditID
	r.nextCreditID++
	if credit.Status == "" {
		credit.Status = models.CommissionCreditStatusPending
	}
	copy := *credit
	r.credits[credit.ID] = &copy
	return nil
}

func (r *fakeCommissionRepository) FindCreditByID(_ context.Context, id uint) (*models.SellerCommissionCredit, error) {
	credit, ok := r.credits[id]
	if !ok {
		return nil, gormRecordNotFound()
	}
	copy := *credit
	return &copy, nil
}

func (r *fakeCommissionRepository) ListCredits(_ context.Context, sellerID uint) ([]models.SellerCommissionCredit, error) {
	credits := make([]models.SellerCommissionCredit, 0, len(r.credits))
	for _, credit := range r.credits {
		if sellerID > 0 && credit.SellerID != sellerID {
			continue
		}
		credits = append(credits, *credit)
	}
	return credits, nil
}

func (r *fakeCommissionRepository) SettleCredit(_ context.Context, id, actorID uint, settledAt time.Time) (bool, error) {
	credit, ok := r.credits[id]
	if !ok {
		return false, gormRecordNotFound()
	}
	if credit.Status == models.CommissionCreditStatusSettled {
		return false, nil
	}
	credit.Status = models.CommissionCreditStatusSettled
	credit.AppliedAmount = credit.Amount
	credit.SettledAt = &settledAt
	credit.SettledBy = &actorID
	return true, nil
}

func (r *fakeCommissionRepository) ListApplicationsByCredits(_ context.Context, creditIDs []uint) (map[uint][]models.CreditApplication, error) {
	result := make(map[uint][]models.CreditApplication, len(creditIDs))
	for _, creditID := range creditIDs {
		var active []models.CreditApplication
		for _, application := range r.applications[creditID] {
			if application.ReversedAt != nil {
				continue
			}
			active = append(active, application)
		}
		sort.Slice(active, func(i, j int) bool {
			if active[i].CreatedAt.Equal(active[j].CreatedAt) {
				return active[i].ID > active[j].ID
			}
			return active[i].CreatedAt.After(active[j].CreatedAt)
		})
		result[creditID] = active
	}
	return result, nil
}

func (r *fakeCommissionRepository) ListAppliedTotalByContractIDs(_ context.Context, contractIDs []uint) (map[uint]float64, error) {
	result := make(map[uint]float64, len(contractIDs))
	for _, contractID := range contractIDs {
		for _, applications := range r.applications {
			for _, application := range applications {
				if application.ContractID == contractID && application.ReversedAt == nil {
					result[contractID] += application.Amount
				}
			}
		}
	}
	return result, nil
}

func (r *fakeCommissionRepository) PayCommissionWithCredits(_ context.Context, contractID, actorID uint, commission float64) (*models.CommissionPayout, error) {
	payout := &models.CommissionPayout{
		Commission:   models.RoundMoney(commission),
		Applications: []models.CreditApplicationResponse{},
	}
	sellerID := r.contractSellers[contractID]
	var credits []models.SellerCommissionCredit
	for _, credit := range r.credits {
		if credit.SellerID != sellerID {
			continue
		}
		if credit.Status != models.CommissionCreditStatusPending || credit.RemainingAmount() <= models.MoneyEpsilon {
			continue
		}
		credits = append(credits, *credit)
	}
	sort.Slice(credits, func(i, j int) bool {
		if credits[i].CreatedAt.Equal(credits[j].CreatedAt) {
			return credits[i].ID < credits[j].ID
		}
		return credits[i].CreatedAt.Before(credits[j].CreatedAt)
	})
	available := 0.0
	for i := range credits {
		available += credits[i].RemainingAmount()
	}

	now := time.Now()
	for _, allocation := range models.PlanCreditAllocations(commission, credits) {
		credit := r.credits[allocation.CreditID]
		if credit == nil {
			continue
		}
		credit.AppliedAmount = models.RoundMoney(credit.AppliedAmount + allocation.Amount)
		if credit.AppliedAmount >= credit.Amount-models.MoneyEpsilon {
			credit.Status = models.CommissionCreditStatusSettled
			credit.SettledAt = &now
			credit.SettledBy = &actorID
		}
		application := models.CreditApplication{
			ID:         r.nextApplicationID,
			CreditID:   credit.ID,
			ContractID: contractID,
			Amount:     allocation.Amount,
			ActorID:    &actorID,
			CreatedAt:  now,
		}
		r.nextApplicationID++
		r.applications[credit.ID] = append(r.applications[credit.ID], application)
		payout.Applications = append(payout.Applications, application.ToResponse())
		payout.AppliedTotal = models.RoundMoney(payout.AppliedTotal + allocation.Amount)
	}
	payout.NetCash = models.RoundMoney(payout.Commission - payout.AppliedTotal)
	if payout.NetCash < 0 {
		payout.NetCash = 0
	}
	payout.RemainingCredit = models.RoundMoney(models.RoundMoney(available) - payout.AppliedTotal)
	if payout.RemainingCredit < 0 {
		payout.RemainingCredit = 0
	}
	return payout, nil
}

func (r *fakeCommissionRepository) PreviewCommissionPayout(_ context.Context, sellerID *uint, commission float64) (*models.CommissionPayoutPreview, error) {
	preview := &models.CommissionPayoutPreview{Commission: models.RoundMoney(commission)}
	if sellerID != nil {
		for _, credit := range r.credits {
			if credit.SellerID != *sellerID || credit.Status != models.CommissionCreditStatusPending {
				continue
			}
			preview.AvailableCredit += credit.RemainingAmount()
		}
	}
	preview.AvailableCredit = models.RoundMoney(preview.AvailableCredit)
	preview.Applicable = models.RoundMoney(min(preview.AvailableCredit, preview.Commission))
	preview.NetCash = models.RoundMoney(preview.Commission - preview.Applicable)
	preview.RemainingCredit = models.RoundMoney(preview.AvailableCredit - preview.Applicable)
	return preview, nil
}

func (r *fakeCommissionRepository) RestoreAndCreateCredit(_ context.Context, input repository.CommissionCompensationInput) (*models.CommissionCompensation, error) {
	result := &models.CommissionCompensation{}
	now := time.Now()
	for creditID, applications := range r.applications {
		for i := range applications {
			application := &applications[i]
			if application.ContractID != input.ContractID || application.ReversedAt != nil {
				continue
			}
			if credit := r.credits[creditID]; credit != nil {
				credit.AppliedAmount = models.RoundMoney(credit.AppliedAmount - application.Amount)
				if credit.AppliedAmount < 0 {
					credit.AppliedAmount = 0
				}
				credit.Status = models.CommissionCreditStatusPending
				credit.SettledAt = nil
				credit.SettledBy = nil
			}
			application.ReversedAt = &now
			result.Restored = models.RoundMoney(result.Restored + application.Amount)
		}
		r.applications[creditID] = applications
	}
	result.NetCash = models.RoundMoney(input.Gross - result.Restored)
	if result.NetCash < 0 {
		result.NetCash = 0
	}
	if result.NetCash > models.MoneyEpsilon && input.SellerID > 0 {
		credit := &models.SellerCommissionCredit{
			SellerID:   input.SellerID,
			ContractID: input.ContractID,
			Amount:     result.NetCash,
			Status:     models.CommissionCreditStatusPending,
			Source:     input.Source,
		}
		if err := r.CreateCredit(context.Background(), credit); err != nil {
			return nil, err
		}
		result.Credit = credit
	}
	return result, nil
}

type fakeCommissionUserRepository struct {
	repository.UserRepository
	users map[uint]*models.User
}

func (r *fakeCommissionUserRepository) FindByID(_ context.Context, id uint) (*models.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, gormRecordNotFound()
	}
	return user, nil
}

func (r *fakeCommissionUserRepository) FindByIDs(_ context.Context, ids []uint) ([]models.User, error) {
	users := make([]models.User, 0, len(ids))
	for _, id := range ids {
		if user, ok := r.users[id]; ok {
			users = append(users, *user)
		}
	}
	return users, nil
}

func gormRecordNotFound() error {
	return gorm.ErrRecordNotFound
}

func TestCommissionRateResolution(t *testing.T) {
	repo := newFakeCommissionRepository()
	repo.settings = models.CommissionSettings{ID: 1, CommissionDefaultRate: 5, CommissionUnassignedRate: 3}
	sellerRepo := &fakeCommissionUserRepository{users: map[uint]*models.User{
		7: {ID: 7, Role: models.RoleSeller, FullName: "Ana Vendedora", Status: models.StatusActive},
	}}
	service := NewCommissionService(repo, sellerRepo, nil)
	project := models.Project{ID: 1, CommissionRateDirect: 4, CommissionRateBank: 6}

	amount := 100000.0
	base := func(sellerID *uint, financing string) *models.Contract {
		return &models.Contract{
			Amount:        &amount,
			FinancingType: financing,
			SellerID:      sellerID,
			Lot:           models.Lot{Project: project},
		}
	}

	sellerID := uint(7)
	rate, err := service.ResolveRateForContract(context.Background(), base(nil, models.FinancingTypeDirect))
	require.NoError(t, err)
	assert.Equal(t, 3.0, rate, "contracts without a seller use the unassigned rate")

	rate, err = service.ResolveRateForContract(context.Background(), base(&sellerID, models.FinancingTypeDirect))
	require.NoError(t, err)
	assert.Equal(t, 4.0, rate, "projects provide the direct rate")

	rate, err = service.ResolveRateForContract(context.Background(), base(&sellerID, models.FinancingTypeBank))
	require.NoError(t, err)
	assert.Equal(t, 6.0, rate, "projects provide the bank rate")

	repo.exceptions[sellerID] = &models.SellerCommissionRate{ID: 1, SellerID: sellerID, Rate: 7}
	rate, err = service.ResolveRateForContract(context.Background(), base(&sellerID, models.FinancingTypeBank))
	require.NoError(t, err)
	assert.Equal(t, 7.0, rate, "the seller exception wins over the project rate")

	rate, err = service.ResolveRateForContract(context.Background(), base(&sellerID, models.FinancingTypeCash))
	require.NoError(t, err)
	assert.Equal(t, 7.0, rate, "the seller exception wins for every financing type")

	projectWithoutBank := models.Project{ID: 1, CommissionRateDirect: 0}
	contract := base(&sellerID, models.FinancingTypeBank)
	contract.Lot.Project = projectWithoutBank
	delete(repo.exceptions, sellerID)
	rate, err = service.ResolveRateForContract(context.Background(), contract)
	require.NoError(t, err)
	assert.Equal(t, 5.0, rate, "missing project rates fall back to the global default")
}

func TestCommissionApplyFreezesOriginAndAccrual(t *testing.T) {
	repo := newFakeCommissionRepository()
	repo.settings = models.CommissionSettings{ID: 1, CommissionDefaultRate: 5, CommissionUnassignedRate: 3}
	service := NewCommissionService(repo, &fakeCommissionUserRepository{}, nil)

	amount := 100000.0
	sellerID := uint(7)
	approvedAt := time.Date(2026, 5, 20, 15, 4, 0, 0, time.UTC)
	contract := &models.Contract{
		Amount:           &amount,
		FinancingType:    models.FinancingTypeDirect,
		AssignmentOrigin: models.AssignmentOriginReferral,
		SellerID:         &sellerID,
		Lot:              models.Lot{Project: models.Project{ID: 1, CommissionRateDirect: 4}},
	}
	require.NoError(t, service.ApplyToContract(context.Background(), contract, approvedAt))
	assert.Equal(t, 4000.0, contract.CommissionAmount)
	assert.Equal(t, models.CommissionOriginReferral, contract.CommissionOrigin)
	require.NotNil(t, contract.CommissionAccrualDate)
	assert.Equal(t, "2026-05-20", contract.CommissionAccrualDate.Format(models.HistoricalSaleDateLayout))

	contract.AssignmentOrigin = models.AssignmentOriginManual
	require.NoError(t, service.ApplyToContract(context.Background(), contract, approvedAt))
	assert.Equal(t, models.CommissionOriginDirect, contract.CommissionOrigin)

	contract.SellerID = nil
	require.NoError(t, service.ApplyToContract(context.Background(), contract, approvedAt))
	assert.Equal(t, 3000.0, contract.CommissionAmount, "unassigned contracts use 3%")
}

func TestCommissionRulesAndExceptionsCRUD(t *testing.T) {
	repo := newFakeCommissionRepository()
	seller := &models.User{ID: 9, Role: models.RoleSeller, FullName: "Carlos Vendedor", Status: models.StatusActive}
	service := NewCommissionService(repo, &fakeCommissionUserRepository{users: map[uint]*models.User{9: seller}}, nil)
	ctx := context.Background()

	rules, err := service.Rules(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5.0, rules.DefaultRate)
	assert.Equal(t, 3.0, rules.UnassignedRate)
	assert.Equal(t, models.CommissionPaymentRule, rules.PaymentRule)
	assert.Equal(t, 2, rules.AcceptanceWindowHours)

	defaultRate := 5.5
	unassigned := 4.0
	rules, err = service.UpdateRules(ctx, 1, CommissionRulesInput{DefaultRate: &defaultRate, UnassignedRate: &unassigned})
	require.NoError(t, err)
	assert.Equal(t, 5.5, rules.DefaultRate)
	assert.Equal(t, 4.0, rules.UnassignedRate)
	assert.Equal(t, 5.5, repo.settings.CommissionDefaultRate)

	invalid := 101.0
	_, err = service.UpdateRules(ctx, 1, CommissionRulesInput{DefaultRate: &invalid})
	assert.ErrorIs(t, err, ErrInvalidCommissionRate)

	exception, err := service.UpsertException(ctx, 1, 9, 6.5)
	require.NoError(t, err)
	assert.Equal(t, uint(9), exception.SellerID)
	assert.Equal(t, 6.5, exception.Rate)
	assert.Equal(t, "Carlos Vendedor", exception.SellerName)

	listed, err := service.ListExceptions(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)

	require.NoError(t, service.DeleteException(ctx, 1, exception.ID))
	assert.Empty(t, repo.exceptions)
	assert.ErrorIs(t, service.DeleteException(ctx, 1, exception.ID), ErrCommissionExceptionMissing)
}

func TestCommissionService_CreateCancellationCredit(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)

	paidAt := time.Now()
	sellerID := uint(7)
	contract := &models.Contract{
		ID:               11,
		SellerID:         &sellerID,
		CommissionAmount: 1500,
		CommissionPaidAt: &paidAt,
	}

	credit, err := service.CreateCancellationCredit(context.Background(), contract)

	require.NoError(t, err)
	require.NotNil(t, credit)
	assert.Equal(t, sellerID, credit.SellerID)
	assert.Equal(t, uint(11), credit.ContractID)
	assert.Equal(t, 1500.0, credit.Amount)
	assert.Equal(t, models.CommissionCreditStatusPending, credit.Status)
	require.Len(t, repo.credits, 1)
}

func TestCommissionService_CreateCancellationCredit_SkipsUnpaidCommission(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)

	sellerID := uint(7)
	contract := &models.Contract{
		ID:               11,
		SellerID:         &sellerID,
		CommissionAmount: 1500,
	}

	credit, err := service.CreateCancellationCredit(context.Background(), contract)

	require.NoError(t, err)
	assert.Nil(t, credit)
	assert.Empty(t, repo.credits, "a contract without a paid commission must not create a credit")
}

func TestCommissionService_CreateCommissionCreditForPreviousSeller(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)

	contract := &models.Contract{ID: 11}
	credit, err := service.CreateCommissionCredit(context.Background(), contract, 7, 1500)

	require.NoError(t, err)
	require.NotNil(t, credit)
	assert.Equal(t, uint(7), credit.SellerID)
	assert.Equal(t, uint(11), credit.ContractID)
	assert.Equal(t, 1500.0, credit.Amount)
	assert.Equal(t, models.CommissionCreditStatusPending, credit.Status)
	require.Len(t, repo.credits, 1)
}

func TestCommissionService_PayCommissionConsumesCreditsFIFO(t *testing.T) {
	repo := newFakeCommissionRepository()
	repo.contractSellers[88] = 7
	first := &models.SellerCommissionCredit{
		SellerID: 7, ContractID: 1, Amount: 3000,
		Status:    models.CommissionCreditStatusPending,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	second := &models.SellerCommissionCredit{
		SellerID: 7, ContractID: 2, Amount: 10000,
		Status:    models.CommissionCreditStatusPending,
		CreatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.CreateCredit(context.Background(), first))
	require.NoError(t, repo.CreateCredit(context.Background(), second))

	service := NewCommissionService(repo, nil, nil)
	payout, err := service.PayCommission(context.Background(), &models.Contract{ID: 88}, 9, 8000)
	require.NoError(t, err)
	require.NotNil(t, payout)
	assert.Equal(t, 8000.0, payout.Commission)
	assert.Equal(t, 8000.0, payout.AppliedTotal)
	assert.Equal(t, 0.0, payout.NetCash)
	assert.Equal(t, 5000.0, payout.RemainingCredit)
	require.Len(t, payout.Applications, 2)
	assert.Equal(t, uint(1), payout.Applications[0].CreditID)
	assert.Equal(t, 3000.0, payout.Applications[0].Amount)
	assert.Equal(t, uint(2), payout.Applications[1].CreditID)
	assert.Equal(t, 5000.0, payout.Applications[1].Amount)
	assert.Equal(t, 3000.0, repo.credits[1].AppliedAmount)
	assert.Equal(t, models.CommissionCreditStatusSettled, repo.credits[1].Status)
	assert.Equal(t, 5000.0, repo.credits[2].AppliedAmount)
	assert.Equal(t, models.CommissionCreditStatusPending, repo.credits[2].Status)
}

func TestCommissionService_PayCommissionNeverUsesAnotherSellersCredit(t *testing.T) {
	repo := newFakeCommissionRepository()
	repo.contractSellers[88] = 7
	otherSellerCredit := &models.SellerCommissionCredit{
		SellerID: 8, ContractID: 1, Amount: 10000,
		Status: models.CommissionCreditStatusPending,
	}
	require.NoError(t, repo.CreateCredit(context.Background(), otherSellerCredit))

	service := NewCommissionService(repo, nil, nil)
	payout, err := service.PayCommission(context.Background(), &models.Contract{ID: 88}, 9, 8000)
	require.NoError(t, err)
	require.NotNil(t, payout)
	assert.Equal(t, 0.0, payout.AppliedTotal)
	assert.Equal(t, 8000.0, payout.NetCash)
	assert.Empty(t, payout.Applications)
	assert.Equal(t, 0.0, repo.credits[otherSellerCredit.ID].AppliedAmount)
}

func TestCommissionService_RestoreAndCreateCreditUsesCashOnly(t *testing.T) {
	repo := newFakeCommissionRepository()
	repo.contractSellers[88] = 7
	credit := &models.SellerCommissionCredit{
		SellerID: 7, ContractID: 1, Amount: 3000,
		Status: models.CommissionCreditStatusSettled, AppliedAmount: 3000,
	}
	require.NoError(t, repo.CreateCredit(context.Background(), credit))
	repo.applications[credit.ID] = []models.CreditApplication{{
		ID: 1, CreditID: credit.ID, ContractID: 88, Amount: 3000,
		CreatedAt: time.Now(),
	}}

	service := NewCommissionService(repo, nil, nil)
	compensation, err := service.RestoreAndCreateCredit(context.Background(),
		&models.Contract{ID: 88}, 7, 8000, models.CommissionCreditSourceCancellation, 9, false)
	require.NoError(t, err)
	require.NotNil(t, compensation)
	assert.Equal(t, 3000.0, compensation.Restored)
	assert.Equal(t, 5000.0, compensation.NetCash)
	require.NotNil(t, compensation.Credit)
	assert.Equal(t, 5000.0, compensation.Credit.Amount)
	assert.Equal(t, models.CommissionCreditStatusPending, repo.credits[credit.ID].Status)
	assert.Equal(t, 0.0, repo.credits[credit.ID].AppliedAmount)
	assert.NotNil(t, repo.applications[credit.ID][0].ReversedAt)
}

func TestCommissionService_ManualApplyRejectedWhenCreditHasBalance(t *testing.T) {
	repo := newFakeCommissionRepository()
	credit := &models.SellerCommissionCredit{
		SellerID: 7, ContractID: 1, Amount: 3000,
		Status: models.CommissionCreditStatusPending,
	}
	require.NoError(t, repo.CreateCredit(context.Background(), credit))
	service := NewCommissionService(repo, nil, nil)

	_, err := service.ApplyCredit(context.Background(), 9, credit.ID)
	assert.ErrorIs(t, err, ErrCommissionCreditHasBalance)
	assert.Equal(t, models.CommissionCreditStatusPending, repo.credits[credit.ID].Status)
	assert.Equal(t, 0.0, repo.credits[credit.ID].AppliedAmount)
}

func TestCommissionService_PreviewWithoutSellerReturnsGross(t *testing.T) {
	service := NewCommissionService(newFakeCommissionRepository(), nil, nil)
	preview, err := service.PreviewCommissionPayout(context.Background(),
		&models.Contract{SellerID: nil}, 8000)
	require.NoError(t, err)
	assert.Equal(t, 8000.0, preview.Commission)
	assert.Equal(t, 8000.0, preview.NetCash)
	assert.Equal(t, 0.0, preview.AvailableCredit)
}

func TestCommissionService_ApplyCredit(t *testing.T) {
	repo := newFakeCommissionRepository()
	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	service := NewCommissionService(repo, nil, NewAuditService(gdb))

	seller := &models.User{ID: 7, FullName: "Ana Vendedora", Role: models.RoleSeller}
	require.NoError(t, repo.CreateCredit(context.Background(), &models.SellerCommissionCredit{
		SellerID:      seller.ID,
		ContractID:    11,
		Amount:        1500,
		AppliedAmount: 1500,
		Status:        models.CommissionCreditStatusPending,
	}))
	// FindCreditByID preloads the seller in production; mirror it in the fake.
	repo.credits[1].Seller = seller

	credit, err := service.ApplyCredit(context.Background(), 3, 1)

	require.NoError(t, err)
	assert.Equal(t, models.CommissionCreditStatusSettled, credit.Status)
	assert.NotNil(t, credit.SettledAt)
	require.NotNil(t, credit.SettledBy)
	assert.Equal(t, uint(3), *credit.SettledBy)
	assert.Equal(t, "Ana Vendedora", credit.Seller)
	assert.NoError(t, mockSQL.ExpectationsWereMet())
}

func TestCommissionService_ApplyCredit_IsIdempotent(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)
	require.NoError(t, repo.CreateCredit(context.Background(), &models.SellerCommissionCredit{
		SellerID:      7,
		ContractID:    11,
		Amount:        1500,
		AppliedAmount: 1500,
		Status:        models.CommissionCreditStatusPending,
	}))

	_, err := service.ApplyCredit(context.Background(), 3, 1)
	require.NoError(t, err)

	_, err = service.ApplyCredit(context.Background(), 3, 1)
	assert.ErrorIs(t, err, ErrCommissionCreditSettled)
}

func TestCommissionService_ApplyCredit_Missing(t *testing.T) {
	service := NewCommissionService(newFakeCommissionRepository(), nil, nil)

	_, err := service.ApplyCredit(context.Background(), 3, 99)

	assert.ErrorIs(t, err, ErrCommissionCreditMissing)
}

func TestCommissionService_ListCreditsFiltersSeller(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)
	for _, credit := range []*models.SellerCommissionCredit{
		{SellerID: 7, ContractID: 11, Amount: 500},
		{SellerID: 7, ContractID: 12, Amount: 250},
		{SellerID: 8, ContractID: 13, Amount: 900},
	} {
		require.NoError(t, repo.CreateCredit(context.Background(), credit))
	}

	all, err := service.ListCredits(context.Background(), 0)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	own, err := service.ListCredits(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, own, 2)
	for _, credit := range own {
		assert.Equal(t, uint(7), credit.SellerID)
	}
}
