# Intania 888 Backend

Go and Fiber API for Intania 888. PostgreSQL stores application data, Redis
stores browser sessions and cache state, and Goose applies versioned SQL
migrations.

> **Breaking API contract:** this backend uses fixed two-decimal strings for
> money and cookie-based browser sessions with CSRF protection. Coordinate it
> with a compatible frontend. See the [API migration guide](docs/api-migration-from-main.md).

## Requirements

- Go 1.27.1, as declared in `go.mod`
- Docker Engine with the Docker Compose plugin
- [Air](https://github.com/air-verse/air) on `PATH` to run `make dev`
- The golangci-lint version in `.golangci-version` to run `make lint` or `make ci`
- A Google OAuth client for testing browser sign-in

## Quick start

1. Create a local environment file:

   ```sh
   cp .env.example .env
   ```

2. Replace `OAUTH_CLIENT_ID` and `OAUTH_CLIENT_SECRET` in `.env` with a Google
   OAuth web client. Configure this authorized redirect URI in Google:

   ```text
   http://localhost:8080/api/v1/auth/callback
   ```

   The default frontend origin is `http://localhost:3000`. Keep it in
   `CORS_ALLOW_ORIGINS` and set `OAUTH_POST_LOGIN_REDIRECT_URL` to the frontend
   origin you use. For local development, the example JWT secret is suitable
   only for development.

3. Start the API and its dependencies:

   ```sh
   make dev
   ```

   `make dev` starts PostgreSQL and Redis with Compose, applies pending Goose
   migrations, then runs the API through Air. The API listens on
   `http://localhost:8080`. The Compose file starts only PostgreSQL and Redis;
   the Go process runs on the host. Accordingly, `.env.example` uses
   `DB_HOST=localhost`. If you run the API inside the same Compose network, use
   `DB_HOST=postgres` and `CACHE_HOST=redis` instead.

To run without Air, install the dependencies and migrations first, then start
the Go process directly:

```sh
make deps
make migrate-up
APP_ENV=dev go run ./cmd/main.go
```

The example Compose setup publishes PostgreSQL on port `5432` and Redis on
`6379`. Edit `docker-compose.yml` if those host ports are already in use.

## Useful commands

| Command | Purpose |
| --- | --- |
| `make dev` | Start dependencies, migrate the database, and run Air |
| `make deps` | Start PostgreSQL and Redis |
| `make migrate-status` | Show the Goose migration status |
| `make migrate-up` | Apply pending migrations |
| `make seed` | Add the stable sports, group, and color catalogue rows |
| `make test` | Run Go unit and package tests |
| `make test-race` | Run tests with Go's race detector |
| `make test-integration` | Run integration tests with disposable PostgreSQL and Redis containers |
| `make lint` | Check formatting, run `go vet`, and run pinned golangci-lint |
| `make ci` | Run lint, tests, race tests, build, and generated Swagger checks |
| `make docs` | Regenerate the Swagger files |
| `make docs-check` | Check that generated Swagger files match the source annotations |

Integration tests use ports `55432` and `56379` by default. Override
`TEST_POSTGRES_PORT`, `TEST_REDIS_PORT`, or `TEST_COMPOSE_PROJECT` if those
resources conflict with another local task. These tests reset their dedicated
test database; never point `INTANIA888_TEST_DATABASE_URL` at a development or
production database.

`make migrate-down` and `make migrate-reset` are destructive. They require
`ALLOW_DESTRUCTIVE_MIGRATIONS=I_UNDERSTAND_DATA_WILL_BE_LOST`; use them only
with a disposable local database. `docker compose down` stops local services
and keeps their named data volumes.

## Local endpoints

- API base path: `http://localhost:8080/api/v1`
- Swagger UI: `http://localhost:8080/swagger/index.html` when enabled
- Liveness: `http://localhost:8080/healthz`
- Readiness (checks PostgreSQL and Redis): `http://localhost:8080/readyz`

Browser mutations use the session cookie and `X-CSRF-Token` returned by
`GET /api/v1/auth/me`. Configure the exact frontend origin in
`CORS_ALLOW_ORIGINS`. The `/external/*` API uses Bearer authentication. See
[Swagger and API usage](docs/README.md) for the current contract.

## Repository structure

| Path | Responsibility |
| --- | --- |
| `cmd/main.go` | Application composition and process startup |
| `cmd/migrate` | Goose migration CLI |
| `cmd/seed` | Idempotent catalogue and optional policy seeding |
| `internal/domain/<feature>` | Feature services, ports, HTTP handlers, DTOs, and persistence adapters |
| `internal/identity` | Transport-independent account and actor snapshots |
| `internal/httpidentity` | Authenticated Fiber identity and shared profile response |
| `internal/apierror` | Shared HTTP error response, request IDs, and strict JSON decoding |
| `internal/security` | Cookie, session, token, and policy security primitives |
| `internal/persistence/model` | GORM database records |
| `internal/integration` | PostgreSQL/Redis-backed acceptance tests (`integration` build tag) |
| `internal/testutil` | Disposable integration database helpers |
| `pkg` | Shared infrastructure packages: config, database, cache, OAuth, and logging |
| `migrations` | Versioned PostgreSQL SQL migrations |
| `docs` | API guides, coding standard, and architecture decisions |

Feature code is organized by domain. Within a feature package, file names
describe responsibility, such as `service.go`, `adapter.http.go`, and
`adapter.db.go`. See the [coding standard](docs/coding-standard.md) before
changing package boundaries or adding shared helpers.

## Further reading

- [API and Swagger guide](docs/README.md)
- [Coding standard](docs/coding-standard.md)
- [API migration guide](docs/api-migration-from-main.md)
