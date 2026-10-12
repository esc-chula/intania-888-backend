# Rate limiting

Every `/api/v1` request that reaches the shared API limiter, including external
requests, uses a provisional 60,000-request/minute/client-IP guard with a burst
capacity of 2,000. Public reads share that IP budget. Protected browser routes
also have a 300-request/minute/account budget, so sessions belonging to the same
account share a quota and different accounts on campus Wi-Fi have separate quotas.
The shared guard follows the Origin and CORS middleware; requests rejected there
and preflight responses do not reach it. Health, readiness, and Swagger routes
outside `/api/v1` do not use these budgets.

## Policies

| Configuration name | Identity and scope | Refill tokens/minute | Burst capacity |
| --- | --- | ---: | ---: |
| `SHARED` | Client IP, across API routes reaching the guard | 60,000 | 2,000 |
| `BROWSER` | Verified account, protected browser routes | 300 | 20 |
| `LOGIN` | Client IP, shared `/auth/login` and `/auth/authorize` | 1,200 | 100 |
| `CALLBACK` | Client IP, `/auth/callback` independently | 1,200 | 100 |
| `TOKEN` | Authenticated application, `/auth/token` | 2,000 | 200 |
| `REVOKE` | Authenticated application, `/auth/revoke` independently | 2,000 | 200 |
| `INVALID_CLIENT` | Client IP, invalid Basic credentials across token/revoke | 30 | 5 |
| `INVALID_EXTERNAL` | Client IP, invalid/missing bearer credentials or rejected grants | 120 | 10 |
| `EXTERNAL_CLIENT` | Signed delegated client ID, all external endpoints | 10,000 | 300 |
| `EXTERNAL_PROFILE` | Active-grant account, `/external/me` across clients | 60 | 5 |
| `EXTERNAL_TRANSACTION` | Active-grant account, `/external/deduct-coin` across clients | 10 | 3 |

Each identity has one token bucket, implemented by `golang.org/x/time/rate`.
The bucket starts with `BURST` tokens, consumes one token per admitted check, and
refills at `PER_MINUTE / 60` tokens per second up to its burst capacity. Rejected
checks consume no token and do not extend the cooldown. A check admitted by an
outer policy still consumes its token if authentication or an inner policy later
rejects the request.

`PER_MINUTE` is a sustained refill rate, not a strict count in a minute window.
For example, the transaction policy admits three immediate calls, then one every
six seconds. Over any interval of `T` seconds, one bucket admits at most
`BURST + PER_MINUTE * T / 60` calls. This replaces the former weighted-window
attempt counter. Idle, fully refilled buckets are removed after two minutes of
inactivity; cleanup scans run at most once per minute on subsequent requests.

**All counters are in memory, per API process.** Restarts reset allowances and
multiple Cloud Run instances have independent allowances. These policies do not
establish a deployment-wide quota or a verified capacity envelope.

## Identity and authentication order

Browser account checks run after the opaque session has been verified, before
profile and blacklist queries. Rotating browser sessions cannot reset the account
budget. Public reads remain unauthenticated and use the shared IP guard.

External requests first pass the shared IP guard. Missing or invalid signed
credentials consume the invalid-external budget and are rejected. The scope
middleware verifies the signature once, selects the client budget, then verifies the configured application, active grant, ownership, and scope
before consuming an account quota. It then loads the account and checks current
admission and blacklist policies before calling the resource handler. A revoked
grant cannot exhaust an account's endpoint budget; an invalid scope remains `403`.
Account-policy denial and storage failures keep their existing authentication or
dependency responses. The verified claims are passed directly to grant validation.

Token and revoke handlers validate Basic credentials against the application
registry before selecting an application budget, then perform protocol/storage
work. Invalid Basic credentials consume an IP failure budget, never the claimed
application's quota. Valid credentials bypass that failure budget. Callback and
revocation budgets are separate from initiation and token exchange respectively.
The existing five-pending-login-transactions guard still applies.

## Configuration and client IP

Set `RATE_LIMIT_<NAME>_PER_MINUTE` and `RATE_LIMIT_<NAME>_BURST` using the names
above. The defaults appear in `.env.example`. Overrides work in `.env` and in the
process environment; environment values take precedence. Zero and negative values
fail startup validation.

`SERVER_CLIENT_IP_MODE=direct` is the default. It uses the TCP peer and ignores
`X-Forwarded-For`, `X-Real-IP`, and other client-supplied forwarding headers.

`SERVER_CLIENT_IP_MODE=cloud_run` requires Cloud Run's `K_SERVICE` environment.
It assumes the direct Cloud Run ingress appends the actual client address as the
rightmost `X-Forwarded-For` value. Client-supplied prefixes are ignored. Missing,
malformed, or scoped IPv6 values fall back to the TCP peer. IPv4-mapped addresses
are normalized to IPv4 and IPv6 keys are canonicalized. This mode is for the
agreed direct Cloud Run request path; additional proxy layers require a new trust
policy. Merely selecting Fiber's first forwarding-header address is unsafe.

## Rejections and retries

Budget rejections return the existing `429 TOO_MANY_REQUESTS` API envelope,
including on OAuth routes. `Retry-After` is a positive integer number of seconds,
rounded upward. It describes the time until the rejecting bucket has one token; concurrent or
continued traffic can exhaust the budget again. Clients should back off and avoid
blindly replaying mutations. Headers describe the last checked bucket: `X-RateLimit-Limit` is its burst
capacity, `X-RateLimit-Remaining` is the number of whole tokens available after
the check, and `X-RateLimit-Reset` is seconds until the bucket is full, rounded
upward. The reset value is not the delay before a single retry.

CORS still exposes only `Link` and `X-Request-ID`; exposing retry headers to browser
JavaScript remains separate work. Rejections produce `http.rate_limit.rejected`
logs with policy, route pattern, and request ID. Credentials, account IDs, and forwarding
header contents are excluded from those structured events.

## Rollout checks

1. Keep `SERVER_CLIENT_IP_MODE=direct` until the direct Cloud Run ingress assumption
   is verified in an isolated staging revision. Compare resolved client addresses
   with Cloud Run request-log client addresses for both ordinary requests and
   requests containing forged `X-Forwarded-For` prefixes. Use temporary private
   instrumentation; remove it before release and never log credentials or raw
   forwarding headers. If the expected appended address is absent or differs,
   stop activation and investigate the ingress path.
2. Exercise the configured policies on an isolated staging stack with representative
   shared-network traffic. The 60,000/minute guard assumes up to 1,000 campus users
   and is provisional, not a measured server-capacity claim. Verify database
   saturation, request latency, and 429 counts per policy before rollout.
3. Enable `cloud_run` only after those checks pass. No production deployment or
   Cloud Run environment change is part of the local implementation. Monitor
   rejection policies and latency after activation; return to the previous revision
   if verification or capacity assumptions fail.
