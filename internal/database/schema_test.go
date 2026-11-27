package database

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeSchemaMigrator is a schemaChecker stub so VerifySchema can be tested
// without a database connection.
type fakeSchemaMigrator struct {
	tables         map[string]bool
	missingColumns map[string]bool
}

func (f *fakeSchemaMigrator) HasTable(name interface{}) bool {
	return f.tables[nameString(name)]
}

func (f *fakeSchemaMigrator) HasColumn(table interface{}, column string) bool {
	if f.missingColumns[nameString(table)+"."+column] {
		return false
	}
	return f.tables[nameString(table)]
}

func nameString(value interface{}) string {
	if name, ok := value.(string); ok {
		return name
	}
	return ""
}

func TestVerifySchemaPassesWithAllRequiredObjects(t *testing.T) {
	tables := map[string]bool{}
	for _, table := range requiredSchema.tables {
		tables[table] = true
	}
	migrator := &fakeSchemaMigrator{tables: tables}

	err := verifyWithMigrator(migrator)
	require.NoError(t, err)
}

func TestVerifySchemaListsMissingTablesAndColumns(t *testing.T) {
	migrator := &fakeSchemaMigrator{
		tables: map[string]bool{
			"users":     true,
			"projects":  true,
			"lots":      true,
			"contracts": true,
		},
		missingColumns: map[string]bool{
			"contracts.commission_paid_at": true,
		},
	}
	err := verifyWithMigrator(migrator)
	require.Error(t, err)
	message := err.Error()
	require.Contains(t, message, "table credit_applications")
	require.Contains(t, message, "table seller_commission_credits")
	for _, required := range []string{"table payments", "table contract_ledger_entries", "table assignments", "table audit_logs", "table notifications"} {
		require.Contains(t, message, required)
	}
	// contracts exists but commission columns must be reported as missing.
	require.Contains(t, message, "column contracts.commission_paid_at")
	// The fix hint must point at the migrations command.
	require.Contains(t, strings.ToLower(message), "make migrate-up")
}
