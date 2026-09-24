# go-unit-mangement

A Go web server (net/http + GORM + PostgreSQL) with session-based login. The UI in `frontend/` (Vite + React, MUI, React Router, TanStack Query, i18next with English and German) follows the structure of [Tileserve-GO](https://github.com/Nils-witt/Tileserve-GO)'s frontend and is built and embedded into the Go binary.

## Quick start

```sh
make db                                   # start Postgres via docker compose
ADMIN_PASSWORD=change-me make run         # build UI + server, run on :8080
```

On first start, if the database has no users, an administrator account is created from `ADMIN_USERNAME` (default `admin`) and `ADMIN_PASSWORD`. If users exist but none is an administrator (a database from before roles existed), the `ADMIN_USERNAME` account is promoted.

## Users and SSO

Administrators manage accounts on the **Users** page: create local accounts (passwords of at least 8 characters), grant or revoke the administrator role, reset passwords and delete accounts. Resetting a password or deleting an account ends that user's sessions. Administrators can't remove their own role or delete their own account.

SSO uses OpenID Connect (authorization code flow with PKCE) and works with any compliant provider (Keycloak, Authentik, Entra ID, Google, Dex, …). Set `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` and `OIDC_REDIRECT_URL`, and register the redirect URL (`https://<host>/api/auth/oidc/callback`) at the provider. The login page then shows a "Sign in with `OIDC_DISPLAY_NAME`" button.

The first SSO sign-in creates an account linked to the provider's issuer and subject, named after the `preferred_username` claim (or the email, or the subject). New SSO accounts are not administrators and have no password; an administrator can promote them or set a password to also allow password sign-in. Anyone who can sign in at the provider gets an account, so restrict access to the client at the provider if needed.

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

See `.env.example`. Set `COOKIE_SECURE=true` when serving over HTTPS.

Behind a reverse proxy, set `TRUSTED_PROXIES` to its addresses (comma-separated IPs or CIDRs, e.g. `10.0.0.0/8,::1`) so logs show the real client IP. For requests from those addresses the client is taken from `X-Forwarded-For` (the rightmost entry that isn't a trusted proxy) or `X-Real-IP`; the headers are ignored from anyone else, so they can't be spoofed.

## API

The full API is described in [`api/openapi.yaml`](api/openapi.yaml) (OpenAPI 3.1).

| Method | Path               | Description                          |
|--------|--------------------|--------------------------------------|
| POST   | `/api/auth/login`  | `{username, password}` → sets cookie |
| POST   | `/api/auth/logout` | Ends the session                     |
| GET    | `/api/auth/me`     | Current user (401 if logged out)     |
| GET    | `/api/auth/methods` | `{oidc, oidcName}`: sign-in options |
| GET    | `/api/auth/oidc/login?redirect=/path` | Starts SSO (browser navigation) |
| GET    | `/api/auth/oidc/callback` | SSO redirect target          |
| GET    | `/api/users`       | List users (admin)                   |
| POST   | `/api/users`       | `{username, password, isAdmin}` (admin) |
| PUT    | `/api/users/{id}`  | `{isAdmin, password?}` (admin)       |
| DELETE | `/api/users/{id}`  | Delete user (admin)                  |
| GET    | `/api/units`       | List units                           |
| POST   | `/api/units`       | `{name, position?: {lat, lon, height?, timestamp?}}` |
| GET    | `/api/units/events` | WebSocket pushing `{type, id, unit?}` on every unit create/update/delete |
| GET    | `/api/units/{id}`  | Get unit by UUID                     |
| PUT    | `/api/units/{id}`  | Same body as POST; a missing position clears it |
| DELETE | `/api/units/{id}`  | Delete unit                          |
| GET    | `/api/health`      | Health check                         |
