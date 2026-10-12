# Coding standard

This backend keeps each feature in one Go package. Files distinguish business
rules, HTTP contracts and persistence implementations; import rules protect those
boundaries. The standard applies to authored production code. The API contract
is manually maintained in `docs/openapi.yaml`; update it when routes, wire types,
or security requirements change. Tests follow the same naming and formatting
rules.

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

## Go formatting and spacing

These rules apply to all handwritten Go code, including unit/integration tests,
fakes, fixtures, callbacks, and small helpers. Apply the layout deliberately,
then run `gofmt` on the edited files. `gofmt` handles tabs, alignment, and syntax
spacing; it does not choose logical sections or expand every dense expression.

### Indentation and blank lines

- Use `gofmt` tabs for Go indentation. Do not replace indentation tabs with
  spaces or manually pad fields for alignment. Let `gofmt` align them.
- Use exactly one empty line between consecutive functions and methods. A doc
  comment belongs immediately above the declaration it describes.
- Inside a function, separate logical steps with one empty line. Typical steps
  are preparing input, performing an operation, validating its result, and
  producing the output. Do not put an empty line between every statement.
- Keep a call and its immediate error check adjacent. Keep the statements that
  prepare one operation together when they form a single step.
- Do not add empty lines immediately after an opening brace or before a closing
  brace. Do not use multiple consecutive empty lines.
- Keep rationale comments with the code they explain. When separating sections,
  put the empty line before the comment, not between the comment and its code.

For example, this helper has separate setup, file-writing, and loading steps.
The write and its error check remain adjacent:

```go
load := func(contents string) error {
	t.Helper()

	path := filepath.Join(t.TempDir(), "auth.yaml")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAuthRegistry(
		path,
		"development",
		"http://localhost:3000",
		os.Getenv,
	)

	return err
}
```

### Blank line before return

Insert exactly one empty line before a `return` when another statement precedes
it **in the same block**. This applies to functions, `if`/`else` blocks, loops,
callbacks, and switch/select case bodies. An existing empty line is sufficient;
do not add another one.

**Exception:** when `return` is the block's only statement, do not insert an
empty line. Count statements inside that block, not earlier statements in the
surrounding function. A comment alone is not a preceding statement.

A return-only function stays compact vertically, with a multiline body:

```go
func (s fakeAuthService) Logout(context.Context, string) error {
	return s.logoutErr
}
```

A guard containing only a return has no empty line inside it. The function's
final return follows other statements, so it gets one:

```go
func decodePath(path string) (string, error) {
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return "", err
	}

	return decoded, nil
}
```

When a guard performs work before returning, separate that work from the return:

```go
if err != nil {
	h.clearCookie(c, cookieName, true)

	return err
}
```

The same exception applies to `return SomeType{...}` as a function's only
statement: no empty line before that return, even if its literal spans many lines.

### Function bodies, literals, and calls

Write function and callback bodies on multiple lines, including trivial test
stubs. Do not put the signature, statements, and closing brace on one line.
Separate statements with newlines, not semicolons. Normal Go constructs such as
`if err := operation(); err != nil` are allowed.

For keyed struct and map literals with two or more fields/entries, use one
field/entry per line, even when the entire literal would fit on one line. This
includes two-entry `url.Values` maps in tests. Expand enclosing literals when
needed to show nested levels. Preserve field order and use trailing commas.
Do not add blank lines between every field. Short, simple literals such as
`config.DB{}` or `config.Server{Env: env}` can remain on one line.

```go
func newHTTPConfig(env string) config.Config {
	return authTestConfig{
		server: config.Server{
			Env: env,
		},
		oauth: config.OAuth{
			Registry: &config.AuthRegistry{
				Lifetimes: config.AuthLifetimes{
					Login: 600,
				},
			},
		},
	}
}
```

Expand calls when a long argument list or nested expressions are difficult to
scan. Once a call is expanded, place one argument per line and use a trailing
comma. Keep short, clear calls on one line; there is no mandatory column limit.
Do not extract new variables or helpers solely to shorten a line during a
formatting-only change.

```go
response := call(
	"POST",
	"/api/v1/auth/token",
	exchange.Encode(),
	"",
	"",
)
if response.StatusCode != fiber.StatusBadRequest {
	t.Fatal("wrong verifier accepted")
}
```

For a long condition, break after `||` or `&&` and indent continuation lines.
Preserve operand order and short-circuit behavior. Do not split one assertion
into multiple assertions as part of formatting; that can change execution and
failure behavior.

```go
if err := json.NewDecoder(response.Body).Decode(&tokens); err != nil ||
	response.StatusCode != fiber.StatusOK ||
	tokens.UserID != "integration-player" {
	t.Fatalf("exchange: %d, %v", response.StatusCode, err)
}
```

### Test layout

Tests follow the same rules as production code. Separate fixture preparation,
request execution, and result inspection when they are distinct steps. Keep an
operation and its immediate failure check together. Separate successive test
scenarios, such as an invalid verifier followed by a valid exchange. Do not add
comments saying only "Arrange", "Act", or "Assert" when the code is clear.

```go
code := authorize()
exchange := url.Values{
	"grant_type":    {"authorization_code"},
	"code":          {code},
	"redirect_uri":  {"http://localhost:8081/auth/callback"},
	"code_verifier": {strings.Repeat("x", 43)},
}

if response := call(
	"POST",
	"/api/v1/auth/token",
	exchange.Encode(),
	"",
	"",
); response.StatusCode != fiber.StatusBadRequest {
	t.Fatal("wrong verifier accepted")
}

exchange.Set("code_verifier", verifier)
response := call("POST", "/api/v1/auth/token", exchange.Encode(), "", "")

var tokens applicationTokens
if err := json.NewDecoder(response.Body).Decode(&tokens); err != nil ||
	response.StatusCode != fiber.StatusOK ||
	tokens.UserID != "integration-player" {
	t.Fatalf("exchange: %d, %v", response.StatusCode, err)
}
```

Apply these boundaries explicitly when formatting a test:

- Put one empty line after a fixture map/struct is complete and before the
  request that uses it. Do not separate statements building that same fixture.
- Put one empty line after a request/assertion block and before the next
  independent request or scenario. This includes an invalid verifier followed
  by changing it to a valid verifier, replay rejection followed by profile
  access, and profile access followed by a scope check.
- In request helpers and goroutine callbacks, keep request construction and
  header/cookie/client-auth configuration together. Put one empty line after
  that preparation and before `app.Test(request)` (or the equivalent send).
  The send and its immediate error check remain adjacent.
- Put one empty line after initial callback bookkeeping such as `t.Helper()`
  or `defer wait.Done()` before beginning request preparation. Put one empty
  line after a successful send/error-check block before registering body cleanup
  and reading or recording the response. Keep body cleanup and response reads
  together when they are one result-handling step.
- Keep request execution with its immediate status/error check. If the next step
  declares a response payload and decodes it, separate that decoding step from
  execution with one empty line. Keep the payload declaration and decode/check
  together.
- Separate a local helper function definition from its first invocation with
  one empty line. Separate a completed fixture literal from a following helper
  definition in the same way. A blank line between only top-level functions
  does not satisfy this rule for local helpers.
- Review the entire file, including the later refresh/replay/concurrency phases.
  Applying the example to one section is not a complete formatting pass.

A short two-entry map still follows the entry-per-line rule:

```go
refresh := url.Values{
	"grant_type":    {"refresh_token"},
	"refresh_token": {tokens.RefreshToken},
}

refreshed := call("POST", "/api/v1/auth/token", refresh.Encode(), "", "")

var renewed applicationTokens
if err := json.NewDecoder(refreshed.Body).Decode(&renewed); err != nil ||
	refreshed.StatusCode != fiber.StatusOK ||
	renewed.RefreshToken == tokens.RefreshToken {
	t.Fatal("refresh did not rotate")
}
```

For table-driven tests, expand dense case records and nested expected values.
Keep each case's related fields together. Preserve case order, subtest names,
assertions, cleanup, and concurrency settings.

### Formatting-only workflow and review checklist

1. Inspect the current diff and identify the files in scope. Preserve unrelated
   worktree and staged changes.
2. Apply logical spacing and expand dense code manually. Preserve names, values,
   comments, control flow, evaluation order, errors, and API behavior.
3. Run `gofmt` on the edited Go files. Formatting checks passing does not replace
   the manual layout review.
4. Review the diff for the rules below. Run `git diff --check`; if changes are
   staged, also run `git diff --cached --check`.
5. Run tests when the task requires or authorizes them; report the checks actually
   performed. Do not claim behavioral verification from formatting alone.

Checklist:

- [ ] Functions and callbacks have multiline bodies and are separated clearly.
- [ ] Logical sections have one empty line; related operations/checks stay adjacent.
- [ ] Each return follows the same-block rule and its return-only exception.
- [ ] Dense literals/calls are expanded; fields/arguments are easy to scan.
- [ ] Long conditions preserve operand order and short-circuit behavior.
- [ ] Go indentation/alignment comes from `gofmt`, not manually inserted spaces.
- [ ] Existing comments remain attached to their code.
- [ ] No behavior, test expectations, or unrelated files changed.

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
