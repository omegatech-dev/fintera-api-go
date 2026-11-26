package backfills

import (
	"context"
	"fmt"

	"github.com/sjperalta/fintera-api/internal/services"
)

// LegacyDocumentMigrator is the piece of the identity document service the
// backfill needs. Keeping it as an interface lets the task be tested without a
// database.
type LegacyDocumentMigrator interface {
	MigrateLegacyDocuments(ctx context.Context) (services.LegacyMigrationResult, error)
}

// Dependencies carries the services used by built-in backfill tasks.
type Dependencies struct {
	HistoricalSale   *services.HistoricalSaleService
	IdentityDocument LegacyDocumentMigrator
}

// DefaultRegistry returns the backfills known to this deployment.
func DefaultRegistry(deps Dependencies) *Registry {
	registry := NewRegistry()
	if deps.HistoricalSale != nil {
		_ = registry.Register(Task{
			Name:        "historical_clients",
			Version:     "1",
			Description: "Link historical sales to customer accounts",
			Run: func(ctx context.Context) (Result, error) {
				outcome, err := deps.HistoricalSale.BackfillClients(ctx, 200)
				if err != nil {
					return Result{}, err
				}
				details := fmt.Sprintf(
					"linked=%d skipped=%d errors=%d",
					outcome.Linked,
					outcome.Skipped,
					outcome.Errors,
				)
				for _, message := range outcome.Messages {
					details += "; " + message
				}
				return Result{
					RowsAffected: int64(outcome.Linked),
					Details:      truncateDetails(details),
				}, nil
			},
		})
	}
	if deps.IdentityDocument != nil {
		_ = registry.Register(Task{
			Name:        "legacy_identity_documents",
			Version:     "1",
			Description: "Expose the files attached to existing contracts as read-only identity document entries",
			Run: func(ctx context.Context) (Result, error) {
				outcome, err := deps.IdentityDocument.MigrateLegacyDocuments(ctx)
				if err != nil {
					return Result{}, err
				}
				if outcome.Contracts == 0 {
					return Result{
						RowsAffected: 0,
						Details:      "nothing to migrate: no contract carries legacy document paths",
					}, nil
				}
				details := fmt.Sprintf(
					"contracts=%d created=%d skipped=%d missing_files=%d errors=%d",
					outcome.Contracts,
					outcome.Created,
					outcome.Skipped,
					outcome.Missing,
					outcome.Errors,
				)
				return Result{
					RowsAffected: outcome.Created,
					Details:      truncateDetails(details),
				}, nil
			},
		})
	}
	return registry
}

func truncateDetails(value string) string {
	const max = 4000
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
