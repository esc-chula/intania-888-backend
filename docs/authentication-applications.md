# Application authentication

## Before and after

The previous browser flow fetched a Google login URL and returned to one global frontend destination. Login now begins through browser navigation with a registered `client_id`. Callback and post-login destinations come from an operator-managed YAML registry.

888 retains its own opaque browser session. Games receives an authorization code through its backend callback, exchanges it with 888, and issues its own browser session. The 888 frontend is not part of the Games redirect path. Google always calls the 888 backend callback.

## Configuration

Set `AUTH_CONFIG_FILE` to the registry path. Use `config/auth.development.yaml` for local registrations and `config/auth.production.example.yaml` as a production template. Set the upstream `OAUTH_CLIENT_ID`, `OAUTH_CLIENT_SECRET`, and `JWT_ACCESS_TOKEN_SECRET` through the environment or the environment-specific .env file.

Cookie applications define an ID, exact frontend origin, default return path, onboarding path, and login error path. Code applications define an ID, exact backend callback URLs, allowed scopes, and the environment-variable name holding their client secret. Set that secret on both backends. Do not put secret values in YAML.

The registry controls login transaction, authorization code, access token, and delegation idle/absolute lifetimes in seconds. 888 browser session lifetimes remain in `SESSION_IDLE_TTL_SECONDS` and `SESSION_ABSOLUTE_TTL_SECONDS`; these sessions and delegations have separate lifecycles.

Production destinations require HTTPS. Local development supports localhost HTTP. Callback URLs must match a registered URL exactly. Register the 888 frontend origin in `CORS_ALLOW_ORIGINS`; server-to-server token exchange does not require a browser Origin.

## Direct browser login

Navigate to `/api/v1/auth/login?client_id=intania-888-web&return_to=%2F`. The ID in this example belongs to the development registry. `return_to` must be a safe relative path on the registered frontend origin; omitted values use the configured default.

888 reuses an existing admitted session when possible. Otherwise it creates a one-use Google state and PKCE verifier, binds the transaction to a separate HttpOnly cookie, and redirects to Google. The callback consumes the bound transaction, checks identity and access policy, and sets the 888 session cookie. New users are sent to onboarding with the validated original path in `return_to`.

Protected browser requests use credentials and a CSRF token obtained from `/auth/me`. Hold CSRF in memory. Never expose session IDs or bearer credentials to frontend JavaScript.

## Backend delegation

1. The game backend generates random state, a PKCE verifier, and a browser binding. It stores the intended game return path locally.
2. Navigate to `/api/v1/auth/authorize` with `client_id`, an exact registered `redirect_uri`, `response_type=code`, `state`, `scope`, `code_challenge`, and `code_challenge_method=S256`.
3. 888 reuses its session or runs Google login. The browser returns to the registered game backend callback with a short-lived code and the original state.
4. The game backend checks and consumes its browser-bound state. It sends a form-encoded `POST /api/v1/auth/token` with HTTP Basic client authentication, `grant_type=authorization_code`, `code`, `redirect_uri`, and `code_verifier`.
5. 888 consumes the code once and returns `access_token`, `refresh_token`, `token_type`, `expires_in`, `scope`, and `user_id`. The game stores credentials server-side and issues its own opaque session cookie.
6. The backend refreshes with `grant_type=refresh_token` and `refresh_token`, using the same client authentication. Refresh rotates the credential. Reusing a consumed credential revokes its delegation.
7. Logout revokes the game's delegation with form-encoded `POST /api/v1/auth/revoke`, client authentication, and `token`. Revoking a delegation does not revoke the independent 888 session.

`profile.read` authorizes `/external/me`. `coins.spend` authorizes `/external/deduct-coin`. Delegated credentials must satisfy their current client registration, grant, token scopes, and account policy. Cookies cannot authenticate token exchange or revocation.

## Legacy-token retirement

Unscoped administrator-issued tokens are disabled by default. Their issuer returns `410 LEGACY_AUTH_RETIRED`; bearer authentication returns `401`. Administrator revocation remains available.

To migrate an existing consumer, set an explicit cutoff:

```yaml
legacy_external_tokens:
  accept_until: "2026-10-15T00:00:00Z"
```

This is an example, not a recommended deadline. Choose a deployment-specific cutoff. Before it, administrator-issued tokens remain usable without application scope binding. Their expiry and Redis TTL use the configured access-token lifetime, capped by time remaining until the cutoff. After it, all legacy tokens are rejected even if a Redis record remains. An empty cutoff disables compatibility immediately.

Register each consumer, deploy code exchange and backend credential storage, verify its required scopes, then leave the cutoff empty or allow it to expire. Do not extend the window as a substitute for migration.

## Deployment checklist

- Configure upstream Google credentials and the 888 callback in the Google registration.
- Register each downstream application and provision matching backend client secrets.
- Set registry paths, exact browser origins, and production HTTPS destinations.
- Deploy the game session storage and its callback before switching its login link.
- Replace JSON login fetching with browser navigation and remove browser token storage.
- Decide whether an existing legacy consumer needs a bounded migration window.

Authentication changes do not resolve cross-database coin-spending consistency. That recovery work remains a separate phase.
