package database

import (
	"fmt"

	"gorm.io/gorm"
)

// requiredSchema describes the tables and key columns the API queries on its
// money paths. It is verified at startup so a deployment whose migrations were
// not applied fails with a clear message instead of a runtime SQL error in the
// middle of a cancellation or payout.
var requiredSchema = struct {
	tables  []string
	columns map[string][]string
}{
	tables: []string{
		"users",
		"projects",
		"lots",
		"contracts",
		"payments",
		"contract_ledger_entries",
		"assignments",
		"seller_commission_credits",
		"credit_applications",
		"notifications",
		"audit_logs",
	},
	columns: map[string][]string{
		"contracts":                 {"commission_amount", "commission_paid_at", "seller_id", "guid"},
		"seller_commission_credits": {"amount", "applied_amount", "source"},
		"seller_commission_rates":   {"seller_id", "rate"},
		"credit_applications":       {"credit_id", "contract_id", "amount", "reversed_at"},
	},
}

// VerifySchema reports every required table or column missing from the
// database. A missing item means the deployment did not run its migrations:
// `make migrate-up` (or a container restart) applies them.
func VerifySchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("schema verification requires a database connection")
	}
	return verifyWithMigrator(db.Migrator())
}

// schemaChecker is the narrow slice of the migration API the verification
// needs, so tests can stub it without a live database. Signatures match
// gorm.Migrator.
type schemaChecker interface {
	HasTable(name interface{}) bool
	HasColumn(table interface{}, column string) bool
}

func verifyWithMigrator(migrator schemaChecker) error {
	missing := make([]string, 0)
	for _, table := range requiredSchema.tables {
		if !migrator.HasTable(table) {
			missing = append(missing, "table "+table)
			// Columns of a missing table are missing by definition.
			continue
		}
		for _, column := range requiredSchema.columns[table] {
			if !migrator.HasColumn(table, column) {
				missing = append(missing, fmt.Sprintf("column %s.%s", table, column))
			}
		}
	}

	if len(missing) == 0 {
		return nil
	}
	details := missing[0]
	for _, item := range missing[1:] {
		details += ", " + item
	}
	return fmt.Errorf("database schema is out of date (missing %s); run `make migrate-up` with this environment's DATABASE_URL, or restart the Docker container so pending migrations run", details)
}
