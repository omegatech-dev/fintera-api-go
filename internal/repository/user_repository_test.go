package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
