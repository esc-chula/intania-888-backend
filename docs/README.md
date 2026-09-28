# Swagger documentation

> **Breaking backend release:** Money values are fixed two-decimal JSON strings, bill rates are server-owned, bill/match lifecycle routes changed, and browser authentication now uses an opaque session cookie. The frontend must migrate with this backend release.

The API documentation is generated from Go annotations with `swag` and served
by Fiber's Swagger UI middleware.

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

# Browser authentication contract

The frontend must use `credentials: "include"` for API requests. The only browser credential is the API's HttpOnly session cookie. Do not read or store an OAuth access token, refresh token, session ID, or external JWT in frontend JavaScript.

1. Call `GET /api/v1/auth/login` with the frontend `Origin` header and navigate the browser to the returned `url`.
2. Google redirects to `GET /api/v1/auth/callback`. The API sets the session cookie and redirects to the configured frontend URL. Only a first registration adds `is_new_user=true`.
3. Call `GET /api/v1/auth/me` with credentials. The response contains the existing `profile` object and a `csrf_token` string. Keep the CSRF token in memory and send it in `X-CSRF-Token` on POST, PUT, PATCH, and DELETE requests. Send the exact configured frontend `Origin` on all requests. Reload `/auth/me` after a page refresh.
4. Call `POST /api/v1/auth/logout` with credentials and the CSRF header. Discard the in-memory token after a 204 response. Logout is also 204 when the session is absent or expired. A 503 means revocation could not be confirmed; keep the cookie and retry.

A 401 on a browser route means the session is missing or expired; clear frontend profile and CSRF state, then offer login. A 503 means Redis, user, or policy status is uncertain; retain the frontend state and retry. The browser no longer uses `/auth/refresh` or `Authorization: Bearer`.

Development on `http://localhost:3000` and `http://localhost:8080` uses HttpOnly `session` and `oauth` cookies with `SameSite=Lax`. Production on same-site HTTPS subdomains uses `__Host-session` and temporary `__Host-oauth`, with `Secure`, `HttpOnly`, `Path=/`, and `SameSite=Lax`; neither cookie has a `Domain` attribute. The API clears legacy `access_token`, `refresh_token`, `csrf_token`, and `oauth_state` cookies during login and callback.

**Deprecated:** The `/external/*` routes and their external-token management routes remain available while their original purpose and consumers are investigated. Do not build new integrations against them.

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
