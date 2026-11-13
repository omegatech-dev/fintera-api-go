-- Cash/bank contracts keep their remaining balance in a single "full" payment.
-- Capital repayments used to zero the ledger and close the contract without
-- touching that row, leaving it pending for the customer portal.
--
-- Repair only rows without a receipt on contracts that are already settled:
--   * closed contracts, or
--   * approved contracts whose cached balance is exactly zero.
--
-- The row is marked as paid and its remaining amount is zeroed. paid_amount is
-- also 0 so the real money is not counted twice: the capital repayment payment
-- already carries the credit in the ledger.
UPDATE payments p
SET status = 'paid',
    amount = 0,
    paid_amount = 0,
    payment_date = CURRENT_DATE,
    approved_at = NOW()
FROM contracts c
WHERE p.contract_id = c.id
  AND p.payment_type = 'full'
  AND p.status = 'pending'
  AND (
    c.status = 'closed'
    OR (c.status = 'approved' AND ABS(c.balance) <= 0.005)
  );
