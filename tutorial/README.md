# Interactive Jupyter Notebook Tutorials — Alexandria (`alexandria-server`, `alexandria-client`, `alexandriac`)

This directory contains **5 incremental Jupyter Notebooks** (`.ipynb`) that walk end to end through all flows of **Alexandria** (`alexandria-server`, `alexandria-client`, and `alexandriac`) by sending **real HTTP requests** against the locally compiled Go binaries.

Every notebook comes with **pre-captured execution outputs** (`stdout`, interactive HTML widgets, HTTP status codes, `VHub-Verdict` headers, JCS + Ed25519 signed manifests, and JSONL events), so you can browse them directly or re-run them cell by cell.

---

## Incremental Notebook Series

| # | Notebook | Flows & HTTP Requests Covered |
| :- | :--- | :--- |
| **01** | [`01_dsl_and_alexandriac_compiler.ipynb`](./01_dsl_and_alexandriac_compiler.ipynb) | Declarative **`alexandria.profiles/v1`** profile DSL (`OdrlProfile` and `SpaceProfile`), 5-level validation (`N1`–`N5`), deterministic compilation of all artifacts with `bin/alexandriac`, and term evolution (`active -> deprecated -> retired`). |
| **02** | [`02_alexandria_server_governance_and_pub.ipynb`](./02_alexandria_server_governance_and_pub.ipynb) | Governance API (`/v1/*`) and Immutable Distribution (`/pub/*`) on **`alexandria-server`**: `GET /.well-known/did.json` identity, Bearer authentication and role matrix (`401`/`403`), drafts (`POST/PUT /v1/drafts`) with `ETag`/`If-Match` (`428`/`412`), candidate proposal (`:propose`), signed release approval (`:approve`), `GET /v1/versions` listing, `303 See Other` redirect on `/pub/demo`, and `GET /v1/events` audit log. |
| **03** | [`03_alexandria_client_sync_and_dsp_proxy.ipynb`](./03_alexandria_client_sync_and_dsp_proxy.ipynb) | Sidecar **`alexandria-client`**: verified synchronization protocol **`S1`–`S13`**, Admin API (`GET /status`, `GET /versions`, `GET /metrics`), L7 hot-path validation of DSP messages (`ContractRequestMessage`, `ContractOfferMessage`), comparison of `enforce`, `shadow`, and `observe` modes, blocking external `@context` URIs (`CONTEXT_NOT_IN_PACKAGE`), and `vhub.verdict` event emission. |
| **04** | [`04_f6_lifecycle_transitions_agreements_and_revocation.ipynb`](./04_f6_lifecycle_transitions_agreements_and_revocation.ipynb) | **Full Normative Scenario F6** with deterministic time travel (`clock_file`, `DP-9`) from September to November 2026: `1.0.0 -> 2.0.0` transition, propagation window (`Announced`), grace window (`Transition`) with RFC 9651 `VHub-Verdict: warning;...;sunset=@1793613600`, `urn:uuid:A1` agreement capture on `dsp-egress`, `lease_margin`, agreement continuity after retirement (`Retired`), and transfer blocking after revocation (`AGREEMENT_REVOCATION_SCHEDULED` -> `AGREEMENT_VERSION_REVOKED`). |
| **05** | [`05_resilience_security_and_negative_cases.ipynb`](./05_resilience_security_and_negative_cases.ipynb) | Closed error catalog **F5 (`N-<CODE>`)**: 9-row approval table invariants (`DATES_GRACE_ON_COMPATIBLE`, `DATES_EMERGENCY_ON_COMPATIBLE`, `DATES_ZERO_GRACE_WITHOUT_EMERGENCY`, `DATES_JUSTIFICATION_REQUIRED`, `DATES_EMERGENCY_WITH_GRACE`), revocation invariants (`REV-1` to `REV-4`), and client operational state machine (`Active -> Degraded -> FailClosed` with `CLIENT_FAIL_CLOSED`) during prolonged Hub outages. |

---

## Local Lab Architecture ([`lab.py`](./lab.py))

The helper module [`lab.py`](./lab.py) is written **100% in the Python 3 standard library** (`urllib.request`, `subprocess`, `socket`, `json`, `tempfile`), so **no external packages** (`requests`, etc.) are required to execute code cells:

- **`ensure_binaries()`**: Automatically compiles `bin/alexandriac`, `bin/alexandria-server`, `bin/alexandria-client`, and `bin/fake-connector` if not already built.
- **`AlexandriaLab`**: Orchestrates real `alexandria-server`, `alexandria-client`, and `fake-connector` processes on free local TCP ports, sharing a clock file (`clock_file`, `DP-9`) for deterministic time travel via `lab.set_clock("2026-10-10T12:00:00Z")`.
- **`http_request(method, url, ...)`**: Executes real HTTP requests (including forward-proxy support for `dsp-egress`) and renders interactive HTML widgets displaying the method, key headers (`ETag`, `Cache-Control`, `Location`, `VHub-Verdict`, `WWW-Authenticate`), and syntax-highlighted JSON payloads.

---

## How to Run the Notebooks

### 1. Launch JupyterLab with a Single Command ([`./tutorial/run`](./run))
Automatically builds the Go binaries (`bin/alexandriac`, `bin/alexandria-server`, `bin/alexandria-client`, `bin/fake-connector`), creates `tutorial/.venv` (if needed), installs `jupyterlab`, and opens your browser directly in `tutorial/`:

```bash
./tutorial/run
# or from the repository root with Make or Task:
make tutorial
task tutorial
```

### 2. Execute and Verify All Notebooks Headlessly in the Terminal
Executes every code cell of the notebooks against the real Go binaries and updates the outputs in each `.ipynb` file in place:

```bash
make tutorial-exec
# or with Task:
task tutorial:exec
# or filtering a specific notebook (e.g., 04):
./tutorial/run.py exec -f 04 -v
```

### 3. Directly in VS Code / Cursor / GoLand / JetBrains
1. Open any `.ipynb` file in this `tutorial/` directory.
2. Select any system **Python 3.10+** interpreter (or `tutorial/.venv/bin/python`).
3. Click **Run All**.
