# Alexandria Vocabulary Hub (`alexandria-server`, `alexandria-client`, `alexandriac`)

![banner alexandria](./docs/static/banner.png)

[![ci](https://github.com/ging/alexandria/actions/workflows/ci.yaml/badge.svg)](https://github.com/ging/alexandria/actions/workflows/ci.yaml)
[![docs](https://img.shields.io/badge/docs-fumadocs-FF3366?logo=bookstack&logoColor=white)](https://ging.github.io/alexandria/)
[![go report card](https://goreportcard.com/badge/github.com/ging/alexandria)](https://goreportcard.com/report/github.com/ging/alexandria)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

> **Laboratory Environment Notice (`DP-14`):** The configurations in `config/` (`server.yaml`, `client.yaml`) and `deploy/` (`server.yaml`, `server.json`, `client.yaml`, `client.json`, `rules.seed.json`), the local OAuth 2.0/OIDC issuer `deploy/fake-idp` (`DP-4`), the simulated DSP connector `deploy/fake-connector` (`Part 5.2`), local PEM signing keys, and transient lab TLS certificates (`-hub-ca`) are for laboratory and PoC demonstration only (`PRE-01`–`PRE-07`, `DP-1`–`DP-14`) and **are not intended for production deployment**.

---

## Overview

Reference implementation of **Specification v0.3.0** (`alexandria-vhub`, F2–F8, Appendices A–C, 81 `REQ-*`, and the 77-code closed F5 catalog):

- **[`cmd/alexandria-server`](cmd/alexandria-server/main.go) (`C-HUB`):** Namespaces and stewards, OAuth 2.0/OIDC `at+jwt` authentication via JWKS (`A1`–`A5`) and the §3.5 authorization table, `hosted` and `proxy` vocabulary registry (`/pub/{ns}/vocab/{name}`), draft/candidate/release/revocation lifecycle, W3C VC 2.0 credentials signed with `eddsa-jcs-2022`, periodic `VersionListing` re-issuance, JSON-LD 1.1 `/v1` responses, and immutable `/pub` artifacts (including `/pub/_vhub/v1/context.jsonld` and `vocab.ttl`).
- **[`cmd/alexandria-client`](cmd/alexandria-client/main.go) (`C-CLI`):** L7 edge proxy in front of an unmodified DSP connector, dynamic rule set (`A.6`) with CRUD (`/rules`) and side-effect-free `:explain` (`/rules:explain`), synchronization `S1`–`S13`, proof verification `V0`–`V4` (`did:key` and `did:web`), state machine (`Onboarding`, `Active`, `Transition`, `Degraded`, `FailClosed`), classification (`ok`, `warning`, `violation`), mode actions (`observe`, `shadow`, `enforce`), `VHub-Verdict` RFC 9651 header, agreement capture and transfer admission, package garbage collection, and passive catalog audit.
- **[`cmd/alexandriac`](cmd/alexandriac/main.go):** Offline CLI compiler for the profile language (`alexandria.profiles/v1`).

---

## Build, Test & Run the F6 Demo in 3 Commands

```bash
# 1. Build binaries (bin/alexandria-server, bin/alexandria-client, bin/alexandriac, bin/fake-idp, bin/fake-connector)
make build

# 2. Run unit, hexagonal architecture, conformance (81 T-* and 76 N-*), E2E F6, and traceability checks
make test conformance e2e trace

# 3. Launch fake-idp + fake-connector + server + client with a shared file_clock and print the F6 verdict table
make demo
```

---

## `Makefile` & `Taskfile.yaml` Targets

| Target | Description |
|:---|:---|
| `make build` / `task build` | Builds `bin/alexandria-server`, `bin/alexandria-client`, `bin/alexandriac`, `bin/fake-idp`, and `bin/fake-connector`. |
| `make test` / `task test` | Runs unit, contract, and hexagonal architecture tests (`-race`). |
| `make conformance` / `task conformance` | Runs the conformance battery (`T-DOC-*`, `T-SYS-*`, `N-<CODE>` for all 76 reachable F5 codes, and `TestE2E_F6`). |
| `make e2e` / `task e2e` | Runs the end-to-end `TestE2E_F6` scenario across the four real binaries. |
| `make demo` / `task demo` | Launches the four binaries with a shared `file_clock`, replays F6.1–F6.4, and prints the 29-row verdict table. |
| `make trace` / `task trace` | Regenerates [`TRACEABILITY.md`](TRACEABILITY.md) and verifies 81/81 `REQ-*` and 76/77 F5 codes. |
| `make tutorial` / `task tutorial` | Launches the interactive JupyterLab tutorial environment (`./tutorial/run`). |
| `make docs-dev` / `task docs:dev` | Starts the Fumadocs documentation portal locally (`http://localhost:3000`). |

---

## Repository Structure

```text
alexandria/
├── cmd/
│   ├── alexandria-server/    # Vocabulary Hub server composition root (C-HUB)
│   ├── alexandria-client/    # L7 edge validation & sync proxy composition root (C-CLI)
│   └── alexandriac/          # Offline profile DSL compiler CLI
├── config/                   # server.yaml & client.yaml (common_config, auth_config, did_config, observability, vhub/client_config)
├── conformance/              # Conformance battery, negative corpus (N-*), F6 E2E, demo & trace generator
├── deploy/                   # server.yaml/.json, client.yaml/.json, rules.seed.json, fake-idp/ & fake-connector/
├── docs/                     # Documentation portal (Fumadocs + Next.js)
├── internal/
│   ├── archtest/             # Hexagonal architecture, DP-1 dependency & DP-8 clock enforcement tests
│   ├── client/               # Bounded context: alexandria-client (module.go, domain, app, config, adapters)
│   ├── clock/                # System and file-backed timex.Clock implementations (DP-8)
│   ├── core/                 # Pure shared core: timex, semver, canon (JCS), ni, did, vc, lifecycle, pkgv, codes, problem, schemas, jsonld
│   ├── dsl/                  # Profile language compiler (alexandria.profiles/v1)
│   ├── server/               # Bounded context: alexandria-server (module.go, domain, app, config, adapters)
│   └── testkit/              # In-memory test doubles, fake IdP, fake clock & F6 fixtures
├── spec/                     # Normative Appendix A schemas, A.5 context.jsonld, examples & requirements.tsv
├── tutorial/                 # Interactive Jupyter Notebook tutorials
├── DECISIONS.md              # DP-1..DP-14 and agent implementation decisions
├── OPEN_QUESTIONS.md         # Unreachable F5 code rationale, Q18..Q22 & spec observations
├── PROFILES_DSL.md           # Profile DSL specification notes
├── TRACEABILITY.md           # Auto-generated requirement, module, and negative-code traceability matrix
├── Makefile                  # Build, test, conformance, e2e, demo & trace targets
└── Taskfile.yaml             # Equivalent Task runner definitions
```
