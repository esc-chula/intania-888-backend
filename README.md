# Intania 888 Backend

Go and Fiber API for Intania 888. PostgreSQL stores application data, Redis
stores browser sessions and cache state, and Goose applies versioned SQL
migrations.

> **Breaking API contract:** exact decimals are JSON strings: two fractional
> digits for money and six for odds/payout multipliers. Protected browser
> operations use HttpOnly cookies with CSRF protection; shared catalogue,
> fixture, and standings reads are public. Deploy with a compatible frontend; see
> the [API migration guide](docs/api-migration-from-main.md) and
> [exact-decimal frontend migration](docs/api-migration-from-main.md#exact-decimal-frontend-migration).

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

   The default 888 frontend origin is `http://localhost:3000`. Keep it in
   `CORS_ALLOW_ORIGINS` and in the `intania-888-web` registration in
   `config/auth.development.yaml`. The Swagger login registration uses
   `http://localhost:8080`; keep that origin in `CORS_ALLOW_ORIGINS` too. Set
   `AUTH_CONFIG_FILE` to that registry.
   Google uses `google.callback_uri`; each application has its own frontend
   or backend callback registration. Set `INTANIA_GAMES_CLIENT_SECRET` to a
   random secret of at least 32 characters when using the Games registration,
   and provision the same secret on the Games backend. The example secrets
   are placeholders, not production credentials.

3. Start the API and its dependencies:

   ```sh
   make dev
   ```

   `make dev` starts PostgreSQL and Redis with Compose, applies pending Goose
   migrations, then runs the API through Air. The manually maintained API
   contract is in `docs/openapi.yaml`; validate it with `make openapi-check`.
   The API listens on
   `http://localhost:8080`. The Compose file starts only PostgreSQL and Redis;
   the Go process runs on the host. Accordingly, `.env.example` uses
   `DB_HOST=localhost`. If you run the API inside the same Compose network, use
   `DB_HOST=postgres` and `CACHE_HOST=redis` instead.

4. For a fresh database, add the default sports, locations, groups, and colors
   from a second terminal:

   ```sh
   make seed
   ```

To run without Air, start the dependencies and apply migrations first, then run
the API directly:

```sh
make deps
make migrate-up
make run
```

The example Compose setup publishes PostgreSQL on port `5432` and Redis on
`6379`. Edit `docker-compose.yml` if those host ports are already in use.

Login starts through browser navigation to:

```text
http://localhost:8080/api/v1/auth/login?client_id=intania-888-web&return_to=%2F
```

For the local Games integration, the frontend is `http://localhost:3001` and
its backend callback is `http://localhost:8081/auth/callback`. Google still
returns to 888 on port 8080. The Games repository provides its own root Compose
setup; its browser talks to its backend, which exchanges credentials with 888.

## Useful commands

| Command | Purpose |
| --- | --- |
| `make dev` | Start dependencies, migrate the database, and run Air |
| `make run` | Run the API directly with `go run` (Air is not required) |
| `make deps` | Start PostgreSQL and Redis |
| `make migrate-status` | Show the Goose migration status |
| `make migrate-up` | Apply pending migrations |
| `make seed` | Add the stable sport, location, group, and color catalogue rows |
| `make test` | Run Go unit and package tests |
| `make test-race` | Run tests with Go's race detector |
| `make test-integration` | Run integration tests with disposable PostgreSQL and Redis containers |
| `make lint` | Check formatting, run `go vet`, and run pinned golangci-lint |
| `make ci` | Run lint, tests, race tests, build, and OpenAPI contract checks |
| `make openapi-check` | Validate the OpenAPI document with kin-openapi |

Catalogue seeding inserts missing defaults without overwriting edited titles.
Explicitly rerunning `make seed` restores deleted default sport entries. Admins
manage sports through `/api/v1/sport-types/admin`; see the
[API reference](docs/README.md#sport-type-administration) and
[frontend migration guide](docs/api-migration-from-main.md#sport-type-frontend-migration).

## Team coins

Each color earns "team coins" for every decided match it wins by vote: among the
color's members who bet on the match, more must have picked the winner than the
loser (a tie earns nothing, and the margin does not change the award). The fixed
award per won vote is `TEAM_COIN_PER_MATCH_WIN` (a decimal such as `100.00`; the
server refuses to start without it, and the value in `.env.example` is a
placeholder until the organizer sets the real amount). Awards are written to the
append-only `team_coin_events` ledger when a match result is set and corrected
when a bill is voided. The running totals (`team_coin`, `bets_right`,
`bets_wrong`) live on `colors`, like `users.remaining_coin`, and every ledger
insert updates them in the same transaction. `GET
/api/v1/colors/leaderboards/coins` and `/predictions` read those totals.

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
`CORS_ALLOW_ORIGINS`. The `/external/*` API requires registered, scoped delegated Bearer credentials:
`profile.read` for `/external/me` and `coins.spend` for `/external/deduct-coin`.
Obtain credentials through `/auth/authorize` and server-to-server `/auth/token`;
see [application authentication](docs/authentication-applications.md). See
[Swagger and API usage](docs/README.md) for the current contract.

Location, sport, match, and color-standings reads are public and do not evaluate
account allowlist/blacklist rules. Configured-Origin checks and the API-wide
per-IP rate limit still apply; see [public shared reads](docs/README.md#public-shared-reads).

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
| `docs` | API guides and coding standard |

Feature code is organized by domain. Within a feature package, file names
describe responsibility, such as `service.go`, `adapter.http.go`, and
`adapter.db.go`. See the [coding standard](docs/coding-standard.md) before
changing package boundaries or adding shared helpers.

## Further reading

- [API and Swagger guide](docs/README.md)
- [Coding standard](docs/coding-standard.md)
- [API migration guide](docs/api-migration-from-main.md)
- [Application authentication and client registry](docs/authentication-applications.md)
