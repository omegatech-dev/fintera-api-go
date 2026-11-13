-- 000027 marks pending "full" payments on settled contracts as paid and clears
-- paid_amount, but keeps their original amount. The capital repayment payment
-- already carries that money in the ledger, so the schedule total was counting
-- the obligation and the repayment twice.
--
-- Zero the remaining amount of those rows. This is idempotent, so it also
-- repairs environments where 000027 already ran.
UPDATE payments p
SET amount = 0
FROM contracts c
WHERE p.contract_id = c.id
  AND p.payment_type = 'full'
  AND p.amount > 0
  AND p.status = 'paid'
  AND p.paid_amount = 0
  AND (
    c.status = 'closed'
    OR (c.status = 'approved' AND ABS(c.balance) <= 0.005)
  );
