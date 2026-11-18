# Alexandria

A **Vocabulary Hub (`alexandria-server`)**, **edge policy/DSP validation sidecar (`alexandria-client`)**, and **profile compiler (`alexandriac`)** built on the Eclipse Dataspace Protocol (**DSP 2025-1**) and **ODRL 2.2** specification.

Publish sovereign vocabularies and ODRL profiles once in readable YAML, ship cryptographically signed bundles to edge proxies, validate incoming & outgoing messages locally with zero runtime network dependencies, and roll out breaking profile changes without breaking partners overnight.

---

## One Unified Story: GreenCity Mobility (`mobility`)

To make every piece intuitive from day one, our documentation walks through **one consistent running example**—**GreenCity Mobility (`mobility`)**, a city mobility authority sharing bike and e-scooter telemetry across partner apps:

| Flow | What happens | Console Video | Guide |
|:---|:---|:---|:---|
| **1. Profile DSL (`alexandriac`)** | Author `OdrlProfile` (`mob:shareTelemetry`, `mob:region`, `odrl:dateTime`) + `SpaceProfile`, validate `N1–N5`, compile self-contained artifacts, catch `N4` invalid retirement | — | [Tutorial 1](./docs/content/docs/tutorials/01-first-profile-dsl.mdx) |
| **2. Bootstrap & Vocabularies** | Hub admin `ana` signs in with OIDC, creates `mobility`, copies **Hub DID**, appoints steward `sergio`; `sergio` uploads Turtle `mobility-vocab` | [Flow 1 MP4](./docs/public/videos/flow-01-bootstrap.mp4) | [Tutorial 2](./docs/content/docs/tutorials/02-hub-governance-lifecycle.mdx) |
| **3. Authoring Studio & Release `1.0.0`** | Author YAML in bidirectional **Form ↔ Split ↔ Code** Studio, validate `N1–N5`, propose candidate, browse compiled files & diffs, approve `1.0.0`, explore `/pub` | [Flow 2 MP4](./docs/public/videos/flow-02-authoring-release.mp4) | [Tutorial 2](./docs/content/docs/tutorials/02-hub-governance-lifecycle.mdx) |
| **4. Edge Sync, Rules & Explain** | Sidecar triggers `Sync now` (`r` / `POST /sync`), becomes **Active** immediately on `1.0.0`, inspects `verify.select: /offer`, dry-runs DSP 2025-1 `ContractRequestMessage` payloads | [Flow 3 MP4](./docs/public/videos/flow-03-client-sync-explain.mp4) | [Tutorial 3](./docs/content/docs/tutorials/03-edge-validation-sidecar.mdx) |
| **5. Breaking Change (`2.0.0`) & Grace Window** | Retire `mob:shareTelemetry` cleanly (`1.1.0 deprecated` → `2.0.0 retired`) with 30-day grace; legacy contracts pass with `VHub-Verdict: warning` (`VERDICT_NOT_IN_HEAD`) until Nov 2 cutoff | [Flow 4 MP4](./docs/public/videos/flow-04-breaking-f6.mp4) | [Tutorial 4](./docs/content/docs/tutorials/04-breaking-changes-and-f6.mdx) |
| **6. Keyboard-First Console** | Command palette (`⌘K`), live shortcuts guide (`?`), `/` filter, `↑`/`↓` + `Enter` drawer navigation, `g` go-to chords, `f`/`s`/`y` Studio modes | [Flow 5 MP4](./docs/public/videos/flow-05-keyboard-navigation.mp4) | [Web Console](./docs/content/docs/vhub/console.mdx) |

*(Every flow in the tutorials includes copy-pastable CLI/`curl` equivalents alongside the UI walkthroughs.)*

---

## Quick Start

### 1. Build binaries & Console

Requires **Go** (as pinned in [`go.mod`](./go.mod)) and, to embed the web Console (`/ui/`), **Node.js 20+ & pnpm 10+**:

```bash
# Build Go binaries only (bin/alexandriac, alexandria-server, alexandria-client, fake-idp, fake-connector)
make build

# Build Go binaries with the React Console embedded under /ui/
make build-gui
```

### 2. Run the automated Scenario F6 demo

```bash
make demo
```

Replays the full multi-month lifecycle (`1.0.0` → `2.0.0` breaking → grace window notices → cutoff → agreement revocation) on free local ports.

### 3. Run interactively in the browser

Start all four laboratory components:

```bash
./bin/fake-idp serve
./bin/alexandria-server -config config/server.yaml
./bin/fake-connector -listen 127.0.0.1:8282
# Copy the server's Hub DID into config/client.yaml (did_config.hub_did), then:
./bin/alexandria-client -config config/client.yaml
```

Then open:
- **Unified dev Console** (`make gui-dev`): `http://localhost:5273/` *(shows both **Server** and **Client** workspaces)*
- **Embedded Hub Console**: `https://127.0.0.1:8443/ui/`
- **Embedded Client Console**: `http://127.0.0.1:9090/ui/`

---

## Documentation Site (Fumadocs)

Run the full interactive documentation site locally (with embedded MP4 walkthroughs, DSL reference, and architecture docs):

```bash
make docs-dev     # http://localhost:3000
make docs-build   # Static export under docs/out/
```

---

## Tests & Quality Checks

```bash
make test          # Go unit tests (-race)
make conformance   # Normative requirement suite
make e2e           # Full F6 scenario end-to-end test
make lint          # golangci-lint
```

---

## Repository Layout

| Directory | Purpose |
|:---|:---|
| `cmd/` | Binary entrypoints (`alexandriac`, `alexandria-server`, `alexandria-client`) |
| `internal/dsl/` | `alexandria.profiles/v1` YAML parser, `N1–N5` validator, and JSON Schema / SHACL / HTML / Turtle compilers |
| `internal/server/` | Vocabulary Hub (`C-HUB`) domain, SQLite store, OAuth 2.0 auth, `/v1` & `/pub` handlers |
| `internal/client/` | Edge validation proxy (`C-CLI`), sync continuity verifier, rule matcher, Explain engine, metrics |
| `gui/` | Vite + React + TypeScript Alexandria Console (`Authoring Studio`, `/pub` browser, `Sync now`, `Explain`) |
| `docs/` | Fumadocs documentation site and walkthrough videos (`docs/public/videos/`) |
| `conformance/` | Normative `TestN_*` negative cases and `TestE2E_F6` replay |
| `deploy/` | Seed rules (`rules.seed.json`), JSON schemas, vendored contexts, and laboratory mocks |
