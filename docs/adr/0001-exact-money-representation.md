# ADR-0001: Exact decimal representation

- Status: Accepted
- Date: 2026-09-18
- Updated: 2026-09-29; Rate JSON uses six-decimal strings.
- Decision owners: Intania 888 backend maintainers
- Related backlog: `DEC-001`, `DR-001`, `DR-004`, `DR-005`, `DR-006`, `DR-009`, `DR-017`

## Context

In the legacy `main` backend, PostgreSQL stores balances, bets, rewards, and payouts as `DECIMAL(10,2)`, but Go maps them to `float64`. That API emits JSON numbers and its frontend consumes JavaScript `number` values. This makes database storage exact while calculations, comparisons, and transport through Go and JavaScript use binary floating point.

The next development cycle will also move persistence from GORM toward sqlc. Money therefore needs one exact domain representation with an unambiguous database and API contract before business logic and queries are rewritten.

## Decision

Money is represented as an integer count of hundredths of a coin throughout persistence and Go business logic.

| Boundary | Representation | Example for 888.88 |
| --- | --- | --- |
| PostgreSQL | `BIGINT` minor units | `88888` |
| Go domain | `Money` backed by private `int64` | `Money{minor: 88888}` |
| JSON API | fixed two-decimal string | `"888.88"` |
| Frontend API type | branded decimal string | `MoneyString("888.88")` |
| Frontend calculations | integer minor units using `bigint` when needed | `88888n` |

The public JSON property names remain unchanged. Their values change from JSON numbers to decimal strings in one breaking backend release. The current frontend is intentionally incompatible until its later migration.

### Money and non-money values

The following are money and use `Money`:

- user balances
- bill totals
- daily rewards
- slot spend and reward amounts
- steal amounts and resulting balances
- Stake Mines bet amounts, current/final payouts, and aggregate wager/winning/profit amounts
- external coin deduction amounts and resulting balances

Odds and payout multipliers are exact decimals, but are not money. They use
`Rate`, a nonnegative value backed by `int64` millionth units, with six fractional
digits. Bill-line rates, match team rates, and Stake Mines game/history
multipliers are encoded as canonical six-decimal JSON strings. Business
calculations remain integer-based.

Approximate statistics, including Stake Mines `win_rate`, remain JSON numbers.
`win_rate` is an approximate percentage. Scores, counts, and indexes are also
JSON numbers; they are not converted to `Money` or `Rate`.

`SignedMoney` is a checked signed minor-unit type for derived deltas, including Stake Mines net profit. It uses the same canonical two-decimal JSON string representation.

## Go contract

The domain type owns the invariant; callers cannot construct it by assigning an exposed integer field.

```go
type Money struct {
	minor int64
}

func NewMoneyFromMinor(minor int64) (Money, error)
func ParseMoney(decimal string) (Money, error)

func (m Money) MinorUnits() int64
func (m Money) String() string
func (m Money) Add(other Money) (Money, error)
func (m Money) Sub(other Money) (Money, error)
func (m Money) Compare(other Money) int
func (m Money) IsZero() bool
```

Rules:

- `Money` represents non-negative amounts from zero through `math.MaxInt64` minor units.
- The maximum representable amount is `92,233,720,368,547,758.07`.
- Constructors reject negative values, malformed decimals, and values outside that range.
- `Add` checks for overflow before adding.
- `Sub` returns an error instead of producing a negative amount.
- No ordinary constructor accepts `float32` or `float64`.
- A deliberately named legacy conversion, if temporarily required at a migration boundary, must not be available to business logic and must be deleted with the old schema.
- Zero is a valid value. Absence is represented separately with a pointer or explicit optional type; zero must not mean null.

Repository code maps sqlc-generated `int64` columns to and from `Money`. The domain type does not depend on GORM, pgx, sqlc-generated packages, or database interfaces.

### Multiplication and rounding

Money is multiplied only through an explicit operation with a separate `Rate` value:

```go
func (m Money) Mul(rate Rate) (Money, error)
```

The initial compatibility rounding mode is round-half-up to the nearest minor unit, matching the intent of the current positive-value rounding code. Intermediate multiplication must detect overflow; it must not multiply two `int64` values first and check afterward. An implementation may use quotient/remainder decomposition or a wider integer calculation.

Every business operation must state where rounding occurs. Repeated intermediate rounding is not interchangeable with rounding once at the final payout.

## PostgreSQL contract

Money columns use `BIGINT`, where one unit equals `0.01` coin. This maps directly to Go `int64` and sqlc without a decimal adapter.

Each non-negative money column has a database constraint equivalent to:

```sql
amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0)
```

Atomic balance updates include the invariant in the statement, for example:

```sql
UPDATE users
SET remaining_coin_minor = remaining_coin_minor - $1
WHERE id = $2
  AND remaining_coin_minor >= $1;
```

The affected-row count determines whether the deduction succeeded. Application-only balance checks are insufficient under concurrency.

`NUMERIC(19,2)` is explicitly rejected for money storage because its full scaled range exceeds `int64`. `NUMERIC(18,2)` would fit but adds conversion complexity without a benefit for the selected representation.

## JSON contract

Money is serialized as a JSON string with exactly two decimal digits:

```json
{
  "remaining_coin": "888.88",
  "total": "100.50",
  "reward": "0.00"
}
```

Canonical output:

- always uses ASCII digits
- always contains one decimal point
- always contains exactly two fractional digits
- contains no grouping separators, currency symbol, exponent, leading plus sign, or surrounding whitespace
- emits zero as `"0.00"`

Input may accept `"12"`, `"12.3"`, or `"12.30"` during parsing and normalize them to `"12.00"`, `"12.30"`, and `"12.30"`. It rejects negative values, empty strings, exponents, commas, more than two fractional digits, and values above the `int64` range.

The parser operates on decimal characters directly. It must never parse through `float64`.

JSON numeric tokens are rejected for every Money input. Only JSON strings are accepted.

### Rate JSON contract

`Rate` uses a quoted nonnegative decimal with exactly six fractional digits:

```json
{
  "rate": "1.750000",
  "team_a_rate": "1.750000",
  "team_b_rate": "2.500000",
  "multiplier": "1.030000"
}
```

Quoted inputs with zero through six fractional digits are accepted and padded
without rounding; existing leading-zero parsing behavior is preserved. Numeric
tokens, null, negatives, exponents, whitespace inside values, excess precision,
and values outside nonnegative `int64` micro-units are rejected. The maximum is
`"9223372036854.775807"`; zero is `"0.000000"`.

Money remains a two-decimal string, and derived signed deltas use `SignedMoney`.
Rate storage, six-digit precision, arithmetic, formulas, and payout rounding are
unchanged. This Rate serialization change needs no database migration and has
no temporary numeric/string compatibility mode under `/api/v1`.

## Frontend contract

API money is not typed as a general TypeScript `number`:

```ts
type MoneyString = string & { readonly __money: unique symbol };
type RateString = string & { readonly __rate: unique symbol };
```

Frontend rules:

- Keep API and form money as decimal strings; preserve rates as `RateString` through decoding and state.
- Use `bigint` minor units for exact client-side addition, subtraction, sorting, or comparison.
- Convert to JavaScript `number` only for non-authoritative visualization where rounding cannot change a submitted value.
- Never submit a total, balance, or payout calculated with floating-point arithmetic.
- Format display values from the canonical decimal string or minor units; `toFixed(2)` on a JavaScript number is not the money model.
- Parse rates into `bigint` micro-units and format display precision using integer half-up rounding.
- Multiply accumulator rates exactly and round once at the final money amount; submitted bills contain only stake and selections.
- Replace rate `.toFixed()` and truthiness fallback checks with explicit formatting and zero/null handling.
- The backend remains authoritative for bill totals, rates applied to a wager, rewards, and payouts.

See [the frontend handoff](../frontend-exact-decimals.md) for the client migration.

## Archive-and-reset rollout

The approved rollout archives the old production database and provisions a fresh database from the Goose baseline. No old rows are imported by default, no shadow columns or dual writes are introduced, and the old decimal columns do not exist in the fresh schema. The archive is retained for audit/reference and must be restore-tested before cutover.

## Required tests

- Parsing and formatting boundaries: `0`, one fractional digit, two fractional digits, malformed input, excess scale, negative input, and maximum value.
- Money JSON round trips produce canonical two-decimal strings and never use floating point.
- Rate JSON covers canonical six-decimal strings, short inputs, zero, maximum, overflow, invalid numeric/null input, and exact round trips.
- HTTP responses cover bill/nested-match rates, direct match rates, and Stake Mines game/history multipliers; scores/counts/indexes and approximate statistics stay numeric.
- Addition, subtraction, and rate multiplication cover exact results, rounding ties, underflow, and overflow.
- Database round trips preserve zero, ordinary values, and the maximum supported value.
- Migration tests prove fresh up/down/up behavior, constraints, and repeated-up no-ops.
- HTTP contract tests prove numeric Money tokens are rejected and output is canonical string form.
- Frontend contract tests cover decoding, display, exact sorting/addition, and form submission.
- Concurrent debit/credit tests verify balance constraints and transaction behavior.

## Consequences

Benefits:

- exact and deterministic money calculations
- direct PostgreSQL/sqlc mapping
- explicit overflow and insufficient-balance handling
- no accidental JavaScript floating-point money arithmetic
- one canonical API representation and display format

Costs:

- breaking JSON response types require a coordinated rollout
- current DTOs and frontend interfaces must change
- frontend rate types, formatting, and accumulator previews must migrate to six-decimal strings
- the reset requires an archive, restore rehearsal, fresh database, migration, and configuration cutover

## Rejected alternatives

### Continue using `float64`

Rejected because database exactness does not protect calculations and comparisons performed in Go or JavaScript.

### Store `NUMERIC(19,2)` and scan into `int64`

Rejected because the full scaled range exceeds `int64`, and every query would require conversion logic.

### Send money as JSON numbers with two printed decimal places

Rejected because the frontend parses them into JavaScript floating-point numbers and JSON numeric values do not retain a meaningful fixed-decimal type.

### Send minor units as JSON numbers

Rejected as the public contract because PostgreSQL `BIGINT`/Go `int64` values can exceed JavaScript's safe integer range. Minor units may be represented as frontend `bigint` only after parsing a string.
