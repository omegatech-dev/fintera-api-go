package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractRepository_FindActiveByLot(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewContractRepository(db)

	mock.ExpectQuery(`WHERE lot_id = \$1 AND active = \$2`).
		WithArgs(uint(3), true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "lot_id", "active", "status"}).
			AddRow(7, 3, true, models.ContractStatusApproved))

	contract, err := repo.FindActiveByLot(context.Background(), 3)

	require.NoError(t, err)
	require.NotNil(t, contract)
	assert.Equal(t, uint(7), contract.ID)
	assert.Equal(t, uint(3), contract.LotID)
	assert.True(t, contract.Active)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContractRepository_FindPendingReservations(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewContractRepository(db)

	mock.ExpectQuery(`WHERE contracts\.status = \$1 AND contracts\.created_at < NOW\(\) - INTERVAL '48 hours'`).
		WithArgs(models.ContractStatusSubmitted).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))

	contracts, err := repo.FindPendingReservations(context.Background(), 48)

	require.NoError(t, err)
	assert.Empty(t, contracts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContractRepository_FindReservationsAwaitingVoucher(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewContractRepository(db)

	mock.ExpectQuery(`JOIN payments ON payments\.contract_id = contracts\.id AND payments\.payment_type = \$1 AND payments\.status = \$2`).
		WithArgs(models.PaymentTypeReservation, models.PaymentStatusPending, models.ContractStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))

	contracts, err := repo.FindReservationsAwaitingVoucher(context.Background(), 40)

	require.NoError(t, err)
	assert.Empty(t, contracts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContractRepository_GetStats(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewContractRepository(db)

	mock.ExpectQuery(`SELECT status, count\(\*\) as count FROM "contracts" GROUP BY "status"`).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow(models.ContractStatusPending, 3).
			AddRow(models.ContractStatusApproved, 5).
			AddRow(models.ContractStatusRejected, 2).
			AddRow(models.ContractStatusCancelled, 1))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "contracts" WHERE active = \$1`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(payments.amount\), 0\) FROM "payments" JOIN contracts ON contracts.id = payments.contract_id WHERE contracts.active = \$1 AND payments.status IN \(\$2,\$3\)`).
		WithArgs(true, models.PaymentStatusPending, models.PaymentStatusSubmitted).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(2100000))

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(payments.amount\), 0\) as total, COUNT\(\*\) as count FROM "payments" JOIN contracts ON contracts.id = payments.contract_id WHERE contracts.active = \$1 AND payments.status = \$2 AND payments.due_date < CURRENT_DATE`).
		WithArgs(true, models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(1980000, 12))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "lots" WHERE status = \$1 AND NOT EXISTS`).
		WithArgs(models.LotStatusReserved).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	stats, err := repo.GetStats(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(11), stats.Total)
	assert.Equal(t, int64(3), stats.Pending)
	assert.Equal(t, int64(5), stats.Approved)
	assert.Equal(t, int64(2), stats.Rejected)
	assert.Equal(t, int64(1), stats.Cancelled)
	assert.Equal(t, int64(11), stats.Active)
	assert.Equal(t, float64(2100000), stats.PendingBalance)
	assert.Equal(t, float64(1980000), stats.Overdue)
	assert.Equal(t, int64(12), stats.OverdueCount)
	assert.Equal(t, int64(1), stats.ReservationsWithoutContract)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContractRepository_LoadNextPayments(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewContractRepository(db).(*contractRepository)

	contracts := []models.Contract{{ID: 7}, {ID: 8}, {ID: 9}}
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT \* FROM "payments" WHERE contract_id IN \(\$1,\$2,\$3\) AND status IN \(\$4,\$5\) AND \(\(document_path IS NULL OR document_path = ''\)\) ORDER BY due_date ASC, id ASC`).
		WithArgs(uint(7), uint(8), uint(9), models.PaymentStatusPending, models.PaymentStatusSubmitted).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contract_id", "amount", "due_date", "status", "payment_type", "description"}).
			AddRow(101, 7, 50000, due, models.PaymentStatusPending, models.PaymentTypeReservation, "Pago de Reserva").
			AddRow(102, 7, 54166, due.AddDate(0, 1, 0), models.PaymentStatusPending, models.PaymentTypeInstallment, "Cuota 1 de 12").
			AddRow(103, 8, 90000, due, models.PaymentStatusPending, models.PaymentTypeDownPayment, "Pago Inicial"))

	err := repo.loadNextPayments(context.Background(), contracts)

	require.NoError(t, err)
	require.NotNil(t, contracts[0].NextPayment)
	assert.Equal(t, uint(101), contracts[0].NextPayment.ID)
	require.NotNil(t, contracts[1].NextPayment)
	assert.Equal(t, uint(103), contracts[1].NextPayment.ID)
	assert.Nil(t, contracts[2].NextPayment)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContractRepository_LoadNextPayments_QueryError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewContractRepository(db).(*contractRepository)

	mock.ExpectQuery(`SELECT \* FROM "payments"`).
		WillReturnError(assert.AnError)

	err := repo.loadNextPayments(context.Background(), []models.Contract{{ID: 7}})

	require.ErrorIs(t, err, assert.AnError)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentRepository_FindOverdue(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`WHERE payments\.status = \$1 AND payments\.due_date < CURRENT_DATE`).
		WithArgs(models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))

	payments, err := repo.FindOverdue(context.Background())

	require.NoError(t, err)
	assert.Empty(t, payments)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentRepository_FindOverdueForActiveContracts(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`JOIN contracts ON contracts\.id = payments\.contract_id AND contracts\.status = \$1 AND contracts\.active = \$2`).
		WithArgs(models.ContractStatusApproved, true, models.StatusActive, models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))

	payments, err := repo.FindOverdueForActiveContracts(context.Background())

	require.NoError(t, err)
	assert.Empty(t, payments)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentRepository_FindPaymentsDueTomorrowForActiveContracts(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`JOIN contracts ON contracts\.id = payments\.contract_id AND contracts\.status = \$1 AND contracts\.active = \$2`).
		WithArgs(models.ContractStatusApproved, true, models.StatusActive, models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))

	payments, err := repo.FindPaymentsDueTomorrowForActiveContracts(context.Background())

	require.NoError(t, err)
	assert.Empty(t, payments)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectRepository_ListPublished(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewProjectRepository(db)

	mock.ExpectQuery(`WHERE is_published = \$1 AND is_active = \$2`).
		WithArgs(true, true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	projects, err := repo.ListPublished(context.Background())

	require.NoError(t, err)
	assert.Empty(t, projects)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectRepository_FindPublishedByGUID(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewProjectRepository(db)

	mock.ExpectQuery(`WHERE guid = \$1 AND is_published = \$2 AND is_active = \$3`).
		WithArgs("project-guid", true, true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	project, err := repo.FindPublishedByGUID(context.Background(), "project-guid")

	require.Error(t, err)
	assert.Nil(t, project)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLotRepository_ReserveIfAvailable(t *testing.T) {
	t.Run("wins the reservation", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLotRepository(db)

		mock.ExpectExec(`UPDATE "lots" SET "status"=\$1,"updated_at"=\$2 WHERE id = \$3 AND status = \$4`).
			WithArgs(models.LotStatusReserved, sqlmock.AnyArg(), uint(3), models.LotStatusAvailable).
			WillReturnResult(sqlmock.NewResult(0, 1))

		reserved, err := repo.ReserveIfAvailable(context.Background(), 3)

		require.NoError(t, err)
		assert.True(t, reserved)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("already reserved", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLotRepository(db)

		mock.ExpectExec(`UPDATE "lots" SET "status"=\$1,"updated_at"=\$2 WHERE id = \$3 AND status = \$4`).
			WithArgs(models.LotStatusReserved, sqlmock.AnyArg(), uint(3), models.LotStatusAvailable).
			WillReturnResult(sqlmock.NewResult(0, 0))

		reserved, err := repo.ReserveIfAvailable(context.Background(), 3)

		require.NoError(t, err)
		assert.False(t, reserved)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
