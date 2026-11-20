package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/sjperalta/fintera-api/internal/models"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	dialector := postgres.New(postgres.Config{
		Conn:       db,
		DriverName: "postgres",
	})

	gdb, err := gorm.Open(dialector, &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	return gdb, mock
}

func TestUserRepository_CountActiveClientsByCreator(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectQuery(`SELECT created_by, count\(\*\) as count FROM "users" WHERE discarded_at IS NULL AND role = \$1 AND status = \$2 AND created_by IN \(\$3,\$4\) GROUP BY "created_by"`).
		WithArgs("user", "active", uint(10), uint(11)).
		WillReturnRows(sqlmock.NewRows([]string{"created_by", "count"}).
			AddRow(10, 3).
			AddRow(11, 0))

	counts, err := repo.CountActiveClientsByCreator(context.Background(), []uint{10, 11})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), counts[10])
	assert.Equal(t, int64(0), counts[11])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_CountActiveClientsByCreator_Empty(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewUserRepository(db)

	counts, err := repo.CountActiveClientsByCreator(context.Background(), nil)

	assert.NoError(t, err)
	assert.Empty(t, counts)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_List_Sorting(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewUserRepository(db)

	tests := []struct {
		name          string
		sortBy        string
		sortDir       string
		expectedOrder string
	}{
		{
			name:          "Default sorting",
			sortBy:        "",
			sortDir:       "",
			expectedOrder: "created_at DESC",
		},
		{
			name:          "Valid field and direction",
			sortBy:        "full_name",
			sortDir:       "asc",
			expectedOrder: "full_name ASC",
		},
		{
			name:          "Valid field and DESC direction",
			sortBy:        "email",
			sortDir:       "desc",
			expectedOrder: "email DESC",
		},
		{
			name:          "Invalid field (SQL Injection attempt)",
			sortBy:        "email; DROP TABLE users; --",
			sortDir:       "asc",
			expectedOrder: "created_at DESC", // Should fallback to default
		},
		{
			name:          "Invalid direction",
			sortBy:        "full_name",
			sortDir:       "invalid",
			expectedOrder: "full_name ASC", // Should fallback to ASC for valid field
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := &ListQuery{
				Page:    1,
				PerPage: 10,
				SortBy:  tt.sortBy,
				SortDir: tt.sortDir,
				Filters: make(map[string]string),
			}

			mock.ExpectQuery(`SELECT count\(\*\) FROM "users".*`).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

			// Check for the ORDER BY clause in the query
			// We use .* before the expectedOrder to match any table prefixes like 'users.created_at'
			orderRegex := strings.ReplaceAll(tt.expectedOrder, "created_at", ".*created_at")
			orderRegex = strings.ReplaceAll(orderRegex, "full_name", ".*full_name")
			orderRegex = strings.ReplaceAll(orderRegex, "email", ".*email")

			mock.ExpectQuery(`SELECT \* FROM "users" .* ORDER BY ` + orderRegex).
				WillReturnRows(sqlmock.NewRows([]string{"id", "full_name"}).AddRow(1, "Test User"))

			_, _, err := repo.List(context.Background(), query)
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserRepository_UpdateCreditScoreWritesOnlyScoreAndTimestamp(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectExec(`UPDATE "users" SET "credit_score"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(720, sqlmock.AnyArg(), uint(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateCreditScore(context.Background(), 9, 720)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_UpdateCreditScoreMissingUser(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectExec(`UPDATE "users" SET "credit_score"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(720, sqlmock.AnyArg(), uint(9)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateCreditScore(context.Background(), 9, 720)

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyUniqueColumns(t *testing.T) {
	assert.Empty(t, emptyUniqueColumns(&models.User{Identity: "0801-1990-12345", RTN: "08011990123456"}))
	assert.Equal(t, []string{"RTN"}, emptyUniqueColumns(&models.User{Identity: "0801-1990-12345"}))
	assert.Equal(t, []string{"Identity"}, emptyUniqueColumns(&models.User{RTN: "08011990123456"}))
	assert.Equal(t, []string{"RTN", "Identity"}, emptyUniqueColumns(&models.User{}))
}

// A full save of a user without RTN/identity must not write the empty string
// into the unique columns; those become NULL after the save instead.
func TestUserRepository_UpdateStoresEmptyUniqueColumnsAsNull(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewUserRepository(db)

	user := &models.User{
		ID:       7,
		Email:    "historico+x@fintera.local",
		FullName: "Cliente Histórico",
		Identity: "",
		RTN:      "",
	}

	// First statement: the full Save with the empty unique columns omitted.
	mock.ExpectExec(`UPDATE "users" SET .+ WHERE "id" = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Second statement: the omitted columns are normalized to NULL (NULL
	// values arrive as placeholder arguments).
	mock.ExpectExec(`UPDATE "users" SET "identity"=\$[0-9]+,"rtn"=\$[0-9]+,"updated_at"=\$[0-9]+ WHERE id = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Update(context.Background(), user)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Clearing an RTN in an edit must end as NULL, not as the empty string that
// would collide with every other client without a tax id.
func TestUserRepository_UpdateClearsRTNAsNull(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewUserRepository(db)

	user := &models.User{
		ID:       8,
		Email:    "cliente@example.com",
		FullName: "Cliente",
		Identity: "0801-1990-12345",
		RTN:      "",
	}

	mock.ExpectExec(`UPDATE "users" SET .+ WHERE "id" = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "users" SET "rtn"=\$[0-9]+,"updated_at"=\$[0-9]+ WHERE id = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Update(context.Background(), user)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_CreateDuplicateRTNMessage(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewUserRepository(db)

	user := &models.User{Email: "a@example.com", FullName: "A", Identity: "1", RTN: "1"}
	dup := &pgconn.PgError{Code: "23505", ConstraintName: "idx_users_rtn_unique"}

	mock.ExpectQuery(`INSERT INTO "users"`).
		WillReturnError(dup)

	err := repo.Create(context.Background(), user)

	assert.EqualError(t, err, "ya existe un usuario con este RTN")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// These tests run against a real database when DATABASE_URL is set; they are
// the regression tests for the historical-client edit flow that used to fail
// with `duplicate key value violates unique constraint "users_rtn_key"`.
func TestUserRepositoryTwoClientsWithoutRTNCanBeEdited(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping live unique-empty-columns test")
	}

	gdb, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	repo := NewUserRepository(gdb)
	ctx := context.Background()

	for index := range 2 {
		user := &models.User{
			Email:    fmt.Sprintf("historico+test%d-%d@fintera.local", time.Now().UnixNano(), index),
			FullName: fmt.Sprintf("Cliente Histórico %d", index),
			Identity: fmt.Sprintf("TEST%d-%d", time.Now().UnixNano(), index),
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("create user %d: %v", index, err)
		}
		user.FullName = fmt.Sprintf("Cliente Histórico %d editado", index)
		if err := repo.Update(ctx, user); err != nil {
			t.Fatalf("update user %d: %v", index, err)
		}
		var stored models.User
		if err := gdb.First(&stored, user.ID).Error; err != nil {
			t.Fatalf("reload user %d: %v", index, err)
		}
		if stored.RTN != "" {
			t.Fatalf("user %d: expected NULL rtn, got %q", index, stored.RTN)
		}
	}
}
