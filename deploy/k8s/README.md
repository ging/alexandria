# Kubernetes Deployment (kind + nip.io)

This directory deploys Alexandria on a local `kind` cluster using Kustomize (`kubectl apply -k`). It mirrors the stack in [deploy/docker](../docker/README.md), split into `base/` and an optional `overlays/demo/` overlay.

Components:

- `ingress-nginx` with a local CA (`alexandria-tls` and `alexandria-ca` secrets), terminating TLS on port `8443` for `*.127.0.0.1.nip.io` without editing `/etc/hosts` or binding privileged ports. CoreDNS gets `rewrite` entries so pods can reach `*.127.0.0.1.nip.io` through the internal ingress service with full TLS verification.
- `postgres:17-alpine` backed by a PVC (`postgres-data`), initialized with three databases: `keycloak`, `alexandria_hub`, and `alexandria_client`.
- `quay.io/keycloak/keycloak:26.2` connected to PostgreSQL, auto-importing the `alexandria` realm with RFC 9068 access tokens (`typ: at+jwt`), audience `alexandria-vhub`, the `roles` claim (`vhub-admin`), PKCE (`S256`) for `/ui/`, and CLI/M2M clients.
- `ghcr.io/ging/alexandria/server:latest` (`alexandria-server`) with the embedded web console (`/ui/`), Ed25519 signing key and blobs stored in the `hub-data` PVC, and internal OIDC discovery/JWKS against `http://keycloak:8080/realms/alexandria`.
- `overlays/demo/` (optional): adds `ghcr.io/ging/alexandria/client:latest` (`alexandria-client`, PostgreSQL state + `client-state` PVC, auto-discovering `hub_did` over HTTPS) and `ghcr.io/ging/alexandria/fake-connector:latest` (`fake-connector`), exposed via Ingress (`client.127.0.0.1.nip.io:8443`, `dsp.127.0.0.1.nip.io:8443`) and NodePort (`127.0.0.1:9090`, `127.0.0.1:9080`, `127.0.0.1:9081`).

## Prerequisites

- Docker (Docker Desktop, OrbStack, Colima, or Docker Engine)
- `kind`
- `kubectl` (with built-in Kustomize support)
- `openssl`

## Quick Start

From the repository root:

```bash
# Start kind + ingress-nginx + PostgreSQL + Keycloak + alexandria-server (pulls GHCR images)
task k8s:up

# Start the full stack including alexandria-client and fake-connector (pulls GHCR images)
task k8s:up:demo

# Build images locally from source and load them into kind instead of pulling from GHCR
task k8s:up:build
task k8s:up:demo:build
```

You can also invoke the script directly (or use `make k8s-up` / `make k8s-up-demo`):

```bash
./deploy/k8s/scripts/up.sh
./deploy/k8s/scripts/up.sh --demo

# Build and load local images from the root Dockerfile into kind
./deploy/k8s/scripts/up.sh --demo --build
```

Running `up.sh` is idempotent:

1. Creates the `alexandria` kind cluster from [kind-config.yaml](kind-config.yaml) if it does not exist.
2. Installs `ingress-nginx` for `kind` and waits for the controller pod to become ready.
3. Creates the `alexandria-ingress-internal` service (`8443 -> 443`) and adds CoreDNS rewrite rules for `*.127.0.0.1.nip.io`.
4. Generates a local Root CA in `deploy/k8s/.certs/` (gitignored, reused across runs), copies it to `deploy/k8s/ca-root.crt`, and syncs the `alexandria-tls` and `alexandria-ca` secrets.
5. When `--build` is passed, builds local container images from the root `Dockerfile` and loads them with `kind load docker-image`; otherwise Kubernetes pulls `ghcr.io/ging/alexandria/*:latest` directly from GHCR.
6. Runs `kubectl apply -k` (`deploy/k8s/base` or `deploy/k8s/overlays/demo`) and waits for rollouts.

### Trusting the Local CA

The Root CA is created once in `deploy/k8s/.certs/ca.crt` and exported to `deploy/k8s/ca-root.crt`. Trust it once so browsers (for OIDC login in `/ui/`) and `curl` accept the local certificates without warnings:

```bash
# Export the CA on its own if needed
task k8s:ca

# macOS
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain deploy/k8s/ca-root.crt

# Linux (Debian/Ubuntu)
sudo cp deploy/k8s/ca-root.crt /usr/local/share/ca-certificates/alexandria-kind.crt && sudo update-ca-certificates
```

## Endpoints and Credentials

| Service | URL | Notes |
|:---|:---|:---|
| Hub API and publication | `https://hub.127.0.0.1.nip.io:8443` (`/v1`, `/pub`) | Validates Keycloak `at+jwt` tokens |
| Hub web console | `https://hub.127.0.0.1.nip.io:8443/ui/` | OIDC + PKCE (`alexandria-console`) |
| Keycloak OIDC issuer | `https://auth.127.0.0.1.nip.io:8443/realms/alexandria` | Auto-imported `alexandria` realm |
| Keycloak admin console | `https://auth.127.0.0.1.nip.io:8443/admin/` | `admin` / `admin` (`Secret/alexandria-credentials`) |
| Client admin and console (`demo`) | `https://client.127.0.0.1.nip.io:8443/ui/` (or `http://127.0.0.1:9090/ui/`) | Status, dynamic rules, `:explain`, `POST /sync` |
| Client DSP ingress (`demo`) | `https://dsp.127.0.0.1.nip.io:8443` (or `http://127.0.0.1:9080`) | L7 proxy in front of `fake-connector` |

### Preconfigured Realm Users

| User | Password | Role | Fixed `sub` |
|:---|:---|:---|:---|
| `ana` | `ana123` | `vhub-admin` | `11111111-1111-4111-8111-111111111111` |
| `sergio` | `sergio123` | `steward` | `22222222-2222-4222-8222-222222222222` |

A confidential `alexandria-m2m` client (`secret: alexandria-m2m-secret`) with `client_credentials` enabled is also included for automated pipelines.

## Smoke Test with `curl`

```bash
export CURL_CA_BUNDLE="$PWD/deploy/k8s/ca-root.crt"

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

## Operations and Cleanup

```bash
# Inspect pods, services, ingresses, and PVCs
task k8s:status

# Stream logs from all pods in the alexandria namespace
task k8s:logs

# Delete the alexandria namespace (keeps the kind cluster and ingress-nginx)
task k8s:down

# Delete the entire kind cluster
task k8s:delete
```

## Adapting for Production

Create a new Kustomize overlay (for example `deploy/k8s/overlays/prod/`) referencing `../../base` and change the following:

1. Secrets: replace `base/secret.yaml` with ExternalSecrets, SealedSecrets, or cluster-managed secrets using strong passwords (or point at external PostgreSQL and Keycloak instances).
2. TLS: replace `gen-certs.sh` with `cert-manager` annotations (such as `cert-manager.io/cluster-issuer: letsencrypt-prod`) on the `Ingress` objects.
3. Public hostnames on port `443`: patch the hostnames in `ingress.yaml`, `KC_HOSTNAME` in `keycloak.yaml`, `-base`/`-issuer` in `server.yaml`, and `redirectUris` for `alexandria-console` in `alexandria-realm.json`.
4. Container images: pin `ghcr.io/ging/alexandria/server`, `ghcr.io/ging/alexandria/client`, and `ghcr.io/ging/alexandria/fake-connector` from `:latest` to immutable release tags (for example `1.0.0`) using `images:` in the overlay `kustomization.yaml`.
