# 0007. Unified wallet port and pluggable IdentityHub adapter

Status: Accepted
Date: 2026-09-07

## Context

ADR 0003 established that key material lives in an external wallet and that the
node interacts with it via an adapter behind a port owned by `internal/ssi-auth/wallet`.
Initially, Fafnir was the sole implementation.

In modern European and international dataspaces (e.g. Gaia-X, Catena-X, Eclipse
Dataspace Components ecosystem), participants often rely on Eclipse EDC
IdentityHub rather than Fafnir. IdentityHub exposes a modular JAX-RS REST API
split across multiple HTTP contexts and ports (Identity API, Issuer Admin API,
STS, DCP Presentation/Issuance) and features participant-scoped DID management,
credential stores, and key rotation.

The node needs to run against either Fafnir or Eclipse EDC IdentityHub without
changing the core domain logic, without leaking provider-specific transport details
into callers, and without fragmenting the HTTP API that Alexandria exposes.

## Decision

The `wallet.Wallet` interface in `internal/ssi-auth/wallet/ports.go` acts as the
single unified port for all wallet capabilities. Both Fafnir and IdentityHub
are implemented as pluggable driven adapters satisfying this common contract.

Selection is determined declaratively by `wallet_config.provider` in
`config/config.yaml` (`"fafnir"` or `"identityhub"`). The module factory
(`internal/ssi-auth/module.go`) instantiates the chosen adapter at startup,
preserving identical lifecycle, startup timeout, and readiness probe behaviors.

Capabilities supported by only one provider return a typed domain sentinel
`common.ErrNotImplemented` (e.g., key rotation in Fafnir, or OID4VCI/OID4VP
issuance flows in basic IdentityHub), which the REST driving adapter translates
to HTTP 501 Not Implemented with descriptive JSON error payloads.

Integration tests for both adapters are guarded by the `//go:build integration`
tag and perform runtime readiness probes against live services, skipping
gracefully (`t.Skip`) if the target container is not reachable.

## Consequences

- **Extensibility**: Adding new wallet providers (e.g., Walt.id, SpruceID, or
  custodial enterprise HSMs) only requires implementing `wallet.Wallet`.
- **Domain Independence**: No provider-specific imports (`fafnir`, `identityhub`,
  or EDC models) enter `internal/ssi-auth/wallet` or the driving HTTP router.
- **Surface Asymmetry**: Because Fafnir and IdentityHub do not have 100% feature
  parity, some endpoints return 501 depending on the configured backend. This
  trade-off is made explicit to callers via structured error responses rather
  than silent no-ops or partial implementations.
- **Testing Requirements**: Verifying changes against live implementations
  requires running containerized instances (`task identityhub:up` /
  `task wallet:up`). The integration test suite is isolated from default unit
  test runs to keep `task test` fast and deterministic.

## What would change this

A dataspace standardization process unifying wallet management APIs under a
single universal RFC or W3C standard, rendering provider-specific adapters
obsolete.
