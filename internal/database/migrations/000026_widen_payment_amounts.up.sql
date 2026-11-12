-- Every other money column uses NUMERIC(15,2): lots.price, contracts.amount,
-- contracts.balance, payments.paid_amount and the contract ledger entries.
-- payments.amount and payments.interest_amount were still NUMERIC(10,2), so a
-- cash/full payment or a capital repayment above ~100M HNL overflowed with
-- SQLSTATE 22003 while inserting the payment record.
ALTER TABLE payments ALTER COLUMN amount TYPE NUMERIC(15,2);
ALTER TABLE payments ALTER COLUMN interest_amount TYPE NUMERIC(15,2);
