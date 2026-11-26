package backfills

import (
	"context"
	"errors"
	"testing"

	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLegacyMigrator records the call and returns a canned outcome.
type fakeLegacyMigrator struct {
	result services.LegacyMigrationResult
	err    error
	calls  int
}

func (f *fakeLegacyMigrator) MigrateLegacyDocuments(context.Context) (services.LegacyMigrationResult, error) {
	f.calls++
	return f.result, f.err
}

func TestLegacyIdentityDocumentsTaskReportsThatThereIsNothingToMigrate(t *testing.T) {
	migrator := &fakeLegacyMigrator{result: services.LegacyMigrationResult{Contracts: 0}}
	registry := DefaultRegistry(Dependencies{IdentityDocument: migrator})

	task, ok := registry.Find("legacy_identity_documents")
	require.True(t, ok)
	assert.Equal(t, "1", task.Version)

	result, err := task.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, migrator.calls)
	assert.Equal(t, int64(0), result.RowsAffected)
	assert.Contains(t, result.Details, "nothing to migrate")
}

func TestLegacyIdentityDocumentsTaskReportsTheCounts(t *testing.T) {
	migrator := &fakeLegacyMigrator{result: services.LegacyMigrationResult{
		Contracts: 3, Created: 5, Skipped: 1, Missing: 2, Errors: 0,
	}}
	registry := DefaultRegistry(Dependencies{IdentityDocument: migrator})

	task, ok := registry.Find("legacy_identity_documents")
	require.True(t, ok)

	result, err := task.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(5), result.RowsAffected)
	assert.Contains(t, result.Details, "contracts=3")
	assert.Contains(t, result.Details, "created=5")
	assert.Contains(t, result.Details, "skipped=1")
	assert.Contains(t, result.Details, "missing_files=2")
}

func TestLegacyIdentityDocumentsTaskPropagatesTheError(t *testing.T) {
	migrator := &fakeLegacyMigrator{err: errors.New("database is down")}
	registry := DefaultRegistry(Dependencies{IdentityDocument: migrator})

	task, ok := registry.Find("legacy_identity_documents")
	require.True(t, ok)

	_, err := task.Run(context.Background())
	assert.Error(t, err)
}

func TestRegistrySkipsTheIdentityTaskWhenTheServiceIsMissing(t *testing.T) {
	registry := DefaultRegistry(Dependencies{})
	_, ok := registry.Find("legacy_identity_documents")
	assert.False(t, ok)
}
