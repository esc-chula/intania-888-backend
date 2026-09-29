# Coding standard

This backend keeps each feature in one Go package. Files distinguish business
rules, HTTP contracts and persistence implementations; import rules protect those
boundaries. The standard applies to authored production code. Generated Swagger
files are regenerated, and tests follow the same naming and formatting rules.

## Sources and precedence

Use [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and
[Go doc comments](https://go.dev/doc/comment) for language conventions.
[Google's Go style guide](https://google.github.io/styleguide/go/guide) and
[Uber's Go guide](https://github.com/uber-go/guide/blob/master/style.md) provide
additional readability and error-handling guidance. Feature file placement is
our convention, informed by [ports and adapters](https://alistair.cockburn.us/hexagonal-architecture).
These sources do not prescribe a universal Go backend folder tree.

## Ownership and files

| File | Responsibility |
| --- | --- |
| `doc.go` | Package purpose, responsibilities and important guarantees |
| `service.go`, `service.settlement.go` | Business rules and orchestration |
| `types.go` | Transport-independent inputs, results, filters and snapshots |
| `port.go` | Small dependency interfaces defined by their consumer |
| `errors.go` | Feature errors and their contracts |
| `adapter.http.go`, `adapter.http.dto.go` | Routes, handlers and JSON request/response types |
| `adapter.http.mapping.go`, `adapter.http.errors.go` | Service-to-HTTP conversion and error classification |
| `adapter.db.go`, `adapter.db.mapping.go` | Queries, ORM records and snapshot conversion |
| `adapter.cache.go` | Cache serialization and cache error translation |

Keep established role-first, dot-separated filenames. Go treats these as ordinary
files in the same package. Create a file only when it contains a distinct
responsibility. Split by a named use case when it helps navigation; do not impose line-count limits. Helpers stay
with their feature or a purpose-named shared package. Do not add generic `utils`
or `common` collections.

`internal/persistence/model` owns interconnected GORM records. `internal/value`
owns checked money/rate arithmetic and wire encoding. `internal/identity` owns
neutral account and actor snapshots shared by authentication and account use
cases. `internal/httpidentity` owns HTTP identity access and the shared public
profile shape; it is an HTTP boundary, not a service dependency.

## Types and APIs

HTTP requests and responses are distinct from service inputs and results. Core
feature types contain no JSON, validation or ORM tags. Shared values can implement
JSON encoding when that encoding is their established contract. Persistence
ports return neutral snapshots rather than GORM records. Cache adapters own
serialized records; cache ports expose typed operations.

Use Go initialisms consistently: `ID`, `IDs`, `HTTP`, `URL`, `JWT`, `DTO`, `DB`,
`GORM` and `CORS`. Prefer `bill.Service`, `NewService`, `NewHTTPHandler`, and
`NewGORMRepository` over redundant feature prefixes or `Impl` suffixes. Export
cross-package APIs only. Constructors return concrete implementations and accept
explicit required dependencies. Document any optional dependency and its nil
behavior in the constructor comment. Route registration installs routes; it must not
initialize operational dependencies.

Pass `context.Context` first on request-driven I/O operations, through services,
repositories, GORM `WithContext`, Redis and OAuth. Dependency timeouts derive from
the supplied context. Keep background contexts at executable/background-work
entrypoints and tests. Fiber's user context does not automatically provide
client-disconnect cancellation; no request-wide deadline is added by this refactor.

## Handlers, services and transactions

Handlers decode and validate transport input, obtain the authenticated actor,
map to service input, invoke a use case and map the result. Services enforce
business invariants for all callers, including callers outside HTTP. Repositories
execute queries, conditional writes, locks and persistence serialization.

Features requiring atomicity define `WithinTransaction(ctx, callback)` and a
narrow transaction repository. The service owns the business decisions; the
adapter owns begin/commit/rollback. Every operation inside the callback uses its
transaction-bound repository, including reads. Do not call a service that opens
an independent transaction from inside a callback. Transaction repositories must
not escape their callback.

Preserve advisory locks, deterministic row-lock order, guarded balance updates
and idempotency. Return commit failures before reporting success. Log committed
business events and perform external side effects after commit. Add no automatic
transaction retries without separately accounting for randomness and side effects.
See [Go transaction guidance](https://go.dev/doc/database/execute-transactions).

## Documentation, errors and logging

Every authored production package has a package comment. Every exported type,
function, method, variable/constant group and public interface method has a useful
comment beginning with its name (or explaining the declaration group). This
includes exported methods on private receivers. Explain observable purpose,
meaningful errors, side effects, input mutation, rounding and concurrency/locking
guarantees where relevant. Private implementation comments may refer to the
interface contract. Document non-obvious fields; avoid repeating field names.

Keep existing rationale comments and their placement. Use guard clauses, logical
blank lines and descriptive names; keep immediate error checks beside the call.
Expand long literals/calls for scanning. Comments explain contracts or rationale,
not each obvious statement. Avoid arbitrary length and comment-count quotas.

Define feature errors in `errors.go`. Repositories translate driver errors while
preserving causes for `errors.Is`/`errors.As`. HTTP error mapping knows feature
errors, never GORM/Redis/pgx failures. Preserve existing public codes and messages;
never classify by error text. All HTTP failures use `internal/apierror` for the
shared renderer, request ID, safe response and strict JSON binding.

Log propagated request failures once at the HTTP boundary with request ID and
cause. Services may log business events or failures they intentionally recover
from. Never log tokens, credentials or complete request bodies. Handle returned
errors; retained behavior exceptions require a narrowly scoped suppression with
a reason.

Keep a concise Go comment above each exported route handler describing its purpose.
Maintain request, response, security, and schema details in `docs/openapi.yaml`,
and keep its route inventory aligned with registered handlers.

## Checks and review

The linter version is pinned in `.golangci-version`; `make` rejects other versions.
Use `make fmt`, then `make ci`. The explicit configuration enables documentation,
naming, dependency boundaries, error handling, static analysis and justified
suppressions. Review comment usefulness, port granularity, transaction semantics
and wire compatibility manually; linters cannot prove them.

Unit tests need no external services. Test observable results, error identity,
validation, context propagation and business invariants. Use the real shared
error handler in HTTP tests with fake services. Integration tests use a disposable
PostgreSQL database whose name contains `test` and an isolated Redis service.
`make test-integration` starts and removes those dependencies; its prerequisite
check rejects missing test configuration before execution. An integration skip
is not evidence of runtime correctness.

PR quality checks run the same local commands and a separate dependency-backed
integration job. This normalization changes no deployment configuration, schema,
API routes, business outcomes or gameplay odds.
