# Swagger documentation

> **Breaking backend release:** Money values are two-decimal JSON strings, and odds/payout multipliers are six-decimal JSON strings. Bill rates are server-owned, bill/match lifecycle routes changed, and browser authentication uses an opaque session cookie. The frontend must migrate with this backend release.

The API documentation is generated from Go annotations with `swag` and served
by Fiber's Swagger UI middleware.

## Exact decimal values

`Money` and `SignedMoney` are JSON strings with two fractional digits; `Rate`
values are nonnegative JSON strings with six fractional digits. This applies to
bill-line `rate`, match `team_a_rate`/`team_b_rate` (including nested matches),
and Stake Mines game/history `multiplier`. For example, a rate of 1.75 is
`"1.750000"`. The API uses `/api/v1` with no temporary numeric/string dual format.

Rate decoding accepts quoted decimals with zero through six fractional digits
and normalizes output. Numeric tokens, null, negative values, exponents,
whitespace inside values, excess precision, and overflow are rejected.

Scores, counts, indexes, and approximate statistics remain JSON numbers.
Stake Mines `win_rate` is an approximate percentage, not an exact multiplier.
Use the [frontend handoff](frontend-exact-decimals.md) for exact parsing,
formatting, accumulator previews, and client acceptance checks. Rate storage and
arithmetic are unchanged; this serialization change needs no database migration.

## Sport type administration

Authenticated catalogue reads retain the `{ "id": "...", "title": "..." }`
shape. Sport management uses dedicated admin routes under `/api/v1`:

| Route | Access | Success |
| --- | --- | --- |
| `GET /sport-types` | Authenticated user | `200` list; empty catalogue is `[]` |
| `GET /sport-types/{id}` | Authenticated user | `200` resource |
| `POST /sport-types/admin` | Admin | `201` resource |
| `PATCH /sport-types/admin/{id}` | Admin | `200` resource |
| `DELETE /sport-types/admin/{id}` | Admin | `204`, empty body |

All mutations require the browser session, allowed Origin, and `X-CSRF-Token`.
Create accepts only `id` and `title`:

```json
{ "id": "BADMINTON_ALL", "title": "Badminton" }
```

IDs are immutable and contain 1–100 ASCII letters, digits, underscores, or
hyphens. IDs are not trimmed. Create and PATCH trim the title and require 1–100
Unicode characters. PATCH accepts only `{ "title": "New title" }`. Duplicate
titles are allowed; an existing ID returns `409 CONFLICT`. Missing resources
return `404 RESOURCE_NOT_FOUND`. Unknown fields and invalid bodies use the
shared `400 INVALID_REQUEST` contract.

The fresh Goose schema (`00001_fresh_schema.sql`) uses restrictive sport-type
foreign keys. Recreate local databases built from the older baseline before
enabling admin deletion; editing an applied baseline does not update them.
DELETE removes only unused entries. Matches, tournament groups, or stage
references block deletion with
`409 SPORT_TYPE_IN_USE`; dependent records and the sport entry remain intact.
Constraints enforce this during concurrent writes. Archiving is outside this API.

Catalogue seeding inserts missing defaults without overwriting edited titles.
Explicitly rerunning `make seed` can restore deleted default entries.

Use the [frontend handoff](frontend-sport-types.md) for admin controls and dynamic
selectors. Keep sport IDs as keys; display titles are mutable and may be duplicated.

## Access

Development:

```text
http://localhost:8080/swagger/index.html
```

Production:

```text
https://888api.chula.engineering/swagger/index.html
```

The development configuration leaves the UI open. Production requires HTTP
Basic Auth using `SWAGGER_USERNAME` and `SWAGGER_PASSWORD`.

The runtime OpenAPI document is available at `/swagger/doc.json`.

The **Authorize** button accepts external JWTs only for `/external/*`. Browser routes use the session cookie and CSRF header.

## Generate the specification

Regenerate the tracked files after changing Swagger annotations:

```bash
make docs
```

Check that the tracked generated files are current without changing them:

```bash
make docs-check
```

The API target used by **Try it out** comes from `SERVER_URL`, including the
`/api/v1` base path.

## Access policy administration

The browser-only policy API is available at `/api/v1/auth/policies`. It
requires the HttpOnly session cookie, an administrator with `role_id=ADMIN`,
and exact-Origin and session-bound CSRF protections. Frontend
requests must use `credentials: "include"` and send the `csrf_token`
returned by `/auth/me` in `X-CSRF-Token` for mutations. Bearer-only
`/external/*` authentication is not accepted for this API.

### Policy resource

```json
{
  "id": "2c2f1f7e-6ca5-4e76-a355-1d50b5e77c52",
  "kind": "blacklist",
  "principal_type": "email",
  "principal": "blocked@example.com",
  "reason": "Account suspended",
  "enabled": true,
  "expires_at": null,
  "created_at": "2026-09-26T02:00:00Z",
  "updated_at": "2026-09-26T02:00:00Z"
}
```

`kind` is `allowlist` or `blacklist`. Allowlist entries accept only
`principal_type=email`; blacklist entries accept `email` or
`google_subject`. Emails are normalized to lowercase. `reason` is required
and limited to 500 characters. `expires_at` is either `null` or a future
RFC3339 timestamp. Admin users are implicitly allowlisted and do not require
an allowlist row; blacklist rules still take precedence.

### Routes

`GET /api/v1/auth/policies` lists active entries by default. Optional query
parameters are `kind`, `principal_type`, `status` (`active`, `inactive`, or
`all`), `limit` (1–200), and an opaque `cursor`. The response is:

```json
{
  "items": [],
  "next_cursor": null
}
```

`POST /api/v1/auth/policies` creates an enabled entry:

```json
{
  "kind": "allowlist",
  "principal_type": "email",
  "principal": "partner@example.com",
  "reason": "Approved external participant",
  "expires_at": "2027-01-01T00:00:00Z"
}
```

It returns `201 Created` with the policy resource. Duplicate identities return
`409 Conflict`.

`PATCH /api/v1/auth/policies/:id` changes only `reason`, `expires_at`, or
`enabled`:

```json
{
  "reason": "Approval revoked",
  "expires_at": null,
  "enabled": false
}
```

It returns `200 OK` with the updated resource. `DELETE
/api/v1/auth/policies/:id` disables the entry and returns `204 No Content`.
Errors use `{ "error": "..." }` with `400`, `401`,
`403`, `404`, `409`, or `503` status codes as appropriate.

The existing `cmd/seed` binary supports an explicit `--policy-file
/secure/path/policies.json --policy-dry-run` validation mode. The import file
must remain outside the repository and has this envelope:

```json
{
  "entries": [
    {
      "kind": "allowlist",
      "principal_type": "email",
      "principal": "partner@example.com",
      "reason": "Approved external participant",
      "expires_at": null
    }
  ]
}
```

Validate it first, then import it without `--policy-dry-run`:

```bash
go run ./cmd/seed --policy-file /secure/path/policies.json --policy-dry-run
go run ./cmd/seed --policy-file /secure/path/policies.json
```

Existing administrators are promoted or demoted manually by an operator
through `users.role_id`; the user-update API does not change roles.

# Updating your profile

Use `PATCH /api/v1/users/me` with the session cookie, configured frontend
`Origin`, and `X-CSRF-Token` obtained from `GET /api/v1/auth/me`. The API takes
the account ID from authentication. The request accepts only `name`,
`nick_name`, and `group_id`:

```json
{
  "nick_name": "Oak",
  "group_id": null
}
```

Send only fields to change. Omitted fields remain unchanged; `null` clears
`nick_name` or `group_id`. A supplied `name` must be a nonempty string. A group
ID must identify an existing group, or be null. Empty updates, invalid values,
and unknown fields return `400 INVALID_REQUEST`. The API rejects `id`, `email`,
`role_id`, and `remaining_coin` in this request. Success returns `200` with the
updated profile in the existing profile response format.

**Deprecated:** `PATCH /api/v1/users/:id`. Migrate profile edits to `/users/me`.
The legacy endpoint requires `:id` to equal the signed-in user's ID; a mismatch
returns `403 FORBIDDEN`. For matching IDs it delegates to the same handler as
`/users/me`, using the same editable fields, omission behavior, and nullable
clears. The old identity, email, and role request fields are rejected.
`GET /users/:id` remains available.

Administrators continue to use `PATCH /api/v1/users/admin/:id` for edits to other
accounts through the existing administrator-only route.

# Browser authentication contract

The frontend must use `credentials: "include"` for API requests. The only browser credential is the API's HttpOnly session cookie. Do not read or store an OAuth access token, refresh token, session ID, or external JWT in frontend JavaScript.

1. Call `GET /api/v1/auth/login` with the frontend `Origin` header and navigate the browser to the returned `url`.
2. Google redirects to `GET /api/v1/auth/callback`. The API sets the session cookie and redirects to the configured frontend URL. Only a first registration adds `is_new_user=true`.
3. Call `GET /api/v1/auth/me` with credentials. The response contains the existing `profile` object and a `csrf_token` string. Keep the CSRF token in memory and send it in `X-CSRF-Token` on POST, PUT, PATCH, and DELETE requests. Send the exact configured frontend `Origin` on all requests. Reload `/auth/me` after a page refresh.
4. Call `POST /api/v1/auth/logout` with credentials and the CSRF header. Discard the in-memory token after a 204 response. Logout is also 204 when the session is absent or expired. A 503 means revocation could not be confirmed; keep the cookie and retry.

A 401 on a browser route means the session is missing or expired; clear frontend profile and CSRF state, then offer login. A 503 means Redis, user, or policy status is uncertain; retain the frontend state and retry. The browser no longer uses `/auth/refresh` or `Authorization: Bearer`.

Development on `http://localhost:3000` and `http://localhost:8080` uses HttpOnly `session` and `oauth` cookies with `SameSite=Lax`. Production on same-site HTTPS subdomains uses `__Host-session` and temporary `__Host-oauth`, with `Secure`, `HttpOnly`, `Path=/`, and `SameSite=Lax`; neither cookie has a `Domain` attribute. The API clears legacy `access_token`, `refresh_token`, `csrf_token`, and `oauth_state` cookies during login and callback.

## External minigame backend

The `/api/v1/external/*` API is used by a separate backend that runs other
minigames, such as the war game, and reports game outcomes to this backend.
The integration purpose is now confirmed. Current Swagger annotations still
mark the external routes and their token-management routes deprecated.

The implemented routes are `GET /api/v1/external/me`, which returns the
JWT-bound user's profile, and `POST /api/v1/external/deduct-coin`, which deducts
coins from that user's balance using a money-string `amount`. External requests
use Bearer authentication independently of browser cookies, Origin checks,
and CSRF. The token identifies a user rather than the minigame service itself.

There is currently no external endpoint for submitting game results or crediting
winnings. That part of the intended integration still needs a defined contract.

External clients use a separate, revocable one-hour JWT. An authenticated administrator can issue one for an existing user with `POST /api/v1/auth/external-tokens` and body `{"user_id":"..."}`; the response contains `token`, `id`, and `expires_in`. The administrator can revoke it with `DELETE /api/v1/auth/external-tokens/{id}`. Both administrative mutations require the browser session, allowed Origin, and CSRF header. The regular frontend should not request these tokens. Existing external JWTs must be reissued through this API at cutover.

# API error contract and frontend handoff

Every failed `/api/v1` request returns a stable code, a safe message, and the
server generated request ID. `X-Request-ID` contains the same value and is
exposed to configured browser origins. Validation failures can include
`details` keyed by safe request field names. Clients should branch on `code`
and display `message`; they must not parse message text. Unknown server errors
are returned as `INTERNAL_ERROR` without internal details.

```json
{
  "code": "DAILY_REWARD_ALREADY_CLAIMED",
  "message": "Daily reward already claimed",
  "request_id": "c48c6fe7-c83e-4e1f-a4d8-78370f2a94fd"
}
```

The code catalog follows the status policy: `INVALID_REQUEST` (400),
`UNAUTHORIZED` (401), `FORBIDDEN` (403), `RESOURCE_NOT_FOUND` (404),
`DAILY_REWARD_ALREADY_CLAIMED`, `CONFLICT`, and domain state-conflict codes (409),
`INSUFFICIENT_BALANCE` (422), `TOO_MANY_REQUESTS` (429),
`DEPENDENCY_UNAVAILABLE` (503), and `INTERNAL_ERROR` (500). Domain conflict
codes include `BILL_CONFLICT`, `MATCH_RESULT_CONFLICT`, `GAME_STATE_CONFLICT`,
`STEAL_TOKEN_CONFLICT`, and `STEAL_TOKEN_INVALID`.

Frontend migration steps:

- Preserve structured failures through API wrappers that currently swallow or
  replace them. Show `message` and use `code` for behavior.
- Replace bill message matching with codes such as `INSUFFICIENT_BALANCE`,
  `BILL_CONFLICT`, and `RESOURCE_NOT_FOUND`.
- Advance the daily reward claim button only after a successful response. A
  `DAILY_REWARD_ALREADY_CLAIMED` conflict may show the already-claimed state;
  other failures must remain visible and retryable.
- Include `X-Request-ID` in support reports so operators can find the server
  log entry.

The frontend migration is outside this backend branch.

## Coding standard

The required layout, documentation and dependency rules are documented in
[coding-standard.md](coding-standard.md). Run `make fmt` before `make ci`.
Behavior intentionally retained during normalization is recorded in
[refactor-followups.md](refactor-followups.md).

Browser routes use the `CookieSession` documentation scheme. Swagger 2.0 does
not support a native cookie security scheme; its `Cookie` header representation
is descriptive. Browser cookies are supplied by an authenticated browser session,
and protected mutations also require `X-CSRF-Token` and an allowed Origin.
`BearerAuth` applies only to the external minigame integration endpoints.
