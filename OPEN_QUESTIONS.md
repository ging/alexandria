# OPEN_QUESTIONS

This document records:
1. The single F5 catalog code that is unreachable in the PoC (Part 6 DoD item 3).
2. The status of F7.2 open questions (`Q18`–`Q22`) in this PoC.
3. Specification ambiguities and schema/context observations found during v0.3.0 implementation (Part 7).

---

## 1. Unreachable F5 Catalog Code in the PoC (DoD Item 3)

Out of the **77 codes** in the closed F5 catalog (`internal/core/codes/catalog.tsv` / `internal/core/codes/codes.go`), **76 codes** have an automated minimal negative case `N-<CODE>` in `conformance/negative_test.go` and `conformance/negative_helpers_test.go`.

The single unreachable code in the PoC is:

| Code | Emitter | Channel | Severity | Requirement | Why It Is Unreachable in the PoC |
| --- | --- | --- | --- | --- | --- |
| `VERDICT_SHACL_WARNING` | C-CLI | classification | `info` | `REQ-CLI-011` | Per **DP-7**, `evaluator_poc` (`internal/client/adapters/evaluator_poc`) only evaluates `json-schema` artifacts (whose failures always have `Violation` severity under §4.7 step 3) and skips `shacl` and `context` artifacts with an `Info` finding of code `POC_EVAL_UNSUPPORTED_ROLE`. Because `Warning` findings only come from SHACL shapes (`sh:Warning`), the `r.shacl_warnings → VERDICT_SHACL_WARNING` branch in `internal/client/domain/classify.go` is verified by unit tests (`TestT_CLI_011_ClassificationTable`), but `evaluator_poc` never produces `shacl` warning findings end-to-end. |

---

## 2. F7.2 Open Questions (`Q18`–`Q22`)

None of `Q18`–`Q22` blocks v1 (F7.2). Their treatment in this PoC is:

| ID | Question in F7.2 | Blocks v1? | Treatment in This PoC |
| --- | --- | --- | --- |
| **Q18** | Register `https://w3id.org/alexandria-vhub/v1` on w3id.org or switch to project domain; meanwhile use self-hosted option (§3.4) | No | **DP-9**: `rdf.context = "self-hosted"` in `deploy/server.json`; `alexandria-server` serves `/pub/_vhub/v1/context.jsonld` (Appendix A.5 byte-for-byte) and `/pub/_vhub/v1/vocab.ttl`, and `alexandria-client` sets `trust.context` to that URL (`REQ-HUB-030`, `REQ-CLI-034`). |
| **Q19** | Sending `"providerPid": null` in `ContractNegotiationError` and `TransferError` when the incoming request has no `providerPid` (Q4, §4.8) | No | Implemented per §4.8 (`"providerPid": null` when absent in the incoming request body, `internal/client/domain/verdict.go`, `REQ-CLI-013`). |
| **Q20** | Four-eyes rule: requiring the approver of a candidate to differ from its proposer (R17, §3.5) | No | Any steward of the namespace may propose and approve candidates (`R17`, `internal/server/app/release.go`, `REQ-HUB-003`). |
| **Q21** | Expiration/TTL for entries in the client agreement registry (§4.10) | No | Agreement records are retained indefinitely in SQLite (`internal/client/adapters/store_sqlite`, `REQ-CLI-015`, `REQ-CLI-017`). |
| **Q22** | Allowlist of hosts for `proxy` vocabularies to prevent SSRF into internal networks (§3.6, Appendix B) | No | Implemented via `vocab.proxy_allow` and the test-only `vocab.allow_insecure_localhost` option in `server.json` (`DP-11`, `DP-13`, `internal/server/app/vocabularies.go`, `internal/server/adapters/fetch_http`). |

---

## 3. Specification Observations Detected During Implementation (Part 7)

### G9 — Appendix A.5 JSON-LD Context vs. W3C VC 2.0 Protected Terms (`iss` and `sub`)
- **Where:** §3.4, §3.8, Appendix A.1, A.2, A.5, A.7 (`REQ-HUB-029`, `REQ-HUB-030`, `REQ-HUB-031`).
- **Issue:** Signed credentials (`VersionListing`, `VersionRecord`, `PackageManifest`) carry `@context: ["https://www.w3.org/ns/credentials/v2", C]`. The W3C VC 2.0 context (`https://www.w3.org/ns/credentials/v2`) sets `@protected: true` at root and defines `iss` and `sub` as protected terms mapped to `https://www.iana.org/assignments/jose#iss` and `#sub` (with `@type: @id`). Meanwhile, Appendix A.5 (`context.jsonld`) defines `"iss": "avh:iss"` and `"sub": "avh:sub"` at top level (used on `Steward` resources). When a strict JSON-LD 1.1 processor (`json-gold`) expands a credential with `["https://www.w3.org/ns/credentials/v2", C]`, it rejects the second context with `protected term redefinition` on `iss` and `sub`.
- **Handling in PoC (Part 7 rule 4):** Per Part 7 rule 4 (*never modify appendices or schemas*), `spec/schemas/context.jsonld` is embedded and served byte-for-byte (`REQ-HUB-030`). All non-credential `/v1` responses expand with the unmodified A.5 context (`TestT_HUB_029_ResponsesExpandAsJSONLD`), and `TestT_HUB_029_CredentialsConflictOnlyOnIssAndSub` in `internal/server/adapters/http_v1/handler_test.go` verifies both the protected-term conflict and that credentials expand cleanly once `iss`/`sub` are scoped to `Steward` or omitted from credential contexts.
- **Recommended spec fix:** In Appendix A.5, scope `iss` and `sub` inside a type-scoped context on `Steward` (`"Steward": {"@id": "avh:Steward", "@context": {"iss": "avh:iss", "sub": "avh:sub"}}`) or rename the JSON properties on `Steward` to `issuer` / `subject`.

### G10 — F6.3 Lease-Margin Row (`2026-11-02T10:00:05Z`) and §3.8 `VersionListing` Membership
- **Where:** §3.8 (`REQ-HUB-009`), §4.6 (`REQ-CLI-009`), and F6.3 (`2026-11-02T10:00:05Z`).
- **Issue:** By §3.8, a version belongs to `versions[]` while its reference state is `Scheduled`, `Published`, or `Deprecated`, or when `revoked_from != null`. At `enforced_from(2.0.0) = 2026-11-02T10:00:00Z`, version `1.0.0` becomes `Retired` in reference time, so any listing reissued by the hub in `[enforced_from, enforced_from + L)` already omits `1.0.0`. Meanwhile, F6.3 row 4 (`2026-11-02T10:00:05Z`, with `L = 10s`) expects local `V_t = {2.0.0, 1.0.0}` because the client's periodic sync happened before the barrier (e.g. `2026-11-02T06:00:00Z` under `sync_interval = PT6H`).
- **Handling in PoC:** In `TestT_SYS_007_F6Rows` and `conformance/f6/scenario.go`, the client syncs before each barrier as in F6 (`2026-11-02T06:00:00Z`), reproducing all F6.3 rows identically.
- **Recommended spec clarification:** Either note in §3.8 that `versions[]` retains a newly `Retired`/`Superseded` version until `enforced_from + lease_margin + clock_skew`, or note in §4.4 that the client keeps previously known non-revoked versions in its local snapshot until `enforced_from + L`.

### G11 — `/pub` Route Collision with Namespaces Named `packages` or `blobs`

- **Where:** §3.2 (slug pattern, only `_vhub` reserved) and §3.10 (`/pub/packages/{digest}/manifest.json`, `/pub/blobs/sha256/{hex}`, `/pub/{ns}/{version}/{path}`).
- **Issue:** The slugs `packages` and `blobs` are valid, so `/pub/packages/{x}/manifest.json` is ambiguous between a package manifest and the file `manifest.json` of version `{x}` of namespace `packages`.
- **Handling in PoC:** `http_pub` routes the fixed prefixes first, so artefacts of namespaces named `packages` or `blobs` at those exact shapes are shadowed. No code is invented to reject the slugs.
- **Options:** reserve `packages` and `blobs` next to `_vhub` (preferred, reusing `NS_EXISTS` as for `_vhub`), or move the content-addressed routes under `/pub/_vhub/`.

### G12 — Who Reads the Events of a Namespace (§3.5 vs. §3.9)

- **Where:** §3.5 authorization table (row "…revocaciones de ns; eventos de ns": admin is 403 unless also steward) and §3.9 (`GET /v1/namespaces/{ns}/events`: "steward de ns o admin").
- **Handling in PoC:** the §3.5 table is applied, as §3.9 says its "Quién" column follows that table: only stewards of `ns` read its events; the admin reads global events at `GET /v1/events`.
- **Options:** align §3.9 with §3.5 (preferred), or move "eventos de ns" to the "Listar los stewards de ns" row of §3.5.
