package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/require"
)

func TestCommissionRepository_PayCommissionWithCreditsSerializesConcurrentPayouts(t *testing.T) {
	db := setupLiveDB(t)
	if !db.Migrator().HasTable(&models.CreditApplication{}) ||
		!db.Migrator().HasColumn(&models.SellerCommissionCredit{}, "applied_amount") {
		t.Skip("commission credit application migration is not applied")
	}

	contract := seedOverdueContract(t, db, 0)
	sellerID := contract.ApplicantUserID
	require.NoError(t, db.Model(&models.Contract{}).Where("id = ?", contract.ID).Updates(map[string]interface{}{
		"seller_id":         sellerID,
		"commission_amount": 800,
	}).Error)

	credit := &models.SellerCommissionCredit{
		SellerID:   sellerID,
		ContractID: contract.ID,
		Amount:     1000,
		Status:     models.CommissionCreditStatusPending,
		Source:     models.CommissionCreditSourceCancellation,
	}
	require.NoError(t, db.Create(credit).Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM credit_applications WHERE contract_id = ?", contract.ID)
		db.Exec("DELETE FROM seller_commission_credits WHERE id = ?", credit.ID)
	})

	repo := NewCommissionRepository(db)
	start := make(chan struct{})
	type result struct {
		payout *models.CommissionPayout
		err    error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			payout, err := repo.PayCommissionWithCredits(context.Background(), contract.ID, sellerID, 800)
			results <- result{payout: payout, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	succeeded, rejected := 0, 0
	for result := range results {
		if result.err == nil {
			succeeded++
			require.Equal(t, 800.0, result.payout.AppliedTotal)
			require.Equal(t, 0.0, result.payout.NetCash)
		} else if errors.Is(result.err, ErrCommissionAlreadyPaid) {
			rejected++
		} else {
			t.Fatalf("unexpected payout error: %v", result.err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, rejected)

	var persisted models.SellerCommissionCredit
	require.NoError(t, db.First(&persisted, credit.ID).Error)
	require.Equal(t, 800.0, persisted.AppliedAmount)
	require.Equal(t, models.CommissionCreditStatusPending, persisted.Status)
	var applicationCount int64
	require.NoError(t, db.Model(&models.CreditApplication{}).
		Where("credit_id = ? AND reversed_at IS NULL", credit.ID).
		Count(&applicationCount).Error)
	require.EqualValues(t, 1, applicationCount)

	compensation, err := repo.RestoreAndCreateCredit(context.Background(), CommissionCompensationInput{
		ContractID: contract.ID,
		SellerID:   sellerID,
		Gross:      800,
		Source:     models.CommissionCreditSourceReassignment,
		ActorID:    sellerID,
		ClearPaid:  true,
	})
	require.NoError(t, err)
	require.Equal(t, 800.0, compensation.Restored)
	require.Equal(t, 0.0, compensation.NetCash)
	require.Nil(t, compensation.Credit)
	require.NoError(t, db.First(&persisted, credit.ID).Error)
	require.Equal(t, 0.0, persisted.AppliedAmount)
	require.Equal(t, models.CommissionCreditStatusPending, persisted.Status)
	require.NoError(t, db.Model(&models.CreditApplication{}).
		Where("credit_id = ? AND reversed_at IS NOT NULL", credit.ID).
		Count(&applicationCount).Error)
	require.EqualValues(t, 1, applicationCount)
}
