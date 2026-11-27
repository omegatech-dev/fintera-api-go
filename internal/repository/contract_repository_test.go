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

	// DISTINCT ON makes the database return at most one payment per contract:
	// the earliest due_date, then lowest id.
	mock.ExpectQuery(`SELECT DISTINCT ON \(contract_id\) payments\.\* FROM "payments" WHERE contract_id IN \(\$1,\$2,\$3\) AND status IN \(\$4,\$5\) AND \(\(document_path IS NULL OR document_path = ''\)\) ORDER BY contract_id ASC, due_date ASC, id ASC`).
		WithArgs(uint(7), uint(8), uint(9), models.PaymentStatusPending, models.PaymentStatusSubmitted).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contract_id", "amount", "due_date", "status", "payment_type", "description"}).
			AddRow(101, 7, 50000, due, models.PaymentStatusPending, models.PaymentTypeReservation, "Pago de Reserva").
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

	mock.ExpectQuery(`SELECT DISTINCT ON \(contract_id\) payments\.\* FROM "payments"`).
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

func TestPaymentRepository_FindOverdueInBatchesKeysetsUntilShortBatch(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	overdueRows := func(ids ...uint) *sqlmock.Rows {
		rows := sqlmock.NewRows([]string{"id", "contract_id", "status", "due_date", "amount", "payment_type"})
		for _, id := range ids {
			rows.AddRow(id, uint(7), models.PaymentStatusPending,
				time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 1000.0, models.PaymentTypeInstallment)
		}
		return rows
	}
	// Contracts are returned without lot/user foreign keys so the nested
	// preloads are skipped and the mock stays focused on the batch loop.
	contractRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "lot_id", "applicant_user_id"}).AddRow(7, 0, 0)
	}

	mock.ExpectQuery(`payments\.id > \$2`).
		WithArgs(models.PaymentStatusPending, uint(0), sqlmock.AnyArg()).
		WillReturnRows(overdueRows(1, 2))
	mock.ExpectQuery(`FROM "contracts"`).
		WithArgs(uint(7)).
		WillReturnRows(contractRows())

	// The second batch is short (1 < 2), so the loop must stop without a
	// third query.
	mock.ExpectQuery(`payments\.id > \$2`).
		WithArgs(models.PaymentStatusPending, uint(2), sqlmock.AnyArg()).
		WillReturnRows(overdueRows(3))
	mock.ExpectQuery(`FROM "contracts"`).
		WithArgs(uint(7)).
		WillReturnRows(contractRows())

	var seen [][]uint
	err := repo.FindOverdueInBatches(context.Background(), 2, func(batch []models.Payment) error {
		ids := make([]uint, 0, len(batch))
		for i := range batch {
			ids = append(ids, batch[i].ID)
		}
		seen = append(seen, ids)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, [][]uint{{1, 2}, {3}}, seen)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentRepository_FindOverdueInBatchesStopsOnCallbackError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`payments\.id > \$2`).
		WithArgs(models.PaymentStatusPending, uint(0), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contract_id", "status", "due_date", "amount", "payment_type"}).
			AddRow(uint(1), uint(7), models.PaymentStatusPending, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 1000.0, models.PaymentTypeInstallment).
			AddRow(uint(2), uint(7), models.PaymentStatusPending, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 1000.0, models.PaymentTypeInstallment))
	mock.ExpectQuery(`FROM "contracts"`).
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "lot_id", "applicant_user_id"}).AddRow(7, 0, 0))

	calls := 0
	err := repo.FindOverdueInBatches(context.Background(), 2, func([]models.Payment) error {
		calls++
		return assert.AnError
	})

	require.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, 1, calls, "no further callback after the first error")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentRepository_FindOverdueInBatchesEmptyResult(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`payments\.id > \$2`).
		WithArgs(models.PaymentStatusPending, uint(0), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contract_id", "status", "due_date", "amount", "payment_type"}))

	calls := 0
	err := repo.FindOverdueInBatches(context.Background(), 2, func([]models.Payment) error {
		calls++
		return nil
	})

	require.NoError(t, err)
	assert.Zero(t, calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentRepository_ListProjectAndSellerScopeJoinSQL(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	query := NewListQuery()
	query.Filters["status"] = models.PaymentStatusPaid
	query.Filters["project_id"] = "7"
	query.Filters["seller_id"] = "12"

	// Count clone must see the same joins and predicates.
	mock.ExpectQuery(`SELECT count\(\*\).*JOIN contracts AS rep_c ON rep_c\.id = payments\.contract_id JOIN lots AS rep_l ON rep_l\.id = rep_c\.lot_id`).
		WithArgs(models.PaymentStatusPaid, "12", "12", "7").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Empty page: preloads are skipped, so exactly two queries run.
	mock.ExpectQuery(`SELECT payments\.\* FROM "payments" JOIN contracts AS rep_c ON rep_c\.id = payments\.contract_id JOIN lots AS rep_l ON rep_l\.id = rep_c\.lot_id.*\(rep_c\.seller_id = \$2 OR rep_c\.creator_id = \$3\).*AND rep_l\.project_id = \$4`).
		WithArgs(models.PaymentStatusPaid, "12", "12", "7", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	payments, total, err := repo.List(context.Background(), query)

	require.NoError(t, err)
	assert.Empty(t, payments)
	assert.Equal(t, int64(0), total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContractRepository_StreamForCommissionsKeysetsWithCursor(t *testing.T) {
	db, mock := setupMockDB(t)
	mock.MatchExpectationsInOrder(false)
	repo := NewContractRepository(db)

	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	contractRows := func(ids ...uint) *sqlmock.Rows {
		rows := sqlmock.NewRows([]string{"id", "status", "created_at", "applicant_user_id", "lot_id", "seller_id", "creator_id"})
		for _, id := range ids {
			rows.AddRow(id, models.ContractStatusApproved, created.AddDate(0, 0, -int(id)), uint(0), uint(0), nil, nil)
		}
		return rows
	}
	// BelongsTo preloads fire even for zero foreign keys (users, lots); the
	// NULL seller/creator pointers skip theirs, and empty lot rows skip the
	// nested project preload.
	emptyUsers := func() *sqlmock.Rows { return sqlmock.NewRows([]string{"id"}) }
	emptyLots := func() *sqlmock.Rows { return sqlmock.NewRows([]string{"id", "project_id"}) }

	query := &ContractQuery{
		ListQuery: NewListQuery(),
		IsAdmin:   true,
	}
	query.Filters["status_in"] = models.ContractStatusApproved + "," + models.ContractStatusClosed

	// First batch: no cursor predicate, composite DESC ordering to preserve
	// List's created_at DESC report order.
	mock.ExpectQuery(`SELECT \* FROM "contracts" WHERE contracts\.status IN \(\$1,\$2\) ORDER BY contracts\.created_at DESC, contracts\.id DESC LIMIT \$3`).
		WithArgs(models.ContractStatusApproved, models.ContractStatusClosed, sqlmock.AnyArg()).
		WillReturnRows(contractRows(11, 12))
	mock.ExpectQuery(`FROM "users"`).WithArgs(uint(0)).WillReturnRows(emptyUsers())
	mock.ExpectQuery(`FROM "users"`).WithArgs(uint(0)).WillReturnRows(emptyUsers())
	mock.ExpectQuery(`FROM "lots"`).WithArgs(uint(0)).WillReturnRows(emptyLots())
	mock.ExpectQuery(`FROM "lots"`).WithArgs(uint(0)).WillReturnRows(emptyLots())

	// Second batch: row-value keyset cursor; its short result stops the loop.
	mock.ExpectQuery(`WHERE contracts\.status IN \(\$1,\$2\) AND \(contracts\.created_at, contracts\.id\) < \(\$3,\s*\$4\) ORDER BY contracts\.created_at DESC, contracts\.id DESC LIMIT \$5`).
		WithArgs(models.ContractStatusApproved, models.ContractStatusClosed, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(contractRows(13))

	var seen []uint
	err := repo.StreamForCommissions(context.Background(), query, 2, func(batch []models.Contract) error {
		for i := range batch {
			seen = append(seen, batch[i].ID)
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []uint{11, 12, 13}, seen)
}

func TestPaymentRepository_FindOverdueRemindersUsesSingleJoinedQuery(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	due := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT payments\.id AS payment_id.*JOIN contracts ON contracts\.id = payments\.contract_id.*JOIN users ON users\.id = contracts\.applicant_user_id.*JOIN lots ON lots\.id = contracts\.lot_id JOIN projects ON projects\.id = lots\.project_id WHERE \(payments\.status = \$4 AND payments\.due_date < CURRENT_DATE\)`).
		WithArgs(models.ContractStatusApproved, true, models.StatusActive, models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{
			"payment_id", "contract_id", "amount", "due_date", "payment_status",
			"overdue_reminder_sent_at", "upcoming_reminder_sent_at",
			"applicant_user_id", "lot_id", "user_id", "user_full_name", "user_email",
			"lot_name", "project_id", "project_name",
		}).AddRow(501, 40, 1250.75, due, models.PaymentStatusPending, nil, nil,
			7, 12, 7, "Ana Diaz", "ana@example.com", "Lote 12", 3, "Altos de la Sierra"))

	payments, err := repo.FindOverdueReminders(context.Background())

	require.NoError(t, err)
	require.Len(t, payments, 1)
	payment := payments[0]
	assert.Equal(t, uint(501), payment.ID)
	assert.Equal(t, uint(40), payment.ContractID)
	assert.InDelta(t, 1250.75, payment.Amount, 0.001)
	assert.Equal(t, due, payment.DueDate)
	assert.Equal(t, uint(7), payment.Contract.ApplicantUserID)
	assert.Equal(t, "Lote 12", payment.Contract.Lot.Name)
	assert.Equal(t, "Altos de la Sierra", payment.Contract.Lot.Project.Name)
	assert.Equal(t, "ana@example.com", payment.Contract.ApplicantUser.Email)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentRepository_FindUpcomingRemindersUsesSingleJoinedQuery(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewPaymentRepository(db)

	due := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT payments\.id AS payment_id.*JOIN lots ON lots\.id = contracts\.lot_id JOIN projects ON projects\.id = lots\.project_id WHERE \(payments\.status = \$4 AND payments\.due_date <= CURRENT_DATE \+ INTERVAL '3 days'\)`).
		WithArgs(models.ContractStatusApproved, true, models.StatusActive, models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{
			"payment_id", "contract_id", "amount", "due_date", "payment_status",
			"overdue_reminder_sent_at", "upcoming_reminder_sent_at",
			"applicant_user_id", "lot_id", "user_id", "user_full_name", "user_email",
			"lot_name", "project_id", "project_name",
		}).AddRow(502, 41, 800, due, models.PaymentStatusPending, nil, nil,
			8, 13, 8, "Luis Paz", "luis@example.com", "Lote 13", 3, "Altos de la Sierra"))

	// A lead time of three days widens the window instead of pinning it to
	// tomorrow.
	payments, err := repo.FindUpcomingReminders(context.Background(), 3)

	require.NoError(t, err)
	require.Len(t, payments, 1)
	payment := payments[0]
	assert.Equal(t, uint(502), payment.ID)
	assert.Equal(t, "Lote 13", payment.Contract.Lot.Name)
	assert.Equal(t, "Luis Paz", payment.Contract.ApplicantUser.FullName)
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
