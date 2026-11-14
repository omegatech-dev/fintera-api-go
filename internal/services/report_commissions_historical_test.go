package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCommissionsIncludesHistoricalOriginAndScore(t *testing.T) {
	amount := 100000.0
	approved := time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC)
	sellerID := uint(7)

	contractRepo := &mockContractRepository{
		mockList: func(context.Context, *repository.ContractQuery) ([]models.Contract, int64, error) {
			return []models.Contract{{
				ID:                    1,
				GUID:                  "contract-1",
				Status:                models.ContractStatusApproved,
				FinancingType:         models.FinancingTypeDirect,
				Amount:                &amount,
				CommissionAmount:      4000,
				CommissionOrigin:      models.CommissionOriginDirect,
				CommissionAccrualDate: &approved,
				ApprovedAt:            &approved,
				SellerID:              &sellerID,
				Seller:                &models.User{ID: sellerID, FullName: "Ana Vendedora", Role: models.RoleSeller},
				ApplicantUser:         models.User{ID: 20, FullName: "Cliente Reserva"},
				Lot: models.Lot{
					ID:   1,
					Name: "Lote 1",
					Project: models.Project{
						ID:   1,
						Name: "Proyecto Uno",
					},
				},
			}}, 1, nil
		},
	}

	inside := models.HistoricalSale{
		ID:               11,
		ClientName:       "Cliente Histórico",
		ProjectName:      "Proyecto Histórico",
		SalePrice:        200000,
		SaleDate:         time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
		CommissionAmount: 5000,
		CommissionStatus: "paid",
		ValidationStatus: models.HistoricalSaleStatusValidated,
		CreatedBy:        sellerID,
		Creator:          &models.User{ID: sellerID, FullName: "Ana Vendedora", Role: models.RoleSeller},
	}
	outside := models.HistoricalSale{
		ID:               12,
		ClientName:       "Fuera del período",
		SalePrice:        1000,
		SaleDate:         time.Date(2022, 12, 31, 0, 0, 0, 0, time.UTC),
		CommissionAmount: 50,
		CommissionStatus: "pending",
		ValidationStatus: models.HistoricalSaleStatusPending,
		CreatedBy:        sellerID,
	}
	historicalRepo := newFakeHistoricalSaleRepository()
	historicalRepo.sales = []models.HistoricalSale{inside, outside}

	userRepo := &fakeCommissionUserRepository{users: map[uint]*models.User{
		sellerID: {
			ID:         sellerID,
			Role:       models.RoleSeller,
			FullName:   "Ana Vendedora",
			Score:      92,
			ScoreLevel: models.ScoreLevelHigh,
		},
	}}
	assignmentRepo := newFakeAssignmentRepo()
	assignmentRepo.outcomes = []models.ScoreOutcome{{SellerID: sellerID, Accepted: 12, Expired: 1}}
	scoreSvc := NewScoreService(nil, assignmentRepo, nil)

	service := NewReportService(nil, contractRepo, userRepo, nil)
	service.SetHistoricalSaleRepository(historicalRepo)
	service.SetScoreService(scoreSvc)

	items, err := service.GenerateCommissions(context.Background(), "2023-01-01", "2023-12-31", 0, false)
	require.NoError(t, err)
	require.Len(t, items, 2, "the out-of-period historical sale must be excluded")

	var historical *CommissionReportItem
	var reservation *CommissionReportItem
	for i := range items {
		if items[i].IsHistorical {
			historical = &items[i]
		} else {
			reservation = &items[i]
		}
	}
	require.NotNil(t, historical)
	require.NotNil(t, reservation)

	assert.Equal(t, models.CommissionOriginHistorical, historical.Origin)
	assert.InDelta(t, 5000.0, historical.Commission, 0.01)
	assert.Equal(t, "2023-06-15", historical.Date)
	require.NotNil(t, historical.CommissionPaidAt)
	assert.Equal(t, "Cliente Histórico", historical.ClientName)
	require.NotNil(t, historical.Score)
	assert.Equal(t, 92, *historical.Score)
	require.NotNil(t, historical.ScoreMetrics)
	assert.Equal(t, int64(12), historical.ScoreMetrics.AcceptedCount)
	assert.Equal(t, int64(1), historical.ScoreMetrics.ExpiredCount)

	assert.Equal(t, models.CommissionOriginDirect, reservation.Origin)
	assert.Equal(t, "2023-05-10", reservation.Date)
	require.NotNil(t, reservation.Score)
	assert.Equal(t, 92, *reservation.Score)
	assert.Equal(t, models.ScoreLevelHigh, *reservation.ScoreLevel)
}

func TestGenerateCommissionsFiltersReservationsByAccrualDate(t *testing.T) {
	amount := 100000.0
	approved := time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC)
	accrual := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	contractRepo := &mockContractRepository{
		mockList: func(context.Context, *repository.ContractQuery) ([]models.Contract, int64, error) {
			return []models.Contract{{
				ID:                    1,
				Amount:                &amount,
				CommissionAmount:      4000,
				CommissionOrigin:      models.CommissionOriginDirect,
				CommissionAccrualDate: &accrual,
				ApprovedAt:            &approved,
			}}, 1, nil
		},
	}
	service := NewReportService(nil, contractRepo, nil, nil)

	items, err := service.GenerateCommissions(context.Background(), "2023-01-01", "2023-12-31", 0, false)
	require.NoError(t, err)
	assert.Empty(t, items, "the commission belongs to 2024, not to its approval year")
}
