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
| **Configuration** | [One Document Model & Viper Overrides](https://ging.github.io/alexandria/docs/getting-started/configuration) |
| **Architecture** | [Hexagonal Design & Bounded Contexts](https://ging.github.io/alexandria/docs/architecture/bounded-contexts) |
| **Wallet Port** | [Unified Wallet Port & Adapters (Fafnir / IdentityHub)](https://ging.github.io/alexandria/docs/architecture/wallet-port) |
| **Remote Conformance** | [SHACL Shape Validation & Conformance Testing](https://ging.github.io/alexandria/docs/development/conformance) |
| **Authentication & IAM** | [Zitadel OIDC, PKCE & Session Security](https://ging.github.io/alexandria/docs/development/authentication) |
| **REST API Reference** | [HTTP Endpoints Catalog](https://ging.github.io/alexandria/docs/development/endpoints) |
| **Architecture Decisions** | [Accepted ADRs (0001 - 0007)](https://ging.github.io/alexandria/docs/adr) |

---

## Vocabulary Hub (`vhub-server` & `vhub-client`) — Especificación 0.2.1

Además del nodo integrado `cmd/alexandria`, el repositorio raíz contiene la implementación de referencia de **Vocabulary Hub (`vhub-server`)** y su sidecar de validación en el borde (**`vhub-client`**) conforme a la especificación *Sistema Vocabulary Hub v0.2.1* (F2–F8, Apéndices A y B).

> **Aviso de entorno de laboratorio (`DP-14`):** La configuración por defecto en `config/` (`server.json`, `config.json`, `tokens.json`, `credentials.json`) y el conector simulado `deploy/fake-connector` asumen un entorno local de laboratorio (`PRE-01` a `PRE-05`, tokens estáticos `DP-3`, resolución `did:web` en localhost sin TLS con flag explícito `DP-5`, compilador `compiler_poc` `DP-6`, evaluador `evaluator_poc` `DP-7` y registro de transparencia simulado `DP-8`) y **no constituyen un despliegue de producción**. Los módulos `internal/server/module.go` e `internal/client/module.go` exponen los puertos para inyectar los adaptadores de identidad y firma de `internal/ssi-auth` e `internal/auth-proxy`.

### Ejecución rápida y Demo F6 en 3 comandos

```bash
# 1. Compilar binarios (bin/vhub-server, bin/vhub-client, bin/fake-connector)
make build

# 2. Ejecutar suite unitaria, arquitectura, conformidad (T-* y N-*) y trazabilidad
make test conformance trace

# 3. Ejecutar el escenario extremo a extremo F6 con reloj compartido e imprimir la tabla de veredictos
make demo
```

### Objetivos del `Makefile`

| Comando | Descripción |
|:---|:---|
| `make build` | Compila `bin/vhub-server`, `bin/vhub-client` y `bin/fake-connector`. |
| `make test` | Ejecuta las pruebas unitarias, de contrato y de arquitectura (`internal/...`, `cmd/...`). |
| `make conformance` | Ejecuta la batería de conformidad (`T-DOC-*`, `T-SYS-*`, casos negativos `N-<CÓDIGO>` de F5 y `TestE2E_F6`). |
| `make e2e` | Ejecuta únicamente el test extremo a extremo `TestE2E_F6` levantando los tres binarios reales. |
| `make demo` | Levanta `fake-connector`, `vhub-server` y `vhub-client`, recorre F6 e imprime la tabla de veredictos en `stdout`. |
| `make trace` | Ejecuta `go run ./conformance/trace` y regenera [`TRACEABILITY.md`](TRACEABILITY.md) verificando los 61 `REQ-*` y el catálogo F5. |

Documentos de trazabilidad y diseño en la raíz:
- [`TRACEABILITY.md`](TRACEABILITY.md): Matriz generada automáticamente `REQ-* → Sección → Módulo(s) → Prueba(s) T-*` y cobertura `N-<CÓDIGO>`.
- [`DECISIONS.md`](DECISIONS.md): Decisiones `DP-1` a `DP-15` y decisiones técnicas de integración en la raíz del repositorio.
- [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md): Estado de las cuestiones abiertas `Q1`–`Q17` de F7 y justificación del único código F5 no alcanzable en la PoC (`VERDICT_SHACL_WARNING` por `DP-7`).

---

## Repository Structure

```text
alexandria/
├── cmd/
│   ├── alexandria/         # Identity & Auth gateway composition root
│   ├── vhub-server/        # Vocabulary Hub server composition root (C-HUB)
│   └── vhub-client/        # Edge validation & sync sidecar composition root (C-CLI)
├── config/                 # Configuration files (config.yaml, server.json, config.json, tokens.json, drafts/)
├── conformance/            # Schemas (Appendix A), examples, negative corpus (N-*), F6 E2E & trace generator
├── deploy/                 # Docker Compose, Helm charts & fake-connector for E2E/demo
├── docs/                   # Documentation portal (Fumadocs + Next.js)
├── internal/
│   ├── archtest/           # Hexagonal architecture, DP-1 dependency & DP-9 clock enforcement tests
│   ├── auth-proxy/         # Bounded context: Zitadel OIDC guard & session handling
│   ├── client/             # Bounded context: vhub-client (module.go, domain, app, config, adapters)
│   ├── clock/              # System and file-backed timex.Clock implementations (DP-9)
│   ├── core/               # Pure shared core: timex, semver, canon (JCS), ni, did, sign, lifecycle, pkgv, codes, problem, schemas
│   ├── httpapi/            # Gin engine, route mounting pipeline & health probes
│   ├── observability/      # Logging (slog), metrics & telemetry
│   ├── server/             # Bounded context: vhub-server (module.go, domain, app, config, adapters)
│   ├── ssi-auth/           # Bounded context: DID resolution, wallet port & adapters (Fafnir / IdentityHub)
│   ├── storage/            # Database repositories and migrations
│   └── testkit/            # In-memory test doubles, fake clock & F6 fixtures
├── DECISIONS.md            # DP-1..DP-15 and implementation decisions
├── OPEN_QUESTIONS.md       # F7 Q1..Q17 status and unreachable F5 code rationale
├── TRACEABILITY.md         # Auto-generated requirement, module, and negative-code traceability matrix
├── Makefile                # Build, test, conformance, e2e, demo & trace targets
└── Taskfile.yaml           # Automation and development workflows
```

---

## License

This project is licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.
