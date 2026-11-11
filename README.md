# Alexandria Vocabulary Hub

![banner alexandria](./docs/static/banner.png)

<!-- State of the code: CI, release, docs, license -->
[![ci](https://github.com/ging/alexandria/actions/workflows/ci.yaml/badge.svg)](https://github.com/ging/alexandria/actions/workflows/ci.yaml)
[![docs](https://img.shields.io/badge/docs-fumadocs-FF3366?logo=bookstack&logoColor=white)](https://ging.github.io/alexandria/)
[![go report card](https://goreportcard.com/badge/github.com/ging/alexandria)](https://goreportcard.com/report/github.com/ging/alexandria)
[![go reference](https://pkg.go.dev/badge/github.com/ging/alexandria.svg)](https://pkg.go.dev/github.com/ging/alexandria)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![last commit](https://img.shields.io/github/last-commit/ging/alexandria?color=6f42c1)](https://github.com/ging/alexandria/commits/main)

<!-- Technology stack -->
[![go](https://img.shields.io/github/go-mod/go-version/ging/alexandria?logo=go&logoColor=white&label=go)](go.mod)
[![postgres](https://img.shields.io/badge/postgres-17-4169E1?logo=postgresql&logoColor=white)](docker-compose.dev.yaml)
[![zitadel](https://img.shields.io/badge/zitadel-v4-2b3990?logo=auth0&logoColor=white)](https://ging.github.io/alexandria/docs/development/authentication)
[![caddy](https://img.shields.io/badge/caddy-2-1F88C0?logo=caddy&logoColor=white)](Caddyfile)
[![docker](https://img.shields.io/badge/docker-compose-2496ED?logo=docker&logoColor=white)](deploy/docker/README.md)
[![task](https://img.shields.io/badge/task-runner-29BEB0?logo=task&logoColor=white)](Taskfile.yaml)
[![adr](https://img.shields.io/badge/decisions-ADR-informational)](https://ging.github.io/alexandria/docs/adr)

> 📖 **Full Documentation Portal:** Comprehensive guides, architecture diagrams, API specs, and Architecture Decision Records (ADRs) are published at **[ging.github.io/alexandria](https://ging.github.io/alexandria/)**.

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
| **Getting Started** | [Prerequisites & Local Quickstart](https://ging.github.io/alexandria/docs/getting-started/quickstart) |
| **Incremental Tutorials** | [5-Step Hands-On Tutorial Path (`alexandriac`, `alexandria-server`, `alexandria-client`, F6 & Wallets)](https://ging.github.io/alexandria/docs/tutorials) |
| **Vocabulary Hub & DSL** | [`vhub` v0.3.0 Specification, `eunomia.profiles/v1` DSL & F5 Catalog](https://ging.github.io/alexandria/docs/vhub) |
| **Configuration** | [`server.json`, Client `config.json` & Node `config.yaml` Reference](https://ging.github.io/alexandria/docs/getting-started/configuration) |
| **Architecture** | [Hexagonal Design, `archtest` & Bounded Contexts](https://ging.github.io/alexandria/docs/architecture/bounded-contexts) |
| **Wallet Port** | [Unified Wallet Port & Adapters (Fafnir / IdentityHub / DCP)](https://ging.github.io/alexandria/docs/architecture/wallet-port) |
| **Conformance & Traceability** | [81 `REQ-*`, 77 `F5` Codes, Scenario F6 & `TRACEABILITY.md`](https://ging.github.io/alexandria/docs/development/conformance) |
| **Authentication & IAM** | [Zitadel OIDC, PKCE & `alexandria-server` Role Authorization](https://ging.github.io/alexandria/docs/development/authentication) |
| **REST API Reference** | [Complete HTTP Endpoints Catalog & Swagger UI (`/docs`)](https://ging.github.io/alexandria/docs/development/endpoints) |
| **Architecture Decisions** | [Accepted ADRs (0001 - 0009, `DP-1..14`, `Q1..22`)](https://ging.github.io/alexandria/docs/adr) |

---

## Vocabulary Hub (`alexandria-server` & `alexandria-client`) — Specification 0.3.0

Alongside the integrated `cmd/alexandria` node, the root repository contains the reference implementation of **Vocabulary Hub (`alexandria-server`)** and its L7 edge validation proxy (**`alexandria-client`**) conforming to **Specification v0.3.0** (F2–F8, Appendices A–C, 81 `REQ-*`, and the 77-code closed F5 catalog).

> **Laboratory Environment Notice (`DP-14`):** The configurations in `deploy/` (`server.json`, `client.json`, `rules.seed.json`), the local OAuth 2.0/OIDC issuer `deploy/fake-idp` (`DP-4`), the simulated DSP connector `deploy/fake-connector` (`Part 5.2`), local PEM signing keys, and transient lab TLS certificates (`-hub-ca`) are designed for laboratory and PoC demonstration (`PRE-01`–`PRE-07`, `DP-1`–`DP-14`) and **are not intended for production deployment**.

### Quick Verification & F6 Demo in 3 Commands

```bash
# 1. Build binaries (bin/alexandria-server, bin/alexandria-client, bin/fake-idp, bin/fake-connector)
make build

# 2. Run unit, architecture, conformance (81 T-* and 76 N-*), E2E F6, and traceability checks
make test conformance e2e trace

# 3. Launch fake-idp + fake-connector + server + client, replay F6 with file_clock, and print the verdict table
make demo
```

### Profile Language (YAML → Artifacts)

Profiles can also be authored in YAML (`eunomia.profiles/v1`) and compiled offline:

```bash
go run ./cmd/alexandriac compile -o out internal/dsl/testdata/colors.profile.yaml
```

With `"compiler": "dsl"` in `server.json`, the hub accepts YAML drafts (`PROFILES_DSL.md`).

### `Makefile` Targets

| Command | Description |
|:---|:---|
| `make build` | Builds `bin/alexandria`, `bin/alexandriac`, `bin/alexandria-server`, `bin/alexandria-client`, `bin/fake-idp`, and `bin/fake-connector`. |
| `make test` | Runs unit, contract, and hexagonal architecture tests (`internal/...`, `cmd/...`). |
| `make conformance` | Runs the conformance battery (`T-DOC-*`, `T-SYS-*`, `N-<CODE>` for all 76 reachable F5 codes, and `TestE2E_F6`). |
| `make e2e` | Runs the end-to-end `TestE2E_F6` scenario across the four real binaries (`fake-idp`, `fake-connector`, `alexandria-server`, `alexandria-client`). |
| `make demo` | Launches the four binaries with a shared `file_clock`, replays F6.1–F6.4, and prints the 29-row verdict table to `stdout`. |
| `make trace` | Runs `go run ./conformance/trace` to regenerate [`TRACEABILITY.md`](TRACEABILITY.md) and verify 81/81 `REQ-*` and 76/77 F5 codes. |

Root traceability and design documents:
- [`TRACEABILITY.md`](TRACEABILITY.md): Auto-generated matrix `REQ-* → Section → Module(s) → Test(s) T-*` and `N-<CODE>` coverage.
- [`DECISIONS.md`](DECISIONS.md): PoC decisions `DP-1`–`DP-14` and agent implementation decisions.
- [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md): Unreachable F5 code rationale (`VERDICT_SHACL_WARNING` under `DP-7`), F7.2 `Q18`–`Q22`, and spec observations (`G9`, `G10`).

---

## Repository Structure

```text
alexandria/
├── cmd/
│   ├── alexandria/           # Identity & Auth gateway composition root
│   ├── alexandriac/          # Offline profile DSL compiler CLI
│   ├── alexandria-server/    # Vocabulary Hub server composition root (C-HUB)
│   └── alexandria-client/    # L7 edge validation & sync proxy composition root (C-CLI)
├── conformance/              # Conformance battery, negative corpus (N-*), F6 E2E, demo & trace generator
├── deploy/                   # server.json, client.json, rules.seed.json, fake-idp/ & fake-connector/
├── docs/                     # Documentation portal (Fumadocs + Next.js)
├── internal/
│   ├── archtest/             # Hexagonal architecture, DP-1 dependency & DP-8 clock enforcement tests
│   ├── auth-proxy/           # Bounded context: Zitadel OIDC guard & session handling
│   ├── client/               # Bounded context: alexandria-client (module.go, domain, app, config, adapters)
│   ├── clock/                # System and file-backed timex.Clock implementations (DP-8)
│   ├── core/                 # Pure shared core: timex, semver, canon (JCS), ni, did, vc, lifecycle, pkgv, codes, problem, schemas, jsonld
│   ├── dsl/                  # Profile language compiler (eunomia.profiles/v1)
│   ├── httpapi/              # Gin engine, route mounting pipeline & health probes
│   ├── observability/        # Logging (slog), metrics & telemetry
│   ├── server/               # Bounded context: alexandria-server (module.go, domain, app, config, adapters)
│   ├── ssi-auth/             # Bounded context: DID resolution, wallet port & adapters (Fafnir / IdentityHub)
│   ├── storage/              # Database repositories and migrations
│   └── testkit/              # In-memory test doubles, fake IdP, fake clock & F6 fixtures
├── spec/                     # Normative Appendix A schemas, A.5 context.jsonld, examples & requirements.tsv
├── tutorial/                 # Interactive Jupyter Notebook tutorials
├── DECISIONS.md              # DP-1..DP-14 and agent implementation decisions
├── OPEN_QUESTIONS.md         # Unreachable F5 code rationale, Q18..Q22 & spec observations
├── TRACEABILITY.md           # Auto-generated requirement, module, and negative-code traceability matrix
├── Makefile                  # Build, test, conformance, e2e, demo & trace targets
└── Taskfile.yaml             # Automation and development workflows
```

---

## License

This project is licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.
