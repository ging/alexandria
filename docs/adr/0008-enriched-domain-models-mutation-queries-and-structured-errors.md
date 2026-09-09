# 0008. Enriched domain models, mutation-and-query pattern, and structured upstream errors

Status: Accepted
Date: 2026-09-09

## Context

Following ADR 0007 (unified `wallet.Wallet` port and pluggable IdentityHub adapter), Alexandria integrates two radically different wallet implementations:
1. **Fafnir**: a lightweight sovereign wallet focused on OID4VCI and OID4VP flows, storing raw keys on disk and maintaining simple DID documents.
2. **Eclipse EDC IdentityHub**: a comprehensive dataspace identity manager featuring participant-scoped identities, rich keypair semantics (`serializedPublicKey` in PEM or JWK format, `keyContext` such as `JsonWebKey2020`, key usages, validity durations, rotation timestamps), and W3C Verifiable Credentials supporting both VC 1.0 (JWT) and VC 2.0 (JOSE/JWT) containers.

Prior to this decision, the integration suffered from three major friction points:
1. **Semantic Flattening**: The domain models (`wallet.Key`, `wallet.Credential`, `wallet.Did`) reflected only the lowest common denominator, discarding IdentityHub's rich public key serializations, key contexts, and W3C credential structures.
2. **Silent Mutating Operations**: Mutating API calls (`POST /wallet/keys/new`, `POST /wallet/did/new`, `POST /wallet/did/:did/endpoints`, `POST /wallet/keys/:id/rotate`, etc.) returned HTTP `204 No Content` or empty `201/202` responses. Callers were forced to make subsequent roundtrips or guess the resulting state.
3. **Opaque Upstream Errors**: When upstream wallets returned an HTTP error (e.g. IdentityHub returning `409 Conflict` on a duplicate service endpoint, or `400 Bad Request` with structured JSON errors), Alexandria mapped the status to a simple internal error or generic message, obscuring the upstream provider, HTTP status, endpoint path, and response body from API clients.

## Decision

### 1. Semantic Enrichment of Domain Models
The domain entities in `internal/ssi-auth/wallet/types.go` are enriched to preserve advanced cryptographic and dataspace metadata without breaking compatibility:
- **`wallet.Key`**: Extended with `SerializedPublicKey *string`, `KeyContext *string`, `Usage []string`, `DefaultPair *bool`, and `PrivateKeyAlias *string`. In Fafnir, these fields remain `nil` or empty, while IdentityHub maps them directly from `KeyPairDto`.
- **`wallet.Credential`**: Extended with `RawVc string`, `Format string`, `Credential json.RawMessage`, `Types []string`, `ParticipantContextID *string`, and `IssuanceDate *time.Time`. A unified token parser normalizes both VC 1.0 (compact JWT) and VC 2.0 (JOSE with header/payload/signature) into standard claims (`iss`, `sub`, `valid_until`, `types`).
- All REST DTOs (`keyResp`, `credentialResp`) expose these enriched fields using `omitempty` to ensure clean backward compatibility for Fafnir consumers.

### 2. Mutation-and-Query Pattern
All mutating operations across the port (`wallet.Wallet`), the domain service (`wallet.Service`), and the driving REST router (`rest.WalletRouter`) now execute a read-after-write query and return the full updated domain entity:
- Mutating port methods return `(Entity, error)` instead of only `error`.
- The adapter issues the mutation request to the upstream wallet and immediately invokes an internal query (e.g. `GetDidByID`, `GetAllKeys`, `GetParticipant`) to resolve the current state.
- If the upstream provider does not expose an immediate lookup by ID, the adapter falls back gracefully to synthesizing the confirmed entity from the initial plan.
- The REST layer responds with HTTP `200 OK` or `201 Created` accompanied by the serialized JSON representation of the mutated entity.

### 3. Structured Upstream Errors (`UpstreamError` and `OriginalResponse`)
- A specialized error type `common.UpstreamError` is introduced in `internal/common/errors.go`:
  ```go
  type UpstreamError struct {
      Provider   string
      StatusCode int
      Path       string
      RawBody    []byte
      Sentinel   error
  }
  ```
  It implements `Error()`, `Unwrap()`, and `Is(target error) bool`, preserving seamless integration with Go's `errors.Is(err, sentinel)`.
- The REST error translator (`respondError` in `internal/ssi-auth/rest/errors.go`) inspects the error chain. When an `UpstreamError` is present, it enriches the JSON response with an `originalResponse` field containing `{ provider, statusCode, path, body }` (parsing raw JSON bodies automatically).

## Consequences

- **Predictable API Responses**: Callers receive the newly created or updated key, DID, participant, or credential immediately in the response body, eliminating redundant GET requests and race conditions.
- **Superior Observability and Troubleshooting**: Clients encountering upstream rejections receive the exact upstream status and validation messages in the `originalResponse` payload, drastically cutting integration debug time.
- **Strict Hexagonal Separation Maintained**: No Eclipse EDC, Fafnir, or vendor-specific dependencies entered the domain package `internal/ssi-auth/wallet`. All transformations are cleanly handled in adapters.
- **Zero-Regressions for Fafnir**: Fields absent in Fafnir remain `nil` or empty slices and are omitted from serialized responses, preserving full backward compatibility.
