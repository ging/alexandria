# Alexandria Vocabulary Hub (`alexandria-server`, `alexandria-client`, `alexandriac`)

![banner alexandria](./docs/static/banner.png)

[![ci](https://github.com/ging/alexandria/actions/workflows/ci.yaml/badge.svg)](https://github.com/ging/alexandria/actions/workflows/ci.yaml)
[![docs](https://img.shields.io/badge/docs-fumadocs-FF3366?logo=bookstack&logoColor=white)](https://ging.github.io/alexandria/)
[![go report card](https://goreportcard.com/badge/github.com/ging/alexandria)](https://goreportcard.com/report/github.com/ging/alexandria)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

> **Laboratory Environment Notice (`DP-14`):** The configurations in `config/` (`server.yaml`, `client.yaml`) and `deploy/` (`server.yaml`, `server.json`, `client.yaml`, `client.json`, `rules.seed.json`), the local OAuth 2.0/OIDC issuer `deploy/fake-idp` (`DP-4`), the simulated DSP connector `deploy/fake-connector` (`Part 5.2`), local PEM signing keys, and transient lab TLS certificates (`-hub-ca`) are for laboratory and PoC demonstration only (`PRE-01`–`PRE-07`, `DP-1`–`DP-14`) and **are not intended for production deployment**.

---

## Welcome to Alexandria

Alexandria helps dataspace authorities and participants speak the same policy and vocabulary dialect—without fragile manual syncing of JSON Schemas, SHACL shapes, and JSON-LD contexts across every connector.

Author your policies once in readable **`alexandria.profiles/v1`** YAML, publish cryptographically signed releases from the **Vocabulary Hub (`C-HUB`)**, and let every **`alexandria-client` (`C-CLI`)** sidecar verify incoming & outgoing **DSP 2025-1** and **ODRL 2.2** traffic locally with zero network lookups on the request path.

Reference implementation of **Specification v0.3.0** (`alexandria-vhub`, F2–F8, Appendices A–C, 81 `REQ-*`, and the 77-code closed F5 catalog):

- **[`cmd/alexandria-server`](cmd/alexandria-server/main.go) (`C-HUB`):** Namespaces and stewards, OAuth 2.0/OIDC `at+jwt` authentication via JWKS (`A1`–`A5`) and the §3.5 authorization table, `hosted` and `proxy` vocabulary registry (`/pub/{ns}/vocab/{name}`), draft/candidate/release/revocation lifecycle, W3C VC 2.0 credentials signed with `eddsa-jcs-2022`, periodic `VersionListing` re-issuance, JSON-LD 1.1 `/v1` responses, immutable `/pub` artifacts (including `/pub/_vhub/v1/context.jsonld` and `vocab.ttl`), and optional embedded Console UI (`/ui/`).
- **[`cmd/alexandria-client`](cmd/alexandria-client/main.go) (`C-CLI`):** L7 edge proxy (or Envoy ext_authz validator) in front of an unmodified DSP connector, dynamic rule set (`A.6`) with CRUD (`/rules`), scoped `verify.select` (`Policy path`), immediate `POST /sync`, side-effect-free `:explain` (`/rules:explain`), synchronization `S1`–`S13`, proof verification `V0`–`V4` (`did:key` and `did:web`), state machine (`Onboarding`, `Active`, `Transition`, `Degraded`, `FailClosed`), `VHub-Verdict` RFC 9651 notices, agreement capture and transfer admission, package GC, and passive catalog audit.
- **[`cmd/alexandriac`](cmd/alexandriac/main.go):** Offline CLI compiler and `N1`–`N5` validator for `alexandria.profiles/v1` YAML bundles.

---

## One Unified Walkthrough: GreenCity Mobility (`mobility`)

Rather than scattering disconnected snippets, our **[Fumadocs Portal](https://ging.github.io/alexandria/)** follows **one single running example** from YAML authoring all the way through a live breaking upgrade: **GreenCity Mobility (`mobility`)**, a city mobility authority sharing bike & e-scooter telemetry across partner apps.

Every flow comes with both **Console UI** + **CLI / `curl`** instructions and a **dark-mode browser walkthrough video**:

| Flow | What you'll learn | Video Walkthrough | Tutorial / Guide |
|:---|:---|:---|:---|
| **1. Profile DSL (`alexandriac`)** | Author `OdrlProfile` (`mob:shareTelemetry`, `mob:region`, `odrl:dateTime`) + `SpaceProfile` (`dsp =2025-1` vendored), validate `N1–N5`, compile self-contained artifacts, catch `N4` invalid retirement | — | [Tutorial 1](./docs/content/docs/tutorials/01-first-profile-dsl.mdx) |
| **2. Bootstrap & Vocabularies** | Hub admin `ana` signs in via OIDC, creates `mobility`, copies **Hub DID**, appoints steward `sergio`; `sergio` uploads Turtle `mobility-vocab` | [Flow 1 MP4](./docs/public/videos/flow-01-bootstrap.mp4) | [Tutorial 2](./docs/content/docs/tutorials/02-hub-governance-lifecycle.mdx) |
| **3. Authoring Studio & Release `1.0.0`** | Author YAML in bidirectional **Form ↔ Split ↔ Code** Studio, validate `N1–N5`, propose candidate, browse compiled files & diffs, approve `1.0.0` (`initial`), explore `/pub` | [Flow 2 MP4](./docs/public/videos/flow-02-authoring-release.mp4) | [Tutorial 2](./docs/content/docs/tutorials/02-hub-governance-lifecycle.mdx) |
| **4. Edge Sync, Rules & Explain** | Sidecar triggers `Sync now` (`r` / `POST /sync`), becomes **Active** immediately on `1.0.0`, inspects `verify.select: "/offer"`, dry-runs valid & invalid DSP 2025-1 requests | [Flow 3 MP4](./docs/public/videos/flow-03-client-sync-explain.mp4) | [Tutorial 3](./docs/content/docs/tutorials/03-edge-validation-sidecar.mdx) |
| **5. Breaking Change (`2.0.0`) & F6 Grace Window** | Retire `mob:shareTelemetry` cleanly (`1.1.0 deprecated` → `2.0.0 retired`) with 30-day grace; legacy contracts pass with `VHub-Verdict: warning` (`VERDICT_NOT_IN_HEAD`) until Nov 2 cutoff | [Flow 4 MP4](./docs/public/videos/flow-04-breaking-f6.mp4) | [Tutorial 4](./docs/content/docs/tutorials/04-breaking-changes-and-f6.mdx) |
| **6. Keyboard-First Console** | Command palette (`⌘K`), shortcuts guide (`?`), `/` filter, `↑`/`↓` + `Enter` drawer inspection, `g` navigation chords, `f`/`s`/`y` Studio modes | [Flow 5 MP4](./docs/public/videos/flow-05-keyboard-navigation.mp4) | [Web Console](./docs/content/docs/vhub/console.mdx) |

---

## Build, Test & Run in 3 Commands

```bash
# 1. Build Go binaries (or use `make build-gui` to also embed the React Console under /ui/)
make build

# 2. Run unit, hexagonal architecture, conformance (81 T-* and 76 N-*), E2E F6, and traceability checks
make test conformance e2e trace

# 3. Launch fake-idp + fake-connector + server + client with a shared file_clock and print the F6 verdict table
make demo
```

### Interactive Lab with the Web Console

To click through the Console yourself (requires Node.js 20+ & `pnpm 10+` if embedding `/ui/`):

```bash
make build-gui
./bin/fake-idp serve
./bin/alexandria-server -config config/server.yaml
./bin/fake-connector -listen 127.0.0.1:8282
# Copy the server's Hub DID into config/client.yaml (did_config.hub_did), then:
./bin/alexandria-client -config config/client.yaml
```

- **Unified dev Console** (`make gui-dev`): `http://localhost:5273/` *(both **Server** and **Client** workspaces in one tab, `g w` to switch)*
- **Embedded Consoles** (`make build-gui`): Hub at `https://127.0.0.1:8443/ui/` · Client at `http://127.0.0.1:9090/ui/`

---

## `Makefile` & `Taskfile.yaml` Targets

| Target | Description |
|:---|:---|
| `make build` / `task build` | Builds `bin/alexandria-server`, `bin/alexandria-client`, `bin/alexandriac`, `bin/fake-idp`, and `bin/fake-connector`. |
| `make build-gui` | Builds `gui/` and embeds the Web Console under `/ui/` in both `alexandria-server` and `alexandria-client`. |
| `make test` / `task test` | Runs unit, contract, and hexagonal architecture tests (`-race`). |
| `make conformance` / `task conformance` | Runs the conformance battery (`T-DOC-*`, `T-SYS-*`, `N-<CODE>` for all 76 reachable F5 codes, and `TestE2E_F6`). |
| `make e2e` / `task e2e` | Runs the end-to-end `TestE2E_F6` scenario across the four real binaries. |
| `make demo` / `task demo` | Launches the four binaries with a shared `file_clock`, replays F6.1–F6.4, and prints the 29-row verdict table. |
| `make trace` / `task trace` | Regenerates [`TRACEABILITY.md`](TRACEABILITY.md) and verifies 81/81 `REQ-*` and 76/77 F5 codes. |
| `make tutorial` / `task tutorial` | Launches the interactive JupyterLab tutorial environment (`./tutorial/run`). |
| `make gui-dev` / `task gui:dev` | Runs the standalone Console dev server with hot reload (`http://localhost:5273`). |
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
├── deploy/                   # server.yaml/.json, client.yaml/.json, rules.seed.json, vendor/, fake-idp/ & fake-connector/
├── docs/                     # Documentation portal (Fumadocs + Next.js) & walkthrough MP4s (docs/public/videos/)
├── examples/                 # Deployment examples: client as L7 proxy (proxy/) and Envoy + client as validator (envoy/)
├── gui/                      # React + TypeScript + Vite Alexandria Console (embedded via -tags gui)
├── internal/
│   ├── archtest/             # Hexagonal architecture, DP-1 dependency & DP-8 clock enforcement tests
│   ├── client/               # Bounded context: alexandria-client (module.go, domain, app, config, adapters)
│   ├── clock/                # System and file-backed timex.Clock implementations (DP-8)
│   ├── core/                 # Pure shared core: timex, semver, canon (JCS), ni, did, vc, lifecycle, pkgv, codes, problem, schemas, jsonld
│   ├── dsl/                  # Profile language compiler (alexandria.profiles/v1)
│   ├── server/               # Bounded context: alexandria-server (module.go, domain, app, config, adapters)
│   ├── testkit/              # In-memory test doubles, fake IdP, fake clock & F6 fixtures
│   └── webui/                # Embedded Console HTTP handler and built asset bundle
├── spec/                     # Normative Appendix A schemas, A.5 context.jsonld, examples & requirements.tsv
├── tutorial/                 # Interactive Jupyter Notebook tutorials & LaTeX practical manual
├── DECISIONS.md              # DP-1..DP-14 and agent implementation decisions
├── OPEN_QUESTIONS.md         # Unreachable F5 code rationale, Q18..Q22 & spec observations
├── PROFILES_DSL.md           # Profile DSL specification notes
├── TRACEABILITY.md           # Auto-generated requirement, module, and negative-code traceability matrix
├── Makefile                  # Build, test, conformance, e2e, demo, gui & docs targets
└── Taskfile.yaml             # Equivalent Task runner definitions
```
