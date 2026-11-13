package models

import "testing"

func TestHistoricalSaleTableAndDefaults(t *testing.T) {
	sale := HistoricalSale{}
	if sale.TableName() != "historical_sales" {
		t.Fatalf("table = %q", sale.TableName())
	}
	if HistoricalSaleStatusPending == HistoricalSaleStatusValidated ||
		HistoricalSaleStatusValidated == HistoricalSaleStatusRejected {
		t.Fatal("historical sale statuses must be distinct")
	}
}

func TestReferralVisitTable(t *testing.T) {
	visit := ReferralVisit{}
	if visit.TableName() != "referral_visits" {
		t.Fatalf("table = %q", visit.TableName())
	}
}

func TestAssignmentTable(t *testing.T) {
	assignment := Assignment{}
	if assignment.TableName() != "assignments" {
		t.Fatalf("table = %q", assignment.TableName())
	}
}
