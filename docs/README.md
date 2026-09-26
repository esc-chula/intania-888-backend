# Swagger documentation

> **Breaking backend release:** Money values are fixed two-decimal JSON strings, bill rates are server-owned, and bill/match lifecycle routes changed. The current frontend is incompatible until its separate migration.

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

After the UI loads, use its **Authorize** button with `Bearer <access-token>`
to call endpoints protected by the API's JWT middleware.

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
requires the HttpOnly access cookie, an administrator with `role_id=ADMIN`,
and the existing exact-Origin and double-submit CSRF protections. Frontend
requests must use `credentials: "include"` and send the readable
`csrf_token` cookie value in `X-CSRF-Token` for mutations. Bearer-only
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
