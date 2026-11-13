package main

import (
	"log"
	"regexp"

	"github.com/joho/godotenv"
	"github.com/rollbar/rollbar-go"
	"github.com/sjperalta/fintera-api/internal/config"
)

// test_rollbar sends a single test message to Rollbar using the same
// configuration and scrubbing rules as cmd/api. Run it with:
//
//	go run ./cmd/test_rollbar
func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.RollbarToken == "" {
		log.Fatal("ROLLBAR_TOKEN is not set")
	}

	rollbar.SetToken(cfg.RollbarToken)
	rollbar.SetEnvironment(cfg.RollbarEnvironment)
	rollbar.SetCodeVersion(cfg.RollbarCodeVersion)
	rollbar.SetServerRoot(cfg.RollbarServerRoot)
	rollbar.SetScrubFields(regexp.MustCompile(`(?i)password|passwd|secret|token|authorization|cookie|api[_-]?key|session|jwt`))

	log.Printf("Sending test message to Rollbar (environment=%s, version=%s)", cfg.RollbarEnvironment, cfg.RollbarCodeVersion)
	rollbar.Message(rollbar.INFO, "Fintera API Rollbar test message")

	// Wait blocks until queued items are sent, so the process does not exit
	// before the message reaches Rollbar.
	rollbar.Wait()
	log.Println("Test message flushed to Rollbar")
}
