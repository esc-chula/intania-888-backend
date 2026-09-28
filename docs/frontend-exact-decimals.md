# Frontend handoff: exact decimal API strings

The backend now serializes money as two-decimal strings and odds/payout
multipliers as six-decimal strings. This is a coordinated breaking release
under `/api/v1`, with no numeric/string dual format. Backend implementation is
complete separately; the client changes and acceptance checks below belong to
the frontend migration.

## API types and boundaries

| Value / field | Frontend representation |
| --- | --- |
| Money and signed money | Existing decimal-string money type; two fractional digits |
| Bill-line `rate` | Distinct `RateString`; six fractional digits |
| Match `team_a_rate`, `team_b_rate` | `RateString`, including matches nested in bills |
| Stake Mines game/history `multiplier` | `RateString` |
| Scores, counts, indexes | `number` |
| Stake Mines `win_rate` | `number`; approximate percentage |

```ts
type RateString = string & { readonly __rate: unique symbol };
```

Keep the wire string through API decoding and UI/store state. Validate the
string at the API boundary; do not accept numeric tokens, coerce with `Number`
or `parseFloat`, or turn rates into the money type. Canonical zero is
`"0.000000"`; canonical one is `"1.000000"`.

The backend decoder accepts quoted decimal strings with zero through six
fractional digits and preserves existing leading-zero parsing. It rejects
negatives, exponents, whitespace inside the value, excess precision, null,
and overflow. Responses always have exactly six fractional digits. Maximum
micro-units are `9223372036854775807`, corresponding to
`"9223372036854.775807"`.

## Changes in the current frontend

Reference: sibling `intania-888-frontend` at `105fd51`, inspected on 2026-09-29.
Recheck these consumers before implementation if that branch changes.

| Consumer | Required change |
| --- | --- |
| `src/api/slip/slip.dto.ts` | Change bill and nested match rates to `RateString`; remove legacy create-line rate. |
| `src/components/match/MatchInterface.tsx` | Change API match rates and `RoundItem.rateA`/`rateB` to rate strings; keep scores numeric. |
| `src/components/match/MatchUtils.tsx` | Stop converting rates with `Number`; keep score conversions separate. |
| `src/components/match/MatchRound.tsx` and `MatchBar.tsx` | Format odds explicitly; preserve numeric score props for actual scores. |
| `src/store/slip.ts` | Store rate strings, replace floating-point multiplication, and clear old persisted numeric-rate slips. |
| Slip elements/results and `src/app/slip/page.tsx` | Replace rate `.toFixed()` and numeric payout previews with shared exact helpers. |
| `src/api/event/stakemine.ts` | Change game multiplier to `RateString`; add matching history types where consumed. |
| `src/components/stakemine/MineTable.tsx` | Keep multiplier state as a string and use shared formatting; preserve numeric counts/indexes. |

For persisted slip state, bump the persistence version and discard old
numeric-rate selections at cutover. Reload server odds for newly selected
matches. This avoids accepting stale numeric state as a supported API format.

## Shared exact helpers

Add one rate helper module beside the existing money helpers. Expose a boundary
validator returning `RateString`, a parser returning `bigint` micro-units,
rate formatting, and exact accumulator-preview functions.

For a decimal string `whole.fraction`, pad the fraction to six places and compute:

```text
micro = BigInt(whole) * 1_000_000 + BigInt(padded_fraction)
```

Validate decimal characters and the nonnegative int64 range before accepting
it. Normalize shorter inputs to canonical six digits in the boundary validator.
Never pass through a JavaScript number. The frontend's current TypeScript config
has no explicit target: use the `BigInt(...)` constructor instead of bigint
literals so the helpers work with that configuration.
Keep bigint values out of JSON and persisted state; store decimal strings and
calculate bigint values when needed.

For half-up rounding of a nonnegative rational `numerator / denominator`:

```text
rounded = (2 * numerator + denominator) / (2 * denominator)
```

All operands are bigint and division is integer division. Require a positive
denominator. To display a rate with `d` digits, use numerator
`micro * 10^d` and denominator `1_000_000`, then split the rounded integer into
whole/fractional display digits. Pad the fraction to exactly `d` places.
Keep the original API value in state; display rounding must not replace it.

For an accumulator payout preview, parse the stake to bigint minor units:

```text
numerator   = stake_minor * product(rate_micro[i])
denominator = 1_000_000 ^ number_of_rates
payout_minor = half_up(numerator / denominator)
```

Multiply the full numerator/denominator first, then round once. Do not round an
intermediate total multiplier or each leg's money amount. An empty rate list
has product/denominator one. For previews involving resolved draws, a draw
contributes the exact neutral multiplier `"1.000000"`, matching settlement.
Reject a final money result outside the backend's nonnegative int64 minor-unit
range; surface the failed preview instead of displaying an imprecise number.
For combined-odds display, format the unrounded product ratio directly.

The preview is informational: `POST /bills` still submits only the stake
`total` and line `match_id`/`betting_on`. Use accepted server rates and payouts
after placement.

## Zero and null behavior

String `"0.000000"` is truthy. Replace `rate || 2` and loose numeric comparisons
with explicit null/undefined checks and an exact zero check using micro-units.

Preserve existing display/selection defaults: where zero or missing odds used
numeric `2`, use `"2.000000"` after an explicit zero/missing check. Where a missing
match rate mapped to zero, use `"0.000000"`. The Mines multiplier's missing-value
default remains one as `"1.000000"`; a supplied zero must remain zero. The backend
rate fields themselves are non-null; null handling applies to existing UI state
or missing responses, not an additional accepted Rate JSON format.

## Client acceptance checks and release

- Type-check the updated frontend with `pnpm exec tsc --noEmit --incremental false`.
- Verify `"0.000000"`, `"1.750000"`, and `"1.234567"` in match, bill, and Mines UI;
  API/store values remain strings and rate formatting never calls `.toFixed()`.
- With two display digits, `"1.234999"` renders `1.23` and `"1.235000"` renders
  `1.24`; a carry such as `"1.999999"` renders `2.00`.
- For a `"0.01"` stake and two `"1.500000"` rates, the preview is `"0.02"`.
  Rounding each leg first would incorrectly produce `"0.03"`.
- For a `"1.00"` stake and `"1.005000"`, half-up payout is `"1.01"`.
- Preserve exact parsing of `"9007199254.740993"` to micro-units
  `9007199254740993`, above JavaScript's safe integer range.
- Verify explicit zero/null fallback behavior, cleared old persisted slips,
  nonnumeric-input rejection, and a handled payout-overflow preview.
- Confirm bill requests contain stake/selections only and server responses remain
  authoritative. Money is still two-decimal strings; `win_rate`, scores, counts,
  and indexes remain numeric.
- Deploy the backend and compatible clients together. Smoke-check match odds,
  bill placement/readback, Mines game/history multipliers, and payout previews.

These frontend checks are release requirements for the client implementation;
backend tests alone do not establish frontend compatibility.
