# Swagger documentation

> **Breaking backend release:** Money values are two-decimal JSON strings, and odds/payout multipliers are six-decimal JSON strings. Bill rates are server-owned, bill/match lifecycle routes changed, and protected browser operations use an opaque session cookie. Shared catalogue, fixture, and standings reads are public. The frontend must migrate with this backend release.

The API contract is maintained in [`openapi.yaml`](openapi.yaml) and served by
Fiber's Swagger UI middleware. Keep route comments in Go concise; put request,
response, security, and schema details in the OpenAPI file.

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
Use the [exact-decimal frontend migration](api-migration-from-main.md#exact-decimal-frontend-migration)
for exact parsing, formatting, accumulator previews, affected consumers, and
client acceptance checks. Rate storage and arithmetic are unchanged; this
serialization change needs no database migration.

## Public shared reads

All routes below are relative to `/api/v1`. These shared reads do not require a
browser session or run account allowlist/blacklist checks:

| Data | Public routes | Result |
| --- | --- | --- |
| Locations | `GET /locations`, `GET /locations/{id}` | Location catalogue or one `{ id, title }` resource |
| Fixtures and server time | `GET /matches`, `GET /matches/{id}`, `GET /matches/current/time` | Grouped fixtures, one fixture, or current UTC time |
| Sports | `GET /sport-types`, `GET /sport-types/{id}` | Sport catalogue or one `{ id, title }` resource |
| Standings | `GET /colors/leaderboards`, `GET /colors/group-stage` | Color leaderboards or group-stage standings |

The API-wide limit still allows 200 requests per client IP per minute. Browser
requests with an `Origin` must use an exact configured origin; safe GET requests
without an `Origin` are accepted. CORS still grants access only to configured
origins. An unconfigured `Origin` returns `403 FORBIDDEN`, and an exceeded rate
limit returns `429 TOO_MANY_REQUESTS`.

This public access applies only to these shared reads. Account-specific,
game/history, billing, and administrative routes keep their documented access
requirements.

## Location catalogue and match venues

A location is a reusable venue with a stable ID and a mutable display title:

```json
{
  "id": "CIVIL_COURT",
  "title": "สนามโยธา"
}
```

All five location operations are documented in the OpenAPI specification. The
read routes are public; location changes are administrator-only:

| Route | Access | Success |
| --- | --- | --- |
| `GET /locations` | Public | `200` array, ordered by ID; `[]` when empty |
| `GET /locations/{id}` | Public | `200` resource |
| `POST /locations/admin` | Admin | `201` resource |
| `PATCH /locations/admin/{id}` | Admin | `200` renamed resource |
| `DELETE /locations/admin/{id}` | Admin | `204`, empty body, if unused |

Location mutations require a browser session, admin permission, allowed Origin,
and `X-CSRF-Token`. IDs are immutable and contain 1–100 ASCII letters, digits,
underscores, or hyphens. Titles are trimmed and must contain 1–100 Unicode
characters. Duplicate titles are allowed. Create and rename requests are:

```json
{ "id": "COURT_A", "title": "Court A" }
```

```json
{ "title": "North Court" }
```

The first body is for `POST /locations/admin`; the second is the only accepted
field for `PATCH /locations/admin/{id}`. Invalid IDs or titles return
`400 INVALID_REQUEST`; duplicate IDs return `409 CONFLICT`; missing IDs return
`404 RESOURCE_NOT_FOUND`. Deleting a location assigned to a match returns
`409 LOCATION_IN_USE`. Reassign or remove those matches first.

Match creation requires `location_id` from the public catalogue. Match updates
may supply it to move the match and leave it out to keep the current location.
For example, a match request uses the stable ID:

```json
{
  "team_a": "TEAM_A",
  "team_b": "TEAM_B",
  "type": "BADMINTON_ALL",
  "location_id": "COURT_A",
  "start_time": "2026-10-01T09:00:00Z",
  "end_time": "2026-10-01T10:00:00Z"
}
```

Match responses include `location: { "id": "COURT_A", "title": "Court A" }`;
bill-line responses include the same location inside their nested match. Renaming
a location preserves its ID and updates the title returned for existing matches.
The database foreign key rejects unknown location IDs, and `ON DELETE RESTRICT`
protects every assigned location even when writes race with deletion.

`make seed` inserts the default location catalogue after migrations. It adds
missing entries without overwriting edited titles; explicitly rerunning it can
restore a deleted default. Migration `00004_match_locations.sql` makes
`matches.location_id` required and enforces the location reference in the
database.

## Sport type administration

Sport resources retain the `{ "id": "...", "title": "..." }` shape. The GET
catalogue routes above are public. Management writes use dedicated admin routes:

| Route | Access | Success |
| --- | --- | --- |
| `POST /sport-types/admin` | Admin | `201` resource |
| `PATCH /sport-types/admin/{id}` | Admin | `200` resource |
| `DELETE /sport-types/admin/{id}` | Admin | `204`, empty body |

All mutations require the browser session, allowed Origin, and `X-CSRF-Token`.
Create accepts only `id` and `title`:

```json
{ "id": "BADMINTON_ALL", "title": "Badminton" }
```

IDs are immutable and contain 1–100 ASCII letters, digits, underscores, or
hyphens. Both uppercase and lowercase letters are accepted; uppercase is a
naming convention. IDs are not trimmed or converted to uppercase. Create and
PATCH trim the title and require 1–100 Unicode characters. PATCH accepts only
`{ "title": "New title" }`. Duplicate
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

Use the [sport-type frontend migration](api-migration-from-main.md#sport-type-frontend-migration)
for admin controls, dynamic selectors, catalogue state, error handling, and
client acceptance checks. Keep sport IDs as keys; display titles are mutable
and may be duplicated.

## Access

Development:

```text
http://localhost:8080/swagger/index.html
```

Production example, matching the example auth registry:

```text
https://888-api.intania.org/swagger/index.html
```

Use your deployed API host; production example domains are not deployment
authorities. `SERVER_URL` selects the API target, and `google.callback_uri` must
use the callback URL registered with Google.

The development configuration leaves the UI open. Production requires HTTP
Basic Auth using `SWAGGER_USERNAME` and `SWAGGER_PASSWORD`.

The manually maintained OpenAPI 3.0.3 document is available at
`/swagger/openapi.yaml`. The former `/swagger/doc.json` URL redirects there.

Swagger UI has **Sign in with Google** and **Refresh session** controls. It uses
the browser's HttpOnly session cookie, reads the CSRF token from `/auth/me` for
mutations, and shows the latest `X-Request-ID` with a copy button. When Swagger
is enabled, the API origin is allowed so same-origin Try It Out requests pass
the origin check. The browser helper is maintained in
[`cmd/server/swagger-session.js`](../cmd/server/swagger-session.js) and embedded
in the server binary.

Swagger sign-in uses its own registered cookie application and returns the
current tab to `/swagger/index.html` after Google login. Select **Refresh
session** there before trying protected routes. The `intania-888-web`
application still returns to the frontend on port 3000 in development.

Protected browser routes use the session cookie; their mutations also require
the CSRF header and an allowed Origin. Public shared reads do not require a
session. The separate backend integration's Bearer flow is described under
[External minigame backend](#external-minigame-backend).

## Maintain the specification

Edit [`openapi.yaml`](openapi.yaml) directly when an API route or wire contract
changes. Handler comments describe route purpose; the OpenAPI file owns the
request, response, security, and schema documentation.

```bash
make openapi-check
```

This runs the pinned kin-openapi validator to check OpenAPI structure, schemas,
and references. It does not compare documented routes or behavior with handlers.

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
Policy errors use the shared `{ "code", "message", "request_id", "details?" }`
resource envelope with the appropriate HTTP status. The OAuth token error
format does not apply to policy resources.

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
accounts through the administrator-only route. It requires a nonempty `name`;
include the intended `remaining_coin` because an omitted value is written as
`0.00`. An omitted `nick_name` clears it, and `role_id` remains outside the API.

# Browser authentication contract

The frontend must use `credentials: "include"` for API requests. The only browser credential is the API's HttpOnly session cookie. Do not read or store an OAuth access token, refresh token, session ID, or external JWT in frontend JavaScript.

1. Navigate the browser to `GET /api/v1/auth/login?client_id=intania-888-web&return_to=/`. This is a redirect endpoint; do not fetch a JSON login URL. `client_id` must identify a registered cookie application. `return_to` is an optional safe relative path on that application's configured origin.
2. Google redirects to `GET /api/v1/auth/callback`. The API validates and consumes the browser-bound transaction, sets its session cookie, and redirects to the registered frontend. New accounts go to the application's `onboarding_path`, with the original relative destination in `return_to`.
3. Call `GET /api/v1/auth/me` with credentials. Keep the returned `csrf_token` in memory and send it as `X-CSRF-Token` on protected mutations. Browsers supply the Origin header; configure its exact value in CORS.
4. Call `POST /api/v1/auth/logout` with credentials and CSRF. A `204` confirms logout, including an absent or expired session. A `503` leaves revocation unconfirmed; retain state and retry.

Development uses `session` and separate `oauth-tx-<state>` cookies with
`SameSite=None; Secure`, allowing localhost frontends to call a hosted HTTPS
development API. Production uses `__Host-session` and `__Host-oauth-tx-<state>`
with `SameSite=Lax; Secure`. Both environments use HttpOnly, Path=/, and no Domain
attribute, including renewal and deletion. Configure the exact frontend origin
in CORS and the application registry; retain CSRF and callback allowlists.
Browser third-party-cookie restrictions still apply. Use local HTTPS for
consistent browser support or a local proxy/BFF when cross-site cookies are
blocked. Separate transaction cookies allow concurrent login attempts.

A `401` on a protected browser route means authentication is missing or expired. A dependency `503` permits retry without discarding browser state. The browser does not use `/auth/refresh` or store bearer credentials.

See [Application authentication](authentication-applications.md) for registration, backend delegation, configuration, and credential requirements.

## External minigame backend

The separate game backend uses the delegated resources documented in
[`openapi.yaml`](openapi.yaml). It obtains credentials through the registered
[authorization-code flow](authentication-applications.md#backend-delegation),
keeps them server-side, and issues its own game session cookie.

| Route | Required scope | Success |
| --- | --- | --- |
| `GET /external/me` | `profile.read` | `200`, `{ "profile": { ... } }`; no CSRF token |
| `POST /external/deduct-coin` | `coins.spend` | `200`, deduction and remaining balance |

Deduction takes only a JSON money-string amount:

```json
{ "amount": "25.00" }
```

```json
{
  "success": true,
  "deducted_amount": "25.00",
  "remaining_balance": "863.88"
}
```

The token binds the account, registered client, delegation, and scopes. No caller
`user_id` is accepted. Cookie credentials do not authenticate external routes.
Missing/invalid/revoked credentials return `401 UNAUTHORIZED`; missing scope
returns `403 FORBIDDEN`; insufficient balance returns `422 INSUFFICIENT_BALANCE`.
Dependencies fail closed with `503 DEPENDENCY_UNAVAILABLE`.

External routes skip the browser Origin guard and CSRF. Browser CORS still
restricts browser requests; backend-to-backend calls do not need a CORS entry.
The delegated Games flow does not call 888 resources from its frontend, so
register the Games frontend origin on the Games backend. Register its backend
callback under `redirect_uris` in the 888 auth registry.

888 deduction is atomic within its database. It has no idempotency key,
reservation, refund, or transaction spanning the game database. Do not retry an
ambiguous deduction response without a reconciliation design. Game-result
submission and winnings credit endpoints are not implemented.

# API error contract and frontend handoff

`/auth/token` and `/auth/revoke` protocol failures use `{ "error": "..." }`.
Login and authorization use registered error redirects (`error`, plus `state`
for backend clients) or a local HTML error when a trusted destination cannot
be established. The shared API-wide rate limiter can return the resource
envelope with `429` on these routes. See the
[authentication error table](authentication-applications.md#responses-and-errors).

Protected resource failures return a stable code, a safe message, and the
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

Browser routes use the `CookieSession` scheme for development and the
`SecureCookieSession` scheme for production. Both describe the environment's
HttpOnly session cookie; protected mutations also require `X-CSRF-Token` and an
allowed Origin. `BearerAuth` applies only to scoped external resources.
`OAuthClient` is HTTP Basic backend-client authentication for `/auth/token` and
`/auth/revoke`; it is separate from Swagger UI access protection and browser
account login.

Start cookie login by navigating to `/auth/login?client_id=intania-888-web`,
then return to Swagger in the same browser and hostname. Do not fetch login as
JSON or follow Google authentication through **Try it out**. Obtain CSRF from
`/auth/me` before an active-session mutation.
