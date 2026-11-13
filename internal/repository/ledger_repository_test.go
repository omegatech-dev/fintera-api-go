package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ledgerEntryColumns() []string {
	return []string{
		"id", "contract_id", "payment_id", "amount", "description",
		"entry_type", "entry_date", "created_at", "updated_at",
	}
}

func TestLedgerRepository_Create(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewLedgerRepository(db)

	mock.ExpectQuery(`INSERT INTO "contract_ledger_entries"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))

	entry := &models.ContractLedgerEntry{
		ContractID:  7,
		Amount:      -10000,
		Description: "Monto Inicial del Contrato",
		EntryType:   models.EntryTypeInitial,
		EntryDate:   time.Now(),
	}

	require.NoError(t, repo.Create(context.Background(), entry))
	assert.Equal(t, uint(42), entry.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLedgerRepository_CreatePropagatesError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewLedgerRepository(db)

	mock.ExpectQuery(`INSERT INTO "contract_ledger_entries"`).
		WillReturnError(errors.New("insert failed"))

	err := repo.Create(context.Background(), &models.ContractLedgerEntry{
		ContractID:  7,
		Amount:      -10000,
		Description: "Monto Inicial del Contrato",
		EntryType:   models.EntryTypeInitial,
	})

	require.ErrorContains(t, err, "insert failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLedgerRepository_FindByContractID(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewLedgerRepository(db)

	entryDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT \* FROM "contract_ledger_entries" WHERE contract_id = \$1 ORDER BY entry_date ASC, created_at ASC`).
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows(ledgerEntryColumns()).
			AddRow(1, 7, nil, -10000.0, "Monto Inicial del Contrato", models.EntryTypeInitial, entryDate, entryDate, entryDate).
			AddRow(2, 7, 9, 1000.0, "Pago Recibido", models.EntryTypePayment, entryDate, entryDate, entryDate))

	entries, err := repo.FindByContractID(context.Background(), 7)

	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, -10000.0, entries[0].Amount)
	assert.Equal(t, models.EntryTypeInitial, entries[0].EntryType)
	assert.Equal(t, models.EntryTypePayment, entries[1].EntryType)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLedgerRepository_FindByPaymentID(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewLedgerRepository(db)

	entryDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	paymentID := uint(9)
	mock.ExpectQuery(`SELECT \* FROM "contract_ledger_entries" WHERE payment_id = \$1 ORDER BY entry_date ASC, created_at ASC`).
		WithArgs(paymentID).
		WillReturnRows(sqlmock.NewRows(ledgerEntryColumns()).
			AddRow(2, 7, paymentID, 1000.0, "Pago Recibido", models.EntryTypePayment, entryDate, entryDate, entryDate))

	entries, err := repo.FindByPaymentID(context.Background(), paymentID)

	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].PaymentID)
	assert.Equal(t, paymentID, *entries[0].PaymentID)
	assert.Equal(t, 1000.0, entries[0].Amount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLedgerRepository_CalculateBalance(t *testing.T) {
	t.Run("sums the ledger for the contract", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) as balance FROM "contract_ledger_entries" WHERE contract_id = \$1`).
			WithArgs(uint(7)).
			WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-9000.0))

		balance, err := repo.CalculateBalance(context.Background(), 7)

		require.NoError(t, err)
		assert.Equal(t, -9000.0, balance)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns zero when the contract has no entries", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) as balance FROM "contract_ledger_entries" WHERE contract_id = \$1`).
			WithArgs(uint(8)).
			WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0.0))

		balance, err := repo.CalculateBalance(context.Background(), 8)

		require.NoError(t, err)
		assert.Zero(t, balance)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("propagates query errors", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) as balance FROM "contract_ledger_entries" WHERE contract_id = \$1`).
			WithArgs(uint(9)).
			WillReturnError(errors.New("db down"))

		_, err := repo.CalculateBalance(context.Background(), 9)

		require.ErrorContains(t, err, "db down")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestLedgerRepository_FindOrCreateByPaymentAndType(t *testing.T) {
	t.Run("updates the existing interest entry", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		entryDate := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		paymentID := uint(11)
		originalDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		mock.ExpectQuery(`SELECT \* FROM "contract_ledger_entries" WHERE payment_id = \$1 AND entry_type = \$2 ORDER BY "contract_ledger_entries"\."id" LIMIT \$3`).
			WithArgs(paymentID, models.EntryTypeInterest, 1).
			WillReturnRows(sqlmock.NewRows(ledgerEntryColumns()).
				AddRow(5, 7, paymentID, -10.0, "Interés anterior", models.EntryTypeInterest, originalDate, originalDate, originalDate))
		mock.ExpectExec(`UPDATE "contract_ledger_entries" SET .+ WHERE "id" = \$\d+`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		entry := &models.ContractLedgerEntry{
			ContractID:  7,
			PaymentID:   &paymentID,
			Amount:      -25.0,
			Description: "Interés actualizado",
			EntryType:   models.EntryTypeInterest,
			EntryDate:   entryDate,
		}

		require.NoError(t, repo.FindOrCreateByPaymentAndType(context.Background(), entry))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("creates a new interest entry when none exists", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		paymentID := uint(12)
		mock.ExpectQuery(`SELECT \* FROM "contract_ledger_entries" WHERE payment_id = \$1 AND entry_type = \$2 ORDER BY "contract_ledger_entries"\."id" LIMIT \$3`).
			WithArgs(paymentID, models.EntryTypeInterest, 1).
			WillReturnRows(sqlmock.NewRows(ledgerEntryColumns()))
		mock.ExpectQuery(`INSERT INTO "contract_ledger_entries"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(6))

		entry := &models.ContractLedgerEntry{
			ContractID:  7,
			PaymentID:   &paymentID,
			Amount:      -30.0,
			Description: "Interés nuevo",
			EntryType:   models.EntryTypeInterest,
			EntryDate:   time.Now(),
		}

		require.NoError(t, repo.FindOrCreateByPaymentAndType(context.Background(), entry))
		assert.Equal(t, uint(6), entry.ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("creates directly for non-interest entries", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		paymentID := uint(13)
		mock.ExpectQuery(`INSERT INTO "contract_ledger_entries"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))

		entry := &models.ContractLedgerEntry{
			ContractID:  7,
			PaymentID:   &paymentID,
			Amount:      500.0,
			Description: "Pago",
			EntryType:   models.EntryTypePayment,
			EntryDate:   time.Now(),
		}

		require.NoError(t, repo.FindOrCreateByPaymentAndType(context.Background(), entry))
		assert.Equal(t, uint(7), entry.ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("propagates lookup errors other than not found", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		paymentID := uint(14)
		mock.ExpectQuery(`SELECT \* FROM "contract_ledger_entries" WHERE payment_id = \$1 AND entry_type = \$2`).
			WithArgs(paymentID, models.EntryTypeInterest, 1).
			WillReturnError(errors.New("db down"))

		entry := &models.ContractLedgerEntry{
			ContractID:  7,
			PaymentID:   &paymentID,
			EntryType:   models.EntryTypeInterest,
			EntryDate:   time.Now(),
			Description: "Interés",
		}

		err := repo.FindOrCreateByPaymentAndType(context.Background(), entry)

		require.ErrorContains(t, err, "db down")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestLedgerRepository_BatchUpsertInterest(t *testing.T) {
	t.Run("returns early for an empty batch", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		require.NoError(t, repo.BatchUpsertInterest(context.Background(), nil))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("creates new entries and updates existing ones", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		entryDate := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		existingDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		existingPaymentID := uint(11)
		newPaymentID := uint(12)

		mock.ExpectQuery(`SELECT \* FROM "contract_ledger_entries" WHERE payment_id IN \(\$1,\$2\) AND entry_type = \$3`).
			WithArgs(existingPaymentID, newPaymentID, models.EntryTypeInterest).
			WillReturnRows(sqlmock.NewRows(ledgerEntryColumns()).
				AddRow(5, 7, existingPaymentID, -10.0, "Interés anterior", models.EntryTypeInterest, existingDate, existingDate, existingDate))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "contract_ledger_entries"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(6))
		mock.ExpectExec(`UPDATE "contract_ledger_entries" SET "amount"=\$1,"description"=\$2,"entry_date"=\$3,"updated_at"=\$4 WHERE id = \$5`).
			WithArgs(-15.0, "Interés actualizado", entryDate, sqlmock.AnyArg(), uint(5)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		entries := []models.ContractLedgerEntry{
			{
				ContractID:  7,
				PaymentID:   &existingPaymentID,
				Amount:      -15.0,
				Description: "Interés actualizado",
				EntryType:   models.EntryTypeInterest,
				EntryDate:   entryDate,
			},
			{
				ContractID:  7,
				PaymentID:   &newPaymentID,
				Amount:      -20.0,
				Description: "Interés nuevo",
				EntryType:   models.EntryTypeInterest,
				EntryDate:   entryDate,
			},
		}

		require.NoError(t, repo.BatchUpsertInterest(context.Background(), entries))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("does not panic when an entry has no payment ID", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "contract_ledger_entries"`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(8))
		mock.ExpectCommit()

		entries := []models.ContractLedgerEntry{{
			ContractID:  7,
			PaymentID:   nil,
			Amount:      -5.0,
			Description: "Interés sin pago",
			EntryType:   models.EntryTypeInterest,
			EntryDate:   time.Now(),
		}}

		require.NoError(t, repo.BatchUpsertInterest(context.Background(), entries))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back when the insert fails", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewLedgerRepository(db)

		paymentID := uint(15)
		mock.ExpectQuery(`SELECT \* FROM "contract_ledger_entries" WHERE payment_id IN`).
			WithArgs(paymentID, models.EntryTypeInterest).
			WillReturnRows(sqlmock.NewRows(ledgerEntryColumns()))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "contract_ledger_entries"`).
			WillReturnError(errors.New("insert failed"))
		mock.ExpectRollback()

		entryDate := time.Now()
		entries := []models.ContractLedgerEntry{{
			ContractID:  7,
			PaymentID:   &paymentID,
			Amount:      -5.0,
			Description: "Interés",
			EntryType:   models.EntryTypeInterest,
			EntryDate:   entryDate,
		}}

		err := repo.BatchUpsertInterest(context.Background(), entries)

		require.ErrorContains(t, err, "insert failed")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestLedgerRepository_DeleteByContractID(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewLedgerRepository(db)

	mock.ExpectExec(`DELETE FROM "contract_ledger_entries" WHERE contract_id = \$1`).
		WithArgs(uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 3))

	require.NoError(t, repo.DeleteByContractID(context.Background(), 7))
	require.NoError(t, mock.ExpectationsWereMet())
}
