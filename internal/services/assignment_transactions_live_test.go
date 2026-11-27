package services

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type assignmentTransactionFixture struct {
	contract  *models.Contract
	old       *models.User
	next      *models.User
	applicant *models.User
	admin     *models.User
}

func setupAssignmentTransactionLiveDB(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping live assignment transaction test")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	if !db.Migrator().HasTable(&models.Assignment{}) ||
		!db.Migrator().HasTable(&models.CreditApplication{}) {
		t.Skip("assignment or commission credit application schema is not migrated")
	}
	return db
}

func seedAssignmentTransactionFixture(t *testing.T, db *gorm.DB, status string, paid bool) assignmentTransactionFixture {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	makeUser := func(label, role string) *models.User {
		user := &models.User{
			Email:             fmt.Sprintf("atomic-%s-%s@example.com", label, suffix),
			EncryptedPassword: "test-hash",
			Identity:          fmt.Sprintf("atomic-%s-%s", label, suffix),
			RTN:               fmt.Sprintf("atomic-%s-%s", label, suffix),
			Role:              role,
			FullName:          fmt.Sprintf("Atomic %s", label),
			Status:            models.StatusActive,
			Score:             100,
			ScoreLevel:        models.ScoreLevelHigh,
		}
		require.NoError(t, db.Create(user).Error)
		return user
	}
	oldSeller := makeUser("old", models.RoleSeller)
	nextSeller := makeUser("next", models.RoleSeller)
	applicant := makeUser("applicant", models.RoleUser)
	admin := makeUser("admin", models.RoleAdmin)

	project := &models.Project{
		Name: "Atomic assignments " + suffix, Description: "fixture", Address: "fixture",
		LotCount: 1, PricePerSquareUnit: 100, InterestRate: 0,
		CommissionRate: 3, CommissionRateDirect: 3,
	}
	require.NoError(t, db.Create(project).Error)
	lot := &models.Lot{
		GUID: "atomic-lot-" + suffix, ProjectID: project.ID, Name: "AT-1",
		Length: 10, Width: 10, Price: 100000,
	}
	require.NoError(t, db.Create(lot).Error)

	amount := 100000.0
	contract := &models.Contract{
		LotID: lot.ID, ApplicantUserID: applicant.ID, PaymentTerm: 12,
		GUID: "atomic-contract-" + suffix, FinancingType: models.FinancingTypeDirect,
		Status: status, Active: true, Amount: &amount, CommissionAmount: 3000,
		SellerID: &oldSeller.ID, AssignmentOrigin: models.AssignmentOriginManual,
		AssignmentStatus: models.AssignmentStatusAccepted,
	}
	if status == models.ContractStatusPending || status == models.ContractStatusSubmitted {
		contract.AssignmentStatus = models.AssignmentStatusPending
	}
	if paid {
		paidAt := time.Now().UTC().Add(-time.Hour)
		contract.CommissionPaidAt = &paidAt
		contract.CommissionPaidBy = &admin.ID
	}
	require.NoError(t, db.Create(contract).Error)

	// Use per-seller rates to make a stale payout amount distinguishable from a
	// payout performed after reassignment.
	require.NoError(t, db.Create(&models.SellerCommissionRate{SellerID: oldSeller.ID, Rate: 3}).Error)
	require.NoError(t, db.Create(&models.SellerCommissionRate{SellerID: nextSeller.ID, Rate: 5}).Error)

	t.Cleanup(func() {
		db.Exec("DELETE FROM notifications WHERE user_id IN ?", []uint{oldSeller.ID, nextSeller.ID, applicant.ID, admin.ID})
		db.Exec("DELETE FROM audit_logs WHERE user_id IN ?", []uint{oldSeller.ID, nextSeller.ID, applicant.ID, admin.ID})
		db.Exec("DELETE FROM credit_applications WHERE contract_id = ?", contract.ID)
		db.Exec("DELETE FROM seller_commission_credits WHERE contract_id = ? OR seller_id IN ?", contract.ID, []uint{oldSeller.ID, nextSeller.ID})
		db.Exec("DELETE FROM assignments WHERE contract_id = ?", contract.ID)
		db.Exec("DELETE FROM contracts WHERE id = ?", contract.ID)
		db.Exec("DELETE FROM lots WHERE id = ?", lot.ID)
		db.Exec("DELETE FROM projects WHERE id = ?", project.ID)
		db.Exec("DELETE FROM seller_commission_rates WHERE seller_id IN ?", []uint{oldSeller.ID, nextSeller.ID})
		db.Exec("DELETE FROM users WHERE id IN ?", []uint{oldSeller.ID, nextSeller.ID, applicant.ID, admin.ID})
	})

	return assignmentTransactionFixture{
		contract: contract, old: oldSeller, next: nextSeller, applicant: applicant, admin: admin,
	}
}

func newLiveAssignmentServices(db *gorm.DB) (*ContractService, *AssignmentService, *repository.Repositories) {
	repos := repository.NewRepositories(db)
	audit := NewAuditService(db)
	notifications := NewNotificationService(repos.Notification, repos.User)
	assignments := NewAssignmentService(repos.Assignment, repos.Contract, repos.User, notifications, audit)
	assignments.SetCommissionRepo(repos.Commission)
	assignments.SetTransactionRunner(repos.RunInTx)
	commissions := NewCommissionService(repos.Commission, repos.User, audit)
	contracts := NewContractService(
		repos.Contract, repos.Lot, repos.User, repos.Payment, repos.Ledger,
		notifications, nil, audit, nil,
	)
	contracts.SetAssignmentServices(nil, assignments)
	contracts.SetCommissionService(commissions)
	contracts.SetTransactionRunner(repos.RunInTx)
	return contracts, assignments, repos
}

func TestContractService_ReassignRollsBackCreditAssignmentAndSideEffectsLive(t *testing.T) {
	db := setupAssignmentTransactionLiveDB(t)
	fixture := seedAssignmentTransactionFixture(t, db, models.ContractStatusApproved, true)
	oldAssignment := &models.Assignment{
		ContractID: fixture.contract.ID, SellerID: fixture.old.ID,
		Origin: models.AssignmentOriginManual, Status: models.AssignmentStatusPending,
		AssignedAt: time.Now().UTC(), AcceptDeadlineAt: nil,
	}
	require.NoError(t, db.Create(oldAssignment).Error)
	credit := &models.SellerCommissionCredit{
		SellerID: fixture.old.ID, ContractID: fixture.contract.ID, Amount: 1000,
		AppliedAmount: 1000, Status: models.CommissionCreditStatusSettled,
		Source: models.CommissionCreditSourceCancellation,
	}
	require.NoError(t, db.Create(credit).Error)
	application := &models.CreditApplication{
		CreditID: credit.ID, ContractID: fixture.contract.ID, Amount: 1000,
		ActorID: &fixture.admin.ID,
	}
	require.NoError(t, db.Create(application).Error)

	functionName := fmt.Sprintf("test_fail_atomic_assignment_%d", time.Now().UnixNano())
	triggerName := "trigger_" + functionName
	require.NoError(t, db.Exec(fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.id = %d AND NEW.seller_id = %d THEN
				RAISE EXCEPTION 'injected final contract update failure';
			END IF;
			RETURN NEW;
		END $$`, functionName, fixture.contract.ID, fixture.next.ID)).Error)
	require.NoError(t, db.Exec(fmt.Sprintf(
		"CREATE TRIGGER %s BEFORE UPDATE ON contracts FOR EACH ROW EXECUTE FUNCTION %s()",
		triggerName, functionName,
	)).Error)
	t.Cleanup(func() {
		db.Exec(fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON contracts", triggerName))
		db.Exec(fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", functionName))
	})

	contractService, _, _ := newLiveAssignmentServices(db)
	_, _, err := contractService.Reassign(context.Background(), fixture.contract.GUID, ContractReassignInput{
		Mode: ReassignModeSeller, SellerID: &fixture.next.ID, Accept: true, ActorID: fixture.admin.ID,
	})
	require.ErrorContains(t, err, "injected final contract update failure")

	var persistedContract models.Contract
	require.NoError(t, db.First(&persistedContract, fixture.contract.ID).Error)
	require.Equal(t, fixture.old.ID, *persistedContract.SellerID)
	require.NotNil(t, persistedContract.CommissionPaidAt)

	var persistedCredit models.SellerCommissionCredit
	require.NoError(t, db.First(&persistedCredit, credit.ID).Error)
	require.Equal(t, 1000.0, persistedCredit.AppliedAmount)
	require.Equal(t, models.CommissionCreditStatusSettled, persistedCredit.Status)

	var persistedApplication models.CreditApplication
	require.NoError(t, db.First(&persistedApplication, application.ID).Error)
	require.Nil(t, persistedApplication.ReversedAt)

	var assignments []models.Assignment
	require.NoError(t, db.Where("contract_id = ?", fixture.contract.ID).Find(&assignments).Error)
	require.Len(t, assignments, 1)
	require.Equal(t, models.AssignmentStatusPending, assignments[0].Status)

	var createdCredits int64
	require.NoError(t, db.Model(&models.SellerCommissionCredit{}).
		Where("contract_id = ? AND source = ?", fixture.contract.ID, models.CommissionCreditSourceReassignment).
		Count(&createdCredits).Error)
	require.Zero(t, createdCredits)

	var assignmentAudits int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity = ? AND user_id IN ?", "Assignment", []uint{fixture.old.ID, fixture.next.ID, fixture.admin.ID}).
		Where("created_at > ?", time.Now().Add(-time.Minute)).
		Count(&assignmentAudits).Error)
	require.Zero(t, assignmentAudits)
	var notifications int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND notification_type = ?", fixture.next.ID, models.NotificationTypeAssignmentAccepted).
		Count(&notifications).Error)
	require.Zero(t, notifications)
}

func TestContractService_PayCommissionSerializesWithReassignLive(t *testing.T) {
	db := setupAssignmentTransactionLiveDB(t)
	fixture := seedAssignmentTransactionFixture(t, db, models.ContractStatusApproved, false)
	credit := &models.SellerCommissionCredit{
		SellerID: fixture.old.ID, ContractID: fixture.contract.ID, Amount: 1000,
		Status: models.CommissionCreditStatusPending, Source: models.CommissionCreditSourceCancellation,
	}
	require.NoError(t, db.Create(credit).Error)
	contractService, _, _ := newLiveAssignmentServices(db)

	start := make(chan struct{})
	type payResult struct {
		contract *models.Contract
		payout   *models.CommissionPayout
		err      error
	}
	payDone := make(chan payResult, 1)
	reassignDone := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		contract, payout, err := contractService.MarkCommissionPaid(context.Background(), fixture.contract.ID, fixture.admin.ID)
		payDone <- payResult{contract: contract, payout: payout, err: err}
	}()
	go func() {
		defer wg.Done()
		<-start
		_, _, err := contractService.Reassign(context.Background(), fixture.contract.GUID, ContractReassignInput{
			Mode: ReassignModeSeller, SellerID: &fixture.next.ID, Accept: true, ActorID: fixture.admin.ID,
		})
		reassignDone <- err
	}()
	close(start)
	wg.Wait()
	close(payDone)
	close(reassignDone)

	pay := <-payDone
	reassignErr := <-reassignDone
	require.NoError(t, pay.err)
	require.NoError(t, reassignErr)
	require.NotNil(t, pay.contract)
	require.NotNil(t, pay.payout)

	var final models.Contract
	require.NoError(t, db.First(&final, fixture.contract.ID).Error)
	require.Equal(t, fixture.next.ID, *final.SellerID)

	// If payout wins, reassignment restores the credit and clears the paid
	// markers. If reassignment wins, payout uses the locked new seller and its
	// frozen gross commission rather than the pre-lock snapshot.
	if final.CommissionPaidAt == nil {
		require.Equal(t, 3000.0, final.CommissionAmount)
		var pendingTotal float64
		require.NoError(t, db.Model(&models.SellerCommissionCredit{}).
			Select("COALESCE(SUM(amount - applied_amount), 0)").
			Where("seller_id = ? AND status = ?", fixture.old.ID, models.CommissionCreditStatusPending).
			Scan(&pendingTotal).Error)
		require.Equal(t, 3000.0, pendingTotal)
		require.Equal(t, fixture.old.ID, *pay.payout.SellerID)
		require.Equal(t, 1000.0, pay.payout.AppliedTotal)
	} else {
		require.Equal(t, 5000.0, final.CommissionAmount)
		require.Equal(t, fixture.next.ID, *pay.contract.SellerID)
		require.Equal(t, 5000.0, pay.payout.Commission)
		require.Zero(t, pay.payout.AppliedTotal)
	}
}

func TestAssignmentService_AcceptSerializesWithReassignLive(t *testing.T) {
	db := setupAssignmentTransactionLiveDB(t)
	fixture := seedAssignmentTransactionFixture(t, db, models.ContractStatusPending, false)
	_, assignmentService, _ := newLiveAssignmentServices(db)
	deadline := time.Now().UTC().Add(time.Hour)
	pending := &models.Assignment{
		ContractID: fixture.contract.ID, SellerID: fixture.old.ID,
		Origin: models.AssignmentOriginManual, Status: models.AssignmentStatusPending,
		AssignedAt: time.Now().UTC(), AcceptDeadlineAt: &deadline,
	}
	require.NoError(t, db.Create(pending).Error)
	contractService, _, _ := newLiveAssignmentServices(db)

	start := make(chan struct{})
	acceptDone := make(chan error, 1)
	reassignDone := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err := assignmentService.Accept(context.Background(), pending.ID, fixture.old.ID, false)
		acceptDone <- err
	}()
	go func() {
		defer wg.Done()
		<-start
		_, _, err := contractService.Reassign(context.Background(), fixture.contract.GUID, ContractReassignInput{
			Mode: ReassignModeSeller, SellerID: &fixture.next.ID, Accept: false, ActorID: fixture.admin.ID,
		})
		reassignDone <- err
	}()
	close(start)
	wg.Wait()

	acceptErr := <-acceptDone
	reassignErr := <-reassignDone
	require.NoError(t, reassignErr)
	if acceptErr != nil {
		require.ErrorIs(t, acceptErr, ErrAssignmentNotPending)
	}

	var final models.Contract
	require.NoError(t, db.First(&final, fixture.contract.ID).Error)
	require.Equal(t, fixture.next.ID, *final.SellerID)
	require.Equal(t, models.AssignmentStatusPending, final.AssignmentStatus)

	var current []models.Assignment
	require.NoError(t, db.Where("contract_id = ? AND seller_id = ? AND status = ?",
		fixture.contract.ID, fixture.next.ID, models.AssignmentStatusPending).Find(&current).Error)
	require.Len(t, current, 1)
}
