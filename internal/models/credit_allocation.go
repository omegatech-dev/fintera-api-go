package models

import "math"

// CreditAllocation is one credit amount consumed by a commission payout.
type CreditAllocation struct {
	CreditID uint
	Amount   float64
}

// PlanCreditAllocations distributes a commission across the given credits in
// order, using each credit's remaining balance. Callers must pass credits in
// the desired consumption order (FIFO). The plan never exceeds the commission
// or a credit's remaining balance, and every amount is rounded to cents.
func PlanCreditAllocations(commission float64, credits []SellerCommissionCredit) []CreditAllocation {
	allocations := make([]CreditAllocation, 0, len(credits))
	remaining := RoundMoney(commission)
	if remaining <= MoneyEpsilon {
		return allocations
	}
	for i := range credits {
		if remaining <= MoneyEpsilon {
			break
		}
		available := credits[i].RemainingAmount()
		if available <= MoneyEpsilon {
			continue
		}
		amount := RoundMoney(math.Min(available, remaining))
		if amount <= MoneyEpsilon {
			continue
		}
		allocations = append(allocations, CreditAllocation{CreditID: credits[i].ID, Amount: amount})
		remaining = RoundMoney(remaining - amount)
	}
	return allocations
}
