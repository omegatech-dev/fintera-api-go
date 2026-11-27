## ADDED Requirements

### Requirement: Netted commissions expose their credit adjustment

The commissions report SHALL expose, on each commission row, the amount of the
commission that was settled with seller credits (`credit_applied`: the sum of
active credit applications for that contract). The CSV/PDF export SHALL state
the adjustment on the row's state column: a paid commission with credits
applied SHALL read "Pagada el <fecha> · Reajuste por crédito: L <aplicado>
(efectivo L <efectivo>)". Rows without credit applications SHALL render exactly
as before.

#### Scenario: Partially netted commission

- **WHEN** a commission of L 16,800 was paid with L 13,800 of seller credit and L 3,000 in cash
- **THEN** the report row shows `credit_applied = 13,800` and the export state reads "Reajuste por crédito: L 13,800 (efectivo L 3,000)"

#### Scenario: Fully netted commission

- **WHEN** a commission was settled entirely with seller credit
- **THEN** the row shows `credit_applied` equal to the commission and the export states an effective cash of L 0

#### Scenario: Commission without credit applications

- **WHEN** a commission was paid entirely in cash
- **THEN** `credit_applied` is 0 and the row state renders exactly as before, without any adjustment note

#### Scenario: Reversed applications do not count

- **WHEN** a credit application was reversed
- **THEN** it is excluded from the row's `credit_applied`
