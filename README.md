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

`go build` embeds whatever is in `frontend/dist`, so run `npm run build` in `frontend/` before building the binary (`make build` does this).

## Configuration

See `.env.example`. Set `COOKIE_SECURE=true` when serving over HTTPS.

## API

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
| GET    | `/api/health`      | Health check                         |
