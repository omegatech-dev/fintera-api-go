package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSellerReferralsMigrationsArePaired(t *testing.T) {
	migrations := []string{
		"000032_seller_referrals_and_scores",
		"000033_contract_assignments",
		"000034_referral_visits",
		"000035_historical_sales",
		"000036_commission_origin_and_rates",
		"000037_commission_acceptance_window",
		"000038_seller_referral_guid",
	}
	for _, name := range migrations {
		for _, direction := range []string{"up", "down"} {
			path := filepath.Join("migrations", name+"."+direction+".sql")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing migration %s: %v", path, err)
			}
			if strings.TrimSpace(string(contents)) == "" {
				t.Fatalf("migration %s is empty", path)
			}
		}
	}
}

func TestSellerMigrationsContainRequiredColumns(t *testing.T) {
	up, err := os.ReadFile(filepath.Join("migrations", "000032_seller_referrals_and_scores.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{
		"referral_slug",
		"commission_rate_override",
		"score",
		"score_level",
		"score_override",
		"score_override_reason",
		"score_override_by",
		"score_override_at",
	} {
		if !strings.Contains(string(up), column) {
			t.Fatalf("000032 is missing %s", column)
		}
	}

	assignments, err := os.ReadFile(filepath.Join("migrations", "000033_contract_assignments.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"CREATE TABLE IF NOT EXISTS assignments", "seller_id", "assignment_status", "accept_deadline_at"} {
		if !strings.Contains(string(assignments), fragment) {
			t.Fatalf("000033 is missing %s", fragment)
		}
	}
}

func TestSellerReferralGUIDMigration(t *testing.T) {
	up, err := os.ReadFile(filepath.Join("migrations", "000038_seller_referral_guid.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	upSQL := string(up)
	for _, fragment := range []string{
		"ADD COLUMN IF NOT EXISTS referral_guid UUID",
		"ADD COLUMN IF NOT EXISTS referral_guid TEXT",
		"gen_random_uuid()",
		"SET referral_guid = referral_slug",
		"DROP COLUMN IF EXISTS referral_slug",
	} {
		if !strings.Contains(upSQL, fragment) {
			t.Fatalf("000038 up is missing %s", fragment)
		}
	}

	down, err := os.ReadFile(filepath.Join("migrations", "000038_seller_referral_guid.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	downSQL := string(down)
	for _, fragment := range []string{
		"ADD COLUMN IF NOT EXISTS referral_slug",
		"SET referral_slug = referral_guid",
		"DROP COLUMN IF EXISTS referral_guid",
	} {
		if !strings.Contains(downSQL, fragment) {
			t.Fatalf("000038 down is missing %s", fragment)
		}
	}
}
