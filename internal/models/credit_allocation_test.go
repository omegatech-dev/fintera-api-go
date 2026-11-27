package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPlanCreditAllocations(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		commission float64
		credits    []SellerCommissionCredit
		want       []CreditAllocation
	}{
		{
			name:       "credit covers commission and leaves balance",
			commission: 8000,
			credits: []SellerCommissionCredit{
				{ID: 12, Amount: 10000, CreatedAt: created},
			},
			want: []CreditAllocation{{CreditID: 12, Amount: 8000}},
		},
		{
			name:       "commission exceeds credit",
			commission: 8000,
			credits: []SellerCommissionCredit{
				{ID: 12, Amount: 3000, CreatedAt: created},
			},
			want: []CreditAllocation{{CreditID: 12, Amount: 3000}},
		},
		{
			name:       "multiple credits are consumed in caller FIFO order",
			commission: 8000,
			credits: []SellerCommissionCredit{
				{ID: 2, Amount: 3000, CreatedAt: created},
				{ID: 3, Amount: 10000, CreatedAt: created.Add(time.Hour)},
			},
			want: []CreditAllocation{
				{CreditID: 2, Amount: 3000},
				{CreditID: 3, Amount: 5000},
			},
		},
		{
			name:       "already applied amount is excluded",
			commission: 6000,
			credits: []SellerCommissionCredit{
				{ID: 4, Amount: 10000, AppliedAmount: 7500, CreatedAt: created},
			},
			want: []CreditAllocation{{CreditID: 4, Amount: 2500}},
		},
		{
			name:       "last allocation absorbs cent rounding",
			commission: 1.01,
			credits: []SellerCommissionCredit{
				{ID: 5, Amount: 0.33, CreatedAt: created},
				{ID: 6, Amount: 1.00, CreatedAt: created.Add(time.Hour)},
			},
			want: []CreditAllocation{
				{CreditID: 5, Amount: 0.33},
				{CreditID: 6, Amount: 0.68},
			},
		},
		{
			name:       "no credit for zero commission",
			commission: 0,
			credits: []SellerCommissionCredit{
				{ID: 7, Amount: 100, CreatedAt: created},
			},
			want: []CreditAllocation{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PlanCreditAllocations(tt.commission, tt.credits))
		})
	}
}
