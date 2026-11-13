package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/statemachine"
)

// capitalRepaymentEpsilon absorbs rounding noise when comparing money amounts.
const capitalRepaymentEpsilon = 0.005

// applyCapitalToSchedule distributes a capital repayment over the pending
// obligations of a contract, from the end of the schedule backwards.
//
// Installments are removed (or reduced) to shorten the term. Terminal
// obligations such as the full balance, the down payment or the reservation
// are marked as paid when the repayment covers them, so the customer portal
// stops offering a receipt for an obligation that is already settled.
func applyCapitalToSchedule(ctx context.Context, repo repository.PaymentRepository, contractID, excludePaymentID uint, amount float64) (int, error) {
	if amount <= 0 {
		return 0, nil
	}

	payments, err := repo.FindByContract(ctx, contractID)
	if err != nil {
		return 0, fmt.Errorf("failed to load pending payments: %w", err)
	}

	var primary, secondary []*models.Payment
	for i := range payments {
		payment := &payments[i]
		if payment.ID == excludePaymentID ||
			payment.PaymentType == models.PaymentTypeCapitalRepayment {
			continue
		}
		if payment.Status != models.PaymentStatusPending && payment.Status != models.PaymentStatusSubmitted {
			continue
		}

		switch payment.PaymentType {
		case models.PaymentTypeReservation, models.PaymentTypeDownPayment:
			secondary = append(secondary, payment)
		default:
			primary = append(primary, payment)
		}
	}

	adjusted := 0
	for _, group := range [][]*models.Payment{primary, secondary} {
		for i := len(group) - 1; i >= 0 && amount > capitalRepaymentEpsilon; i-- {
			target := group[i]
			targetAmount := target.Amount
			if targetAmount <= 0 {
				continue
			}

			if amount+capitalRepaymentEpsilon >= targetAmount {
				if target.PaymentType == models.PaymentTypeInstallment || target.PaymentType == "" {
					if err := repo.Delete(ctx, target.ID); err != nil {
						return adjusted, fmt.Errorf("failed to delete payment #%d: %w", target.ID, err)
					}
				} else {
					fsm := statemachine.NewPaymentFSM(target)
					if err := fsm.Approve(ctx); err != nil {
						return adjusted, fmt.Errorf("failed to settle payment #%d: %w", target.ID, err)
					}

					now := time.Now()
					zero := 0.0
					target.PaidAmount = &zero
					target.PaymentDate = &now
					target.ApprovedAt = &now
					appendPaymentNote(target, fmt.Sprintf("Cubierto por abono a capital de L%.2f", targetAmount))
					if err := repo.Update(ctx, target); err != nil {
						return adjusted, fmt.Errorf("failed to update payment #%d: %w", target.ID, err)
					}
				}

				amount -= targetAmount
				adjusted++
				continue
			}

			target.Amount -= amount
			appendPaymentNote(target, fmt.Sprintf("Ajustado por abono a capital de L%.2f", amount))
			if err := repo.Update(ctx, target); err != nil {
				return adjusted, fmt.Errorf("failed to update payment #%d: %w", target.ID, err)
			}
			amount = 0
			adjusted++
		}
	}

	return adjusted, nil
}

func appendPaymentNote(payment *models.Payment, note string) {
	if payment.Description != nil && *payment.Description != "" {
		note = fmt.Sprintf("%s (%s)", *payment.Description, note)
	}
	payment.Description = &note
}
