# Alexandria Console (`gui/`)

Web console for **alexandria-server** (C-HUB) and **alexandria-client** (C-CLI).
React 19 · TypeScript · Vite · TanStack Router / Query / Table · Tailwind CSS v4 · Radix · CodeMirror.

> Laboratory tool (DP-14): served either by the Vite dev server, which proxies the two binaries, or embedded in them with `-tags gui`. It is not a hardened production deployment.

## Run

```bash
cd gui
pnpm install
pnpm dev          # http://localhost:5273
```

Upstreams are proxied same-origin (no CORS, lab TLS certificate accepted). Override them in `gui/.env.local` (see `.env.example`):

| Variable | Default | Proxied under |
|:---|:---|:---|
| `ALEXANDRIA_HUB_URL` | `https://127.0.0.1:8443` | `/api/hub` |
| `ALEXANDRIA_CLIENT_ADMIN_URL` | `http://127.0.0.1:9090` | `/api/client` |
| `ALEXANDRIA_OIDC_ISSUER` | `http://127.0.0.1:9400` | `/api/idp` |

`pnpm build` type-checks and emits `dist/`; `pnpm preview` serves it with the same proxies.

## Embedded in the binaries

```bash
make build-gui    # or: task build:gui
```

This runs `pnpm build:embed` (output in `internal/webui/dist`, base path `/ui/`) and rebuilds both binaries with `-tags gui`. Each binary then serves its own half of the console, with no Node, Vite or proxy at runtime:

| Binary | URL | Workspace |
|:---|:---|:---|
| `alexandria-server` | `{base}/ui/` (e.g. `https://127.0.0.1:8443/ui/`) | server, next to `/v1` and `/pub` |
| `alexandria-client` | `http://{admin bind}/ui/` | client, on the admin listener only |

Plain `make build` and `go build ./...` stay free of the console (no `gui` tag, nothing mounted). When embedded, the hub injects its first accepted issuer and audience as the OIDC defaults, and the browser talks to the issuer directly, so the IdP must allow the hub origin (CORS) and the redirect URI `{base}/ui/auth/callback`. Open the hub console on the host of its configured `base`: `/pub` redirects are absolute and would otherwise be cross-origin.

## Authentication (OAuth 2.0 / OIDC)

The hub is an OAuth 2.0 resource server (RFC 9068 `at+jwt`). The console obtains bearer tokens in two ways (identity menu, top right):

- **Sign in with OIDC** — Authorization Code + PKCE (S256) with `state` and `nonce`, driven by the issuer's discovery document. Refresh tokens are redeemed silently before expiry and sign-out uses RP-initiated logout when `end_session_endpoint` is advertised. Register a *public* client with redirect URI `http://localhost:5273/auth/callback`. The issuer must be one the hub accepts (`auth_config.issuer`), and the access token must be a JWT carrying the hub audience and, for admins, the `vhub-admin` role.
- **Access token** — paste a token. This is the path for the lab `fake-idp`, which only publishes discovery and JWKS: `./bin/fake-idp token --sub ana --admin --ttl 24h`.

Several identities can be saved and switched (e.g. an admin and a steward). Tokens are kept in `localStorage` of this origin; the client admin API is unauthenticated.

Hub roles: the **admin** creates namespaces, assigns stewards and reads the global event log; **stewards** perform every namespace operation (vocabularies, drafts, releases, revocation, namespace events).

## Views

**alexandria-server** — Namespaces · Overview and stewards · Vocabularies (hosted/proxy, revisions, content) · Authoring (draft → validate → propose → approve/reject) · Versions (signed listing, timeline, records, revoke) · Events · Publication (`/pub` explorer).

**alexandria-client** — Status (state machine, listing freshness, trust anchor, probes) · Versions · Rules (CRUD with `If-Match`, mode switch) · Explain (side-effect-free dry run) · Agreements · Audit · Metrics.

`⌘K` / `Ctrl K` opens the command palette. Light, dark and system themes.

## Layout

```text
src/
├── lib/          http (problem+json), hub and client API clients, oidc, settings store, formatters
├── components/   ui primitives, shell (top bar, sidebar, command palette, identity), tables, editor
└── routes/       hub/* and client/* views, auth-callback
```
