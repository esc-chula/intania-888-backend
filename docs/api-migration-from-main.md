# API migration: backend `main` to current `development`

This guide records the client-visible contract changes between the backend
`main` branch and the current `development` branch.

## Compared revisions

- Backend `main`: `3c82b1d` (`origin/main`)
- Backend `development` (current HEAD): `65deb2a`
- API base path: still `/api/v1`

## Contract changes

| Area | From backend `main` | To backend `development` | Client action |
| --- | --- | --- | --- |
| Browser authentication | OAuth redirects could return access and refresh tokens in the URL. The frontend exchanged codes through `POST /auth/login/callback`, saved credentials in `localStorage`, sent `Authorization: Bearer`, and refreshed through `POST /auth/refresh`. | `GET /auth/login` returns `{ "url": "..." }` and binds OAuth state to an HttpOnly cookie. Google returns to `GET /auth/callback`, which sets an opaque session cookie and redirects to the configured frontend. `GET /auth/me` returns `{ profile, csrf_token }`; `POST /auth/logout` revokes the session and returns `204`. There is no browser refresh-token route. | Remove browser JWT/localStorage handling and the callback exchange. Set `withCredentials: true`; load the CSRF token from `/auth/me`, keep it in memory, and send `X-CSRF-Token` on every mutation. Navigate to the URL returned by `/auth/login`. |
| Browser request protection | Bearer authentication was the normal browser API credential. | Browser routes use the HttpOnly `session` cookie in local development and `__Host-session` in production. State-changing requests require the session-bound CSRF token and an allowed exact `Origin`. `/external/*` remains a separate, deprecated Bearer-only integration. | Configure the frontend origin in backend CORS/origin settings. Do not attach Bearer tokens to browser routes. Treat `401` as an expired/missing session and `503` as an uncertain session/policy dependency. |
| Money | Money values were represented as JSON numbers and floating-point values in DTOs. | `Money` is a non-negative fixed-point value encoded as a JSON string with exactly two decimal places, for example `"888.88"`. Numeric JSON money, exponent notation, negative values, and precision beyond two decimals are rejected. `Rate` and game multipliers remain JSON numbers. | Type API money as strings. Send quoted decimal values. Convert to a JavaScript number only at UI boundaries; do not send those converted values back as money. |
| Bill creation | Bill line payloads included a client-provided `rate`; creation returned a success message. Bill routes also exposed `PATCH /bills/{id}` and `DELETE /bills/{id}`. | `POST /bills` accepts a money-string `total` and line selections containing only `match_id` and `betting_on`. The backend calculates and snapshots rates. The response is the bill, including status, payout and lifecycle timestamps. The client update/delete routes are removed; admins use `PUT /bills/admin/{id}/void` with an audit `reason`. | Remove `rate` from create payloads. Do not use client rates or totals to calculate authoritative payouts. Replace bill deletion with the admin void workflow where applicable. |
| Match result | Winner and draw used separate routes: `PATCH /matches/{id}/winner/{winner_id}` and `PATCH /matches/{id}/draw`. | Both use `PUT /matches/{id}/result`: `{ "outcome": "winner", "winner_id": "..." }` or `{ "outcome": "draw" }`. `PATCH /matches/{id}/score` remains. Setting a result can settle bills and can return a `409` conflict. | Switch both result actions to the single PUT route and handle conflict responses. |
| Daily reward administration | Admins used `POST /events/daily-rewards` with the date and amount in the body. | Admins read the schedule with `GET /events/daily-rewards`, replace a date override with `PUT /events/daily-rewards/{date}` and `{ "amount": "0.00" }`, or remove it with `DELETE /events/daily-rewards/{date}`. Dates use `DD-MM-YYYY`. | Use the schedule and per-date routes. Keep the amount as a money string. The user claim route `GET /events/redeem/daily` remains. |
| Self profile | `PATCH /users/{id}` accepted a broad user DTO. | Preferred self-service route is `PATCH /users/me`; its partial body permits only `name`, `nick_name`, and `group_id`. `PATCH /users/{id}` remains as a deprecated alias and only accepts the signed-in user's own ID. Neither route can change `remaining_coin`. Admin balance updates use `PATCH /users/admin/{id}` and a money-string `remaining_coin`. | Move self-profile updates to `/users/me`; remove `remaining_coin` and identity/role fields from that request. Do not implement browser coin credits through profile updates. |
| Errors and request bodies | Failures used endpoint-specific `error` or `message` objects; JSON decoding was permissive in many handlers. | API failures use `{ code, message, request_id, details? }`; the same ID is in `X-Request-ID`. Request DTOs reject unknown fields and malformed/non-object JSON. | Branch on `code`, display `message`, and include `request_id` in support diagnostics. Do not depend on message text or send legacy fields such as `rate` and `remaining_coin`. |

## Routes added or removed

| Change | Route |
| --- | --- |
| Added | `PATCH /users/me` |
| Added | `GET /events/daily-rewards` |
| Added | `PUT /events/daily-rewards/{date}` |
| Added | `DELETE /events/daily-rewards/{date}` |
| Added | `PUT /bills/admin/{id}/void` |
| Added | `GET/POST /auth/policies`, `PATCH/DELETE /auth/policies/{id}` (admin only) |
| Added, deprecated external integration | `POST /auth/external-tokens`, `DELETE /auth/external-tokens/{id}` |
| Removed | `POST /auth/login/callback`, `POST /auth/refresh` |
| Replaced | `PATCH /matches/{id}/winner/{winner_id}` and `PATCH /matches/{id}/draw` → `PUT /matches/{id}/result` |
| Replaced | `POST /events/daily-rewards` → schedule GET plus per-date PUT/DELETE |
| Removed | `PATCH /bills/{id}` and `DELETE /bills/{id}`; use admin void for a pending bill |

`GET /sport-types` is present in both backend revisions. The frontend can use
it instead of maintaining a hardcoded sport-type list, but this is not a new
backend route in this comparison.

## Migration sequence

1. **Coordinate the release.** The old browser client cannot authenticate to
   the new browser API because it sends Bearer tokens and has no session cookie
   or CSRF token. Deploy the backend and matching frontend as one cutover.
2. **Check backend configuration.** Set the exact frontend origin in CORS and
   origin policy, configure the OAuth frontend redirect, and provide Redis for
   session storage. Production browser sessions require HTTPS and the
   `__Host-session` cookie policy.
3. **Prepare database state separately.** This backend uses Goose migrations
   (`make migrate-status`, then `make migrate-up`) and an idempotent reference
   data seed (`make seed`). Review the existing database against the migration
   baseline before applying it to an already-populated deployment; this API
   guide does not claim a legacy database conversion has been run. Do not use
   `migrate-down` or `migrate-reset` as a production upgrade step.
4. **Update browser auth and shared transport.** Remove token storage, token
   refresh, and the frontend callback code. Enable credentials, fetch CSRF
   state from `/auth/me`, and attach it to POST/PUT/PATCH/DELETE requests.
5. **Update payloads and response types.** Change money fields to strings,
   remove bill-line rates, move match result updates, migrate daily reward
   overrides, and switch profile updates to `/users/me`.
6. **Update error handling and admin screens.** Preserve the API error
   envelope, expose the request ID, use `/auth/policies`, and retain the new
   server-provided sport catalogue.
7. **Smoke-check the cutover.** Sign in and reload, verify `/auth/me`, perform
   one protected mutation with CSRF, update the current profile, place and read
   a bill, set/delete a daily override as admin, and set a match result. Confirm
   that missing CSRF returns `403`, an expired session returns `401`, and a
   bill/match conflict remains visible as a `409`.

## Source references

- [Backend API reference and error contract](README.md)
- [Generated OpenAPI specification](swagger.yaml)
- [Authentication routes](../internal/domain/auth/adapter.http.go)
- [Bill routes and DTOs](../internal/domain/bill/adapter.http.go)
- [Match routes and DTOs](../internal/domain/match/adapter.http.go)
- [Event routes and DTOs](../internal/domain/event/adapter.http.go)
- [User routes and DTOs](../internal/domain/user/adapter.http.go)
