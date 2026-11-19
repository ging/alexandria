# Docker Compose Deployment (Caddy + PostgreSQL + Keycloak)

This directory deploys Alexandria with Docker Compose:

- `caddy:2.9-alpine`: reverse proxy terminating TLS in front of the Hub and Keycloak (`*.127.0.0.1.nip.io` with a local CA by default, or Let's Encrypt for real domains).
- `postgres:17-alpine`: shared PostgreSQL instance initialized with three databases (`keycloak`, `alexandria_hub` with immutability triggers on append-only tables, and `alexandria_client`).
- `quay.io/keycloak/keycloak:26.2`: OAuth 2.0 / OIDC provider with an auto-imported `alexandria` realm, RFC 9068 access tokens (`typ: at+jwt`), audience `alexandria-vhub`, the `roles` claim (`vhub-admin`), PKCE (`S256`) for `/ui/`, and CLI/M2M clients.
- `ghcr.io/ging/alexandria/server:latest` (`alexandria-server`): Vocabulary Hub with the embedded web console (`/ui/`).
- Optional `demo` profile (`--profile demo`): adds `ghcr.io/ging/alexandria/client:latest` (`alexandria-client`, PostgreSQL state, auto-discovering `hub_did`) and `ghcr.io/ging/alexandria/fake-connector:latest` (`fake-connector`).

## Quick Start (Local `*.127.0.0.1.nip.io`)

By default, Compose pulls the prebuilt multi-arch images from GHCR (`ALEXANDRIA_IMAGE_REGISTRY=ghcr.io/ging/alexandria`, `ALEXANDRIA_TAG=latest`). From the repository root (using `task`, `make`, or `docker compose` directly):

```bash
# Optional: customize settings (including ALEXANDRIA_TAG) via .env
cp deploy/docker/.env.example deploy/docker/.env

# Start Hub + Caddy + PostgreSQL + Keycloak using GHCR images
task docker:up

# Or include alexandria-client and fake-connector
task docker:up:demo

# Build images locally from source instead of pulling from GHCR
task docker:up:build
task docker:up:demo:build
```

Equivalent `docker compose` commands:

```bash
docker compose -f deploy/docker/docker-compose.yaml up -d
docker compose -f deploy/docker/docker-compose.yaml --profile demo up -d

# Rebuild from local source
docker compose -f deploy/docker/docker-compose.yaml --profile demo up -d --build
```

### Trusting Caddy's Local CA

When `CADDY_TLS=internal` (the default), Caddy signs certificates for `hub.127.0.0.1.nip.io` and `auth.127.0.0.1.nip.io` with its local CA. Export and trust it once so browsers (for OIDC login in `/ui/`) and `curl` accept the local certificates without warnings:

```bash
# Export Caddy's root CA to deploy/docker/caddy-root.crt
task docker:ca

# macOS
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain deploy/docker/caddy-root.crt

# Linux (Debian/Ubuntu)
sudo cp deploy/docker/caddy-root.crt /usr/local/share/ca-certificates/alexandria-caddy.crt && sudo update-ca-certificates
```

## Endpoints and Credentials

| Service | URL | Notes |
|:---|:---|:---|
| Hub API and publication | `https://hub.127.0.0.1.nip.io:8443` (`/v1`, `/pub`) | Validates Keycloak `at+jwt` tokens |
| Hub web console | `https://hub.127.0.0.1.nip.io:8443/ui/` | OIDC + PKCE (`alexandria-console`) |
| Keycloak OIDC issuer | `https://auth.127.0.0.1.nip.io:8443/realms/alexandria` | Auto-imported `alexandria` realm |
| Keycloak admin console | `https://auth.127.0.0.1.nip.io:8443/admin/` | `admin` / `admin` (`KEYCLOAK_ADMIN` / `KEYCLOAK_ADMIN_PASSWORD`) |
| Client admin and console (`demo`) | `http://127.0.0.1:9090/ui/` | Status, dynamic rules, `:explain`, `POST /sync` |
| Client DSP ingress (`demo`) | `http://127.0.0.1:9080` | L7 proxy in front of `fake-connector` |

### Preconfigured Realm Users

| User | Password | Role | Fixed `sub` |
|:---|:---|:---|:---|
| `ana` | `ana123` | `vhub-admin` | `11111111-1111-4111-8111-111111111111` |
| `sergio` | `sergio123` | `steward` | `22222222-2222-4222-8222-222222222222` |

A confidential `alexandria-m2m` client (`secret: alexandria-m2m-secret`) with `client_credentials` enabled is also included for automated pipelines.

## Smoke Test with `curl`

```bash
# Export Caddy's local CA for curl
task docker:ca
export CURL_CA_BUNDLE="$PWD/deploy/docker/caddy-root.crt"

# Request an RFC 9068 access token (at+jwt) as ana
ANA_TOKEN=$(curl -fsSL -X POST \
  "https://auth.127.0.0.1.nip.io:8443/realms/alexandria/protocol/openid-connect/token" \
  -d "client_id=alexandria-cli" \
  -d "grant_type=password" \
  -d "username=ana" \
  -d "password=ana123" | jq -r .access_token)

# Create the "mobility" namespace
curl -i -X POST "https://hub.127.0.0.1.nip.io:8443/v1/namespaces" \
  -H "Authorization: Bearer $ANA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"slug":"mobility","displayName":"GreenCity Mobility"}'

# Assign sergio as steward of "mobility"
curl -i -X POST "https://hub.127.0.0.1.nip.io:8443/v1/namespaces/mobility/stewards" \
  -H "Authorization: Bearer $ANA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "iss": "https://auth.127.0.0.1.nip.io:8443/realms/alexandria",
    "sub": "22222222-2222-4222-8222-222222222222",
    "label": "sergio"
  }'
```

## Using a Real Domain with Let's Encrypt

1. Point DNS `A`/`AAAA` records for your subdomains (for example `hub.example.org` and `auth.example.org`) to the Docker host.
2. Configure `deploy/docker/.env` (pin `ALEXANDRIA_TAG` to a release tag):
   ```dotenv
   ALEXANDRIA_TAG=1.0.0
   HUB_DOMAIN=hub.example.org
   AUTH_DOMAIN=auth.example.org
   HTTPS_PORT=443
   CADDY_TLS=ops@example.org
   POSTGRES_PASSWORD=<strong-secret>
   KEYCLOAK_ADMIN_PASSWORD=<strong-secret>
   ```
3. Add `"https://hub.example.org/ui/*"` to `redirectUris` for the `alexandria-console` client (in `deploy/docker/keycloak/alexandria-realm.json` before the first boot, or in the Keycloak admin console) and change the initial test passwords.
4. Run `docker compose -f deploy/docker/docker-compose.yaml up -d`. Caddy obtains and renews TLS certificates via Let's Encrypt (TLS-ALPN-01 on `:443`).
