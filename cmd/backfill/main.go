package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"

	"github.com/sjperalta/fintera-api/internal/backfills"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/database"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
)

// Generic backfill runner. The API runs pending tasks at startup; this binary
// is for local/staging inspection and manual re-runs inside the container.
func main() {
	list := flag.Bool("list", false, "list registered backfills and their latest status")
	taskName := flag.String("task", "", "run one backfill by name")
	force := flag.Bool("force", false, "run a completed task again (requires -task)")
	flag.Parse()

	if *force && *taskName == "" {
		fatal("-force requires -task=<name>")
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("failed to load configuration: %v", err)
	}
	db, err := database.Connect(cfg.DatabaseURL, cfg.Environment)
	if err != nil {
		fatal("failed to connect to the database: %v", err)
	}

	repos := repository.NewRepositories(db)
	store, err := storage.NewLocalStorage(cfg.StoragePath)
	if err != nil {
		fatal("failed to initialize storage: %v", err)
	}
	historicalClientSvc := services.NewHistoricalClientService(repos.User, repos.HistoricalSale)
	historicalSaleSvc := services.NewHistoricalSaleService(repos.HistoricalSale, nil, nil, historicalClientSvc)
	identityDocumentSvc := services.NewIdentityDocumentService(
		repos.IdentityDocument, repos.Contract, store, nil, cfg.IdentityDocumentsRequired,
	)
	registry := backfills.DefaultRegistry(backfills.Dependencies{
		HistoricalSale:   historicalSaleSvc,
		IdentityDocument: identityDocumentSvc,
	})
	runner := backfills.NewRunner(repos.Backfill)
	ctx := context.Background()

	if *list {
		statuses, err := runner.Statuses(ctx, registry)
		if err != nil {
			fatal("failed to list backfills: %v", err)
		}
		fmt.Printf("%-24s %-8s %-10s %s\n", "NAME", "VERSION", "STATUS", "DESCRIPTION")
		for _, status := range statuses {
			fmt.Printf("%-24s %-8s %-10s %s\n", status.Task.Name, status.Task.Version, status.Status, status.Task.Description)
			if status.LastRun != nil {
				fmt.Printf("  last run: %s rows=%d details=%s\n", status.LastRun.Status, status.LastRun.RowsAffected, status.LastRun.Details)
			}
		}
		return
	}

	if *taskName != "" {
		task, ok := registry.Find(*taskName)
		if !ok {
			fmt.Fprintln(os.Stderr, "unknown backfill; available tasks:")
			for _, available := range registry.Tasks() {
				fmt.Fprintf(os.Stderr, "  %s\n", available.Name)
			}
			os.Exit(1)
		}
		result := runner.Run(ctx, task, *force)
		printResult(result)
		if result.Err != nil {
			os.Exit(1)
		}
		return
	}

	failed := false
	for _, result := range runner.RunPending(ctx, registry) {
		printResult(result)
		if result.Err != nil {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func printResult(result backfills.TaskRun) {
	fmt.Println(result.String())
	if result.Run != nil && result.Run.Details != "" {
		fmt.Printf("  %s\n", result.Run.Details)
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
