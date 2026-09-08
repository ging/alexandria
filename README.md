# Alexandria Vocabulary Hub

![banner alexandria](./docs/static/banner.png)

<!-- State of the code: CI, release, docs, license -->
[![ci](https://github.com/caparicio-esd/alexandria/actions/workflows/ci.yaml/badge.svg)](https://github.com/caparicio-esd/alexandria/actions/workflows/ci.yaml)
[![docs](https://img.shields.io/badge/docs-fumadocs-FF3366?logo=bookstack&logoColor=white)](https://caparicio-esd.github.io/alexandria/)
[![go report card](https://goreportcard.com/badge/github.com/caparicio-esd/alexandria)](https://goreportcard.com/report/github.com/caparicio-esd/alexandria)
[![go reference](https://pkg.go.dev/badge/github.com/caparicio-esd/alexandria.svg)](https://pkg.go.dev/github.com/caparicio-esd/alexandria)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![last commit](https://img.shields.io/github/last-commit/caparicio-esd/alexandria?color=6f42c1)](https://github.com/caparicio-esd/alexandria/commits/main)

<!-- Technology stack -->
[![go](https://img.shields.io/github/go-mod/go-version/caparicio-esd/alexandria?logo=go&logoColor=white&label=go)](go.mod)
[![postgres](https://img.shields.io/badge/postgres-17-4169E1?logo=postgresql&logoColor=white)](docker-compose.dev.yaml)
[![zitadel](https://img.shields.io/badge/zitadel-v4-2b3990?logo=auth0&logoColor=white)](https://caparicio-esd.github.io/alexandria/docs/development/authentication)
[![caddy](https://img.shields.io/badge/caddy-2-1F88C0?logo=caddy&logoColor=white)](Caddyfile)
[![docker](https://img.shields.io/badge/docker-compose-2496ED?logo=docker&logoColor=white)](deploy/docker/README.md)
[![task](https://img.shields.io/badge/task-runner-29BEB0?logo=task&logoColor=white)](Taskfile.yaml)
[![adr](https://img.shields.io/badge/decisions-ADR-informational)](https://caparicio-esd.github.io/alexandria/docs/adr)

> 📖 **Full Documentation Portal:** Comprehensive guides, architecture diagrams, API specs, and Architecture Decision Records (ADRs) are published at **[caparicio-esd.github.io/alexandria](https://ging.github.io/alexandria/)**.

---

## Overview

**Alexandria** is a **Vocabulary Hub** engineered for sovereign dataspaces. It provides the single source of semantic truth to host, publish, and version domain ontologies, dereference semantic terms in real-time for dataspace connectors, and execute automated remote conformance tests.

### Core Capabilities

- **Vocabulary Lifecycle & Publishing:** Author, version, document, and publish RDF, OWL, and SKOS vocabularies and dataset profiles extending shared dataspace models.
- **Runtime Semantic Dereferencing:** Real-time IRI resolution for dataspace Connectors parsing Self-Descriptions and data agreements without requiring manual bilateral coordination.
- **Remote Conformance Testing:** Automated semantic verification using W3C SHACL constraint shapes to certify that connector payloads and datasets comply with dataspace standards.
- **Trust Anchor & Delegated Wallets:** Private keys and signing operations never reside in the node; key custody is delegated to external wallets (supporting both **Fafnir** and **Eclipse EDC IdentityHub**).
- **Node-Terminated Authentication:** Secure OpenID Connect (OIDC) authentication with PKCE via **Zitadel**, issuing encrypted `HttpOnly` session cookies.
- **In-Process Modular Monolith:** Engineered in Go around Hexagonal Architecture with strict module boundaries and complete TLS reverse proxy parity via **Caddy**.

---

## Quickstart

### Prerequisites

- **Go 1.26+**
- **Docker & Docker Compose**
- **[Task](https://taskfile.dev)** (manages all development, build, and container lifecycles)
- **OpenSSL** (preinstalled on macOS/Linux for local CA generation)

### One-Command Setup

Alexandria includes automated environment orchestration. Run:

```bash
# First time on a new workstation (trusts local CA, requires sudo once):
task tls:trust

# Bootstrap and start the entire stack:
task dev:auto
```

This single command brings up PostgreSQL, Caddy, Zitadel IAM, the wallet service, provisions required credentials, and starts the Alexandria node with hot-reload (`air`).

### Local Access Points

- **Web Portal / API:** [`https://alexandria.127.0.0.1.nip.io:8443`](https://alexandria.127.0.0.1.nip.io:8443)
- **Login:** `https://alexandria.127.0.0.1.nip.io:8443/api/v1/auth/login`  
  *(Default credentials: `admin@alexandria.auth.127.0.0.1.nip.io` / `Password1!`)*
- **Decentralized Identifier (DID):** [`https://alexandria.127.0.0.1.nip.io:8443/.well-known/did.json`](https://alexandria.127.0.0.1.nip.io:8443/.well-known/did.json)
- **Health Check:** `https://alexandria.127.0.0.1.nip.io:8443/healthz`
- **Documentation (Local):** `task docs:dev` -> [http://localhost:3000](http://localhost:3000)

---

## Developer Commands

All routine development and verification tasks are managed through `Taskfile.yaml`:

| Command | Description |
|:---|:---|
| `task dev:auto` | Orchestrate containers, bootstrap IAM/secrets, and start the node with hot reload. |
| `task test` | Run unit tests with race detection and generate coverage report. |
| `task test:integration` | Run integration test suites against live wallet services. |
| `task test:all` | Run complete unit and integration test suites. |
| `task check` | Full CI verification pipeline: format (`gofumpt`), lint (`golangci-lint`), and tests. |
| `task vuln` | Scan project dependencies for known vulnerabilities (`govulncheck`). |
| `task docs:dev` | Start the local Fumadocs documentation dev server with hot reload. |
| `task docs:build` | Build static documentation export for GitHub Pages. |

---

## Documentation Map

For in-depth explanations, configuration details, and architecture specifications, explore our documentation portal:

| Topic | Documentation Guide |
|:---|:---|
| **Getting Started** | [Prerequisites & Local Quickstart](https://caparicio-esd.github.io/alexandria/docs/getting-started/quickstart) |
| **Configuration** | [One Document Model & Viper Overrides](https://caparicio-esd.github.io/alexandria/docs/getting-started/configuration) |
| **Architecture** | [Hexagonal Design & Bounded Contexts](https://caparicio-esd.github.io/alexandria/docs/architecture/bounded-contexts) |
| **Wallet Port** | [Unified Wallet Port & Adapters (Fafnir / IdentityHub)](https://caparicio-esd.github.io/alexandria/docs/architecture/wallet-port) |
| **Remote Conformance** | [SHACL Shape Validation & Conformance Testing](https://caparicio-esd.github.io/alexandria/docs/development/conformance) |
| **Authentication & IAM** | [Zitadel OIDC, PKCE & Session Security](https://caparicio-esd.github.io/alexandria/docs/development/authentication) |
| **REST API Reference** | [HTTP Endpoints Catalog](https://caparicio-esd.github.io/alexandria/docs/development/endpoints) |
| **Architecture Decisions** | [Accepted ADRs (0001 - 0007)](https://caparicio-esd.github.io/alexandria/docs/adr) |

---

## Repository Structure

```text
alexandria/
├── cmd/alexandria/         # Application entrypoint & composition root
├── config/                 # Baseline configuration (config.yaml)
├── deploy/                 # Docker Compose and Helm deployment charts
├── docs/                   # Documentation portal (Fumadocs + Next.js)
│   ├── app/                # Documentation site routes & landing page
│   ├── content/docs/       # Markdown/MDX documentation source
│   └── adr/                # Original Architecture Decision Records
├── internal/
│   ├── auth-proxy/         # Bounded context: Zitadel OIDC guard & session handling
│   ├── httpapi/            # Gin engine, route mounting pipeline & health probes
│   ├── observability/      # Logging (slog), metrics & telemetry
│   ├── ssi-auth/           # Bounded context: DID resolution, wallet port & adapters
│   │   ├── wallet/         # Unified wallet.Wallet port & domain models
│   │   ├── fafnir/         # Driven adapter for Fafnir wallet
│   │   └── identityhub/    # Driven adapter for Eclipse EDC IdentityHub
│   └── storage/            # Database repositories and migrations
└── Taskfile.yaml           # Automation and development workflows
```

---

## License

This project is licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.
