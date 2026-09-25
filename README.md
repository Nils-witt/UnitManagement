# go-unit-mangement

A Go web server (net/http + GORM + PostgreSQL) with JWT-based login (`Authorization: Bearer` tokens backed by revocable server-side sessions). The UI in `frontend/` (Vite + React, MUI, React Router, TanStack Query, i18next with English and German) follows the structure of [Tileserve-GO](https://github.com/Nils-witt/Tileserve-GO)'s frontend and is built and embedded into the Go binary.

## Quick start

```sh
make db                                   # start Postgres via docker compose
ADMIN_PASSWORD=change-me make run         # build UI + server, run on :8080
```

On startup, if no account is an administrator (on first start, or after SSO group sync revoked the role from everyone), a local administrator is created from `ADMIN_USERNAME` (default `admin`) and `ADMIN_PASSWORD`. Existing accounts are never promoted: if `ADMIN_USERNAME` is taken, the server logs a warning and you need to pick an unused name. With SSO configured, `ADMIN_PASSWORD` is optional: without it no local account is created, and administrators come from `OIDC_ADMIN_GROUP` (see below); if that isn't set either, the server logs a warning that nobody can manage users. Without SSO, startup fails if there is no administrator and no `ADMIN_PASSWORD`.

## Users and SSO

Administrators manage accounts on the **Users** page: create local accounts (passwords of at least 8 characters), grant or revoke the administrator role, reset passwords and delete accounts. Resetting a password or deleting an account ends that user's sessions. Administrators can't remove their own role or delete their own account.

SSO uses OpenID Connect (authorization code flow with PKCE) and works with any compliant provider (Keycloak, Authentik, Entra ID, Google, Dex, …). Set `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` and `OIDC_REDIRECT_URL`, and register the redirect URL (`https://<host>/api/auth/oidc/callback`) at the provider. The login page then shows a "Sign in with `OIDC_DISPLAY_NAME`" button.

The first SSO sign-in creates an account linked to the provider's subject (`sub` claim), named after the `preferred_username` claim (or the email, or the subject). New SSO accounts are not administrators and have no password; an administrator can promote them or set a password to also allow password sign-in. Anyone who can sign in at the provider gets an account, so restrict access to the client at the provider if needed.

Groups are synced from the provider: every SSO sign-in reads the user's groups from the `OIDC_GROUPS_CLAIM` claim (default `groups`; from the ID token, or from the userinfo endpoint if the ID token lacks it), creates groups seen for the first time and makes the user a member of exactly those groups. A user without the claim ends up in no groups. Groups are kept after their last member leaves. Administrators see them on the read-only **Groups** page and as chips on the **Users** page.

To manage administrators at the provider too, set `OIDC_ADMIN_GROUP` to a group name: members of that group get the administrator role at sign-in, and everyone else loses it. The Users page then locks the role of SSO accounts; local accounts are unaffected. Group changes at the provider take effect at the user's next sign-in, not in running sessions. Some providers only send groups when asked for them: set `OIDC_EXTRA_SCOPES` (e.g. `groups` for Dex), or add a groups mapper to the client (Keycloak).

### Using OIDC access tokens with the API

Set `OIDC_ACCESS_TOKEN_AUDIENCE` to let API clients send a JWT access token from the provider directly as `Authorization: Bearer <access token>` (or as the WebSocket subprotocol token), without signing in through the browser first. It takes a comma-separated list; a token is accepted if its `aud` contains at least one of them — e.g. the client id, or a dedicated API audience configured at the provider (for Keycloak, add an "Audience" mapper to the client scope).

Tokens are trusted from the SSO provider above (if configured) and from every issuer listed in `OIDC_ACCESS_TOKEN_ISSUERS` (comma-separated issuer URLs, each discovered at startup via its `/.well-known/openid-configuration`). Additional issuers work without SSO sign-in being configured at all. A token must name one of these issuers in `iss`, be signed with one of that issuer's published keys (its JWKS), and be unexpired; ID tokens are rejected, and so are opaque (non-JWT) access tokens.

The token's subject resolves to an account the same way an SSO sign-in does, creating one on first use. Accounts are keyed by `sub` alone, not by issuer, so the same subject arriving via the SSO provider or any additional issuer is one account. **Only trust issuers that share one subject namespace** (e.g. the same identity provider reachable under several issuer URLs): any trusted issuer can act as any account whose `sub` it can put into a token. If several accounts are already linked to the same subject, the oldest one is used. Groups (and, with `OIDC_ADMIN_GROUP`, the administrator role) are synced from the token's `OIDC_GROUPS_CLAIM` claim only if the access token carries it. A verified token is reused for up to a minute before it is checked again.

## Development

Run the Go server (`go run .`) and, in another terminal, `make dev-frontend`. Vite serves the UI with hot reload on http://localhost:5173 and proxies `/api` to `:8080`.

Format and lint the UI with `npm run format` and `npm run lint` in `frontend/`.

`npm install` in `frontend/` sets up a [Husky](https://typicode.github.io/husky/) pre-commit hook (`frontend/.husky/pre-commit`) that runs `oxlint`, `golangci-lint run` and `govulncheck`. Install the Go tools first (`brew install golangci-lint govulncheck`, or `go install golang.org/x/vuln/cmd/govulncheck@latest`). Skip the hook with `git commit --no-verify`.

`go build` embeds whatever is in `frontend/dist`, so run `npm run build` in `frontend/` before building the binary (`make build` does this).

## CI

`.github/workflows/ci.yml` runs on pushes to `main` and on pull requests: `go vet`, `go test -race` and a `go mod tidy` check, `golangci-lint`, `govulncheck`, the UI's format check, lint and build, and a Docker image build (not pushed). Dependabot (`.github/dependabot.yml`) opens weekly updates for Go modules, npm packages, GitHub Actions and the Docker base images.

## Releasing

Releases are built with [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`). Pushing a `v*` tag runs `.github/workflows/release.yml`, which builds the UI, cross-compiles the server for Linux, macOS and Windows (amd64 and arm64), and publishes the archives to a GitHub release. The version is stamped into the binary and logged at startup.

Try a local build without publishing with `goreleaser release --snapshot --clean` (output in `dist/`).

## Configuration

See `.env.example`. Set `JWT_SECRET` to a random string of at least 32 bytes (e.g. `openssl rand -base64 48`) so sign-ins survive restarts; without it the server signs tokens with a random key per start. Set `COOKIE_SECURE=true` when serving over HTTPS (it guards the short-lived SSO sign-in cookie).

Set `INSTANCE_NAME` to label the deployment (e.g. `Kreis Nord`); it replaces the product name in the page title, header and login page.

Behind a reverse proxy, set `TRUSTED_PROXIES` to its addresses (comma-separated IPs or CIDRs, e.g. `10.0.0.0/8,::1`) so logs show the real client IP. For requests from those addresses the client is taken from `X-Forwarded-For` (the rightmost entry that isn't a trusted proxy) or `X-Real-IP`; the headers are ignored from anyone else, so they can't be spoofed.

## API

The full API is described in [`api/openapi.yaml`](api/openapi.yaml) (OpenAPI 3.1).

| Method | Path               | Description                          |
|--------|--------------------|--------------------------------------|
| POST   | `/api/auth/login`  | `{username, password}` → `{token, tokenType, expiresAt, user}` |
| POST   | `/api/auth/logout` | Ends the session                     |
| GET    | `/api/auth/me`     | Current user (401 if logged out)     |
| GET    | `/api/auth/methods` | `{oidc, oidcName}`: sign-in options |
| GET    | `/api/auth/oidc/login?redirect=/path` | Starts SSO (browser navigation) |
| GET    | `/api/auth/oidc/callback` | SSO redirect target          |
| GET    | `/api/users`       | List users (admin)                   |
| POST   | `/api/users`       | `{username, password, isAdmin}` (admin) |
| PUT    | `/api/users/{id}`  | `{isAdmin, password?}` (admin)       |
| DELETE | `/api/users/{id}`  | Delete user (admin)                  |
| GET    | `/api/groups`      | List groups with members (admin)     |
| GET    | `/api/units`       | List units                           |
| POST   | `/api/units`       | `{name, position?: {lat, lon, height?, accuracy?, speed?, course?, timestamp?}, symbol?, tacticalName?}` |
| GET    | `/api/units/events` | WebSocket pushing `{type, id, unit?}` on every unit create/update/delete |
| GET    | `/api/units/{id}`  | Get unit by UUID                     |
| GET    | `/api/units/{id}/positions?limit=&since=&to=` | Position history, newest first; `since` and `to` (RFC 3339) limit it to measurements in that timeframe |
| PUT    | `/api/units/{id}`  | Same body as POST; a missing position clears it |
| PATCH  | `/api/units/{id}`  | Changes only the fields sent; null clears |
| DELETE | `/api/units/{id}`  | Delete unit                          |
| GET    | `/api/version`     | `{commit, version?}`: build info     |
| GET    | `/api/instance`    | `{name?}`: instance name             |
| GET    | `/api/health`      | Health check                         |
