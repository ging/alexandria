# Matriz de Trazabilidad (`REQ-*` → Sección → Módulo → Prueba `T-*` y Catálogo `N-*`)

Generado automáticamente por `go run ./conformance/trace`.

## Resumen de cobertura

- **Requisitos normativos (F2, F3, F4):** 61 / 61 cubiertos con módulos y pruebas `TestT_*`.
- **Paquetes de producción verificados (`cmd/` + `internal/`):** 45 paquetes con referencia explícita a `REQ-*`.
- **Códigos del catálogo cerrado F5:** 63 / 64 cubiertos con casos negativos `N-<CÓDIGO>` (1 inalcanzable en la PoC por DP-7 y documentado en `OPEN_QUESTIONS.md`).

## 1. Trazabilidad Requisito → Sección → Módulo(s) → Prueba(s) `T-*`

| Requisito | Sección | Módulo(s) | Prueba(s) `T-*` |
| --- | --- | --- | --- |
| `REQ-SYS-001` | §2.11, §4.6, §4.7, §4.9 | `internal/client/app`, `internal/client/domain`, `internal/clock`, `internal/core/jsonld`, `internal/core/semver`, `internal/core/timex` | `TestT_SYS_001_TwoClientsSameOutputs (internal/client/app/edge_test.go)` |
| `REQ-SYS-002` | §2.11, §4.7 | `cmd/vhub-client`, `internal/client`, `internal/client/adapters/http_hub`, `internal/client/app` | `TestT_SYS_002_EdgeHasNoHubAccess (internal/client/app/edge_test.go)` |
| `REQ-SYS-003` | §2.11, §4.2, §4.8 | `internal/client/adapters/proxy_dsp`, `internal/client/app` | `TestT_SYS_003_RequestForwardedByteForByte (internal/client/adapters/proxy_dsp/proxy_test.go)` |
| `REQ-SYS-004` | §2.11, §3.3, §4.6 | `internal/core/lifecycle` | `TestT_SYS_004_FrontiersComeFromListing (internal/client/app/edge_test.go)` |
| `REQ-SYS-005` | §2.11, §3.7 | `internal/server/adapters/blob_fs`, `internal/server/adapters/http_pub`, `internal/server/adapters/sqlite`, `internal/server/app` | `TestT_SYS_005_PubBytesNeverChange (conformance/documents_test.go)` |
| `REQ-SYS-006` | §2.11, §3.7, §4.9 | `internal/client/adapters/store_fs`, `internal/server/adapters/sqlite` | `TestT_SYS_006_AgreementVersionStaysObtainable (conformance/documents_test.go)` |
| `REQ-SYS-007` | §2.11, §3.3, §4.6, §4.7 | `internal/client/domain`, `internal/core/lifecycle` | `TestT_SYS_007_VersionRejectsOnlyAfterEnforcedPlusLease (internal/client/app/edge_test.go)`, `TestT_SYS_007_ContractOnlyAfterEnforcedPlusLease (internal/core/lifecycle/lifecycle_test.go)` |
| `REQ-SYS-008` | §2.11, §4.5 | `internal/client/domain` | `TestT_SYS_008_FailClosedRejectsEverything (internal/client/app/edge_test.go)` |
| `REQ-SYS-009` | §2.11, §3.4, §4.4.1 | `internal/core/did`, `internal/core/sign` | `TestT_SYS_009_UnpublishedKeyNeverTrusted (internal/client/app/sync_test.go)` |
| `REQ-HUB-001` | §2.10, §3.8 | `internal/core/lifecycle`, `internal/server/adapters/compiler_dsl`, `internal/server/adapters/compiler_poc`, `internal/server/app` | `TestT_HUB_001_NegativeDraftCorpus (internal/server/app/app_test.go)`, `TestT_HUB_001_N4AndN5OnPropose (internal/server/app/app_test.go)` |
| `REQ-HUB-002` | §3.6, §3.8 | `internal/server/app`, `internal/server/domain` | `TestT_HUB_002_CandidateImmutableAfterDraftEdit (internal/server/app/app_test.go)`, `TestT_HUB_002_ApproveTwiceInvalidState (internal/server/app/app_test.go)` |
| `REQ-HUB-003` | §3.6, §3.8 | `internal/server/app`, `internal/server/domain` | `TestT_HUB_003_EditorCannotApprove (internal/server/app/app_test.go)` |
| `REQ-HUB-004` | §3.6, §3.8 | `internal/server/adapters/compiler_dsl`, `internal/server/adapters/scitt_fake`, `internal/server/app` | `TestT_HUB_004_FaultInjectionPerStep (internal/server/app/app_test.go)`, `TestT_HUB_004_ReleaseVisibleAfterAllSteps (internal/server/app/app_test.go)` |
| `REQ-HUB-005` | §3.3, §3.8 | `internal/core/lifecycle`, `internal/server/app` | `TestT_HUB_005_Row1_InitialEnforcedAtNewContent (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row2_CompatibleInheritsOpenIGW (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row2_CompatibleAfterIGWUsesNewContent (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row3_GraceOnCompatible (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row4_EmergencyOnCompatible (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row5_BreakingDefaultGrace (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row6_BreakingExplicitGrace (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row7_ZeroGraceWithoutEmergency (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row8_EmergencyEnforcesAtNewContent (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row8_EmergencyRequiresJustification (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_Row9_EmergencyWithGrace (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_005_ApproveUsesPublicationInstant (internal/server/app/app_test.go)` |
| `REQ-HUB-006` | §3.3, §3.8 | `internal/core/lifecycle`, `internal/server/app` | `TestT_HUB_006_Date1NonMonotonic (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_006_Date2TooManyVersions (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_006_F6ApprovalPasses (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_006_ApprovalRejectedOnDate1 (internal/server/app/app_test.go)` |
| `REQ-HUB-007` | §3.3, §3.8 | `internal/core/lifecycle`, `internal/server/app` | `TestT_HUB_007_MergeKeepsDatesAndContinuityDetectsChanges (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_007_DatesStableAcrossListings (internal/server/app/app_test.go)` |
| `REQ-HUB-008` | §3.4, §3.8 | `cmd/vhub-server`, `internal/core/lifecycle`, `internal/server`, `internal/server/adapters/scheduler`, `internal/server/app`, `internal/server/config` | `TestT_HUB_008_IssuesAtMostEveryHalfTTL (internal/server/app/app_test.go)` |
| `REQ-HUB-009` | §3.4, §3.5, §3.8 | `internal/core/lifecycle`, `internal/server/app` | `TestT_HUB_009_F6StatesOfFirstVersion (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_009_MembershipFollowsStates (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_009_SupersededAfterCompatible (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_009_RevokedRowWins (internal/core/lifecycle/lifecycle_test.go)` |
| `REQ-HUB-010` | §3.4, §3.8 | `internal/core/canon`, `internal/core/did`, `internal/core/sign`, `internal/server/adapters/http_did`, `internal/server/adapters/signer_file`, `internal/server/app`, `internal/server/config` | `TestT_HUB_010_IndependentJCSAndSignature (internal/server/app/app_test.go)` |
| `REQ-HUB-011` | §3.7, §3.8 | `internal/server/adapters/blob_fs`, `internal/server/adapters/http_pub`, `internal/server/app` | `TestT_HUB_011_ImmutableBytesAndETag (internal/server/adapters/http_pub/handler_test.go)` |
| `REQ-HUB-012` | §3.6, §3.8 | `internal/server/adapters/http_v1` | `TestT_HUB_012_VersionsArePrivateNoCache (internal/server/adapters/http_v1/handler_test.go)` |
| `REQ-HUB-013` | §3.2, §3.8 | `internal/server/adapters/sqlite`, `internal/server/app` | `TestT_HUB_013_ImmutableTablesRefuseUpdateAndDelete (internal/server/adapters/sqlite/store_test.go)`, `TestT_HUB_013_ReleaseStoreHasNoMutators (internal/server/adapters/sqlite/store_test.go)` |
| `REQ-HUB-014` | §3.3, §3.8 | `internal/core/lifecycle`, `internal/server/app` | `TestT_HUB_014_DefaultRevokedFrom (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_014_Rev1AlreadyRevoked (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_014_Rev2JustificationRequired (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_014_Rev3TooEarly (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_014_Rev4LeavesNoHead (internal/core/lifecycle/lifecycle_test.go)`, `TestT_HUB_014_RevocationIssuesListing (internal/server/app/app_test.go)` |
| `REQ-HUB-015` | §3.4, §3.8 | `internal/server/adapters/http_did`, `internal/server/adapters/signer_file`, `internal/server/app` | `TestT_HUB_015_SignsOnlyWithPublishedKey (internal/server/adapters/signer_file/signer_test.go)`, `TestT_HUB_015_RotationPublishesBeforeSigning (internal/server/app/app_test.go)` |
| `REQ-HUB-016` | §3.7, §3.8 | `internal/server/adapters/http_pub`, `internal/server/app` | `TestT_HUB_016_RedirectForEveryMediaType (internal/server/adapters/http_pub/handler_test.go)` |
| `REQ-HUB-017` | §3.6, §3.8 | `internal/core/codes`, `internal/core/problem`, `internal/server/adapters/auth_static`, `internal/server/adapters/http_v1`, `internal/server/adapters/httpx`, `internal/server/app`, `internal/server/domain` | `TestT_HUB_017_EndpointRoleMatrix (internal/server/adapters/http_v1/handler_test.go)`, `TestT_HUB_017_AuthenticationRequired (internal/server/adapters/http_v1/handler_test.go)` |
| `REQ-HUB-018` | §3.6, §3.8 | `internal/server/adapters/http_v1`, `internal/server/app` | `TestT_HUB_018_ConcurrentWrites (internal/server/adapters/http_v1/handler_test.go)` |
| `REQ-HUB-019` | §3.6, §3.8 | `internal/core/codes`, `internal/core/problem`, `internal/server/adapters/http_v1`, `internal/server/adapters/httpx`, `internal/server/app` | `TestT_HUB_019_NotFoundOnEveryEndpoint (internal/server/adapters/http_v1/handler_test.go)` |
| `REQ-HUB-020` | §3.6, §3.8 | `internal/server/adapters/sqlite`, `internal/server/app` | `TestT_HUB_020_EventsForEveryEffect (internal/server/app/app_test.go)` |
| `REQ-HUB-021` | §3.4, §3.6, §3.8 | `internal/server/app` | `TestT_HUB_021_RecordEqualsListingEntry (internal/server/app/app_test.go)` |
| `REQ-CLI-001` | §4.3, §4.12 | `cmd/vhub-client`, `internal/client`, `internal/client/config`, `internal/client/domain` | `TestT_CLI_001_ValidConfigLoads (internal/client/config/config_test.go)`, `TestT_CLI_001_CFG1Schema (internal/client/config/config_test.go)`, `TestT_CLI_001_CFG2Overlap (internal/client/config/config_test.go)`, `TestT_CLI_001_CFG3GateMessage (internal/client/config/config_test.go)`, `TestT_CLI_001_CFG4Credentials (internal/client/config/config_test.go)`, `TestT_CLI_001_CFG5Upstream (internal/client/config/config_test.go)`, `TestT_CLI_001_RouteInvariants (internal/client/domain/domain_test.go)` |
| `REQ-CLI-002` | §4.5, §4.12 | `internal/client/domain` | `TestT_CLI_002_OnboardingRejectsInEnforce (internal/client/app/edge_test.go)`, `TestT_CLI_002_OnboardingForwardsInShadow (internal/client/app/edge_test.go)` |
| `REQ-CLI-003` | §4.4, §4.12 | `internal/client`, `internal/client/adapters/http_hub`, `internal/client/adapters/sync_timer`, `internal/client/app` | `TestT_CLI_003_SyncOverHTTPAgainstRealHub (internal/client/adapters/http_hub/client_test.go)`, `TestT_CLI_003_SyncAtStartAndEveryInterval (internal/client/app/sync_test.go)` |
| `REQ-CLI-004` | §4.4, §4.12 | `internal/client/app`, `internal/core/lifecycle` | `TestT_CLI_004_UnauthorizedOverHTTP (internal/client/adapters/http_hub/client_test.go)`, `TestT_CLI_004_S1Unreachable (internal/client/app/sync_test.go)`, `TestT_CLI_004_S1Unauthorized (internal/client/app/sync_test.go)`, `TestT_CLI_004_S2AlteredByte (internal/client/app/sync_test.go)`, `TestT_CLI_004_S2DIDUnresolvable (internal/client/app/sync_test.go)`, `TestT_CLI_004_S3Expired (internal/client/app/sync_test.go)`, `TestT_CLI_004_S4Rollback (internal/client/app/sync_test.go)`, `TestT_CLI_004_S4SequenceConflict (internal/client/app/sync_test.go)`, `TestT_CLI_004_S5RevocationDropped (internal/client/app/sync_test.go)`, `TestT_CLI_004_S6DatesChanged (internal/client/app/sync_test.go)`, `TestT_CLI_004_S7DatesInvalid (internal/client/app/sync_test.go)` |
| `REQ-CLI-005` | §4.4, §4.12 | `internal/client/adapters/store_fs`, `internal/client/app` | `TestT_CLI_005_FileStoreRestoresAfterRestart (internal/client/adapters/http_hub/client_test.go)`, `TestT_CLI_005_ContinuitySurvivesRestart (internal/client/app/sync_test.go)`, `TestT_CLI_005_KnownRevocationSurvivesRestart (internal/client/app/sync_test.go)` |
| `REQ-CLI-006` | §4.4, §4.12 | `internal/client/adapters/receipt_scitt_fake`, `internal/client/app`, `internal/core/ni`, `internal/core/pkgv` | `TestT_CLI_006_OneCorruptVersionDiscardsListing (internal/client/app/sync_test.go)`, `TestT_CLI_006_S9ManifestSignatureOfOtherManifest (internal/client/app/sync_test.go)`, `TestT_CLI_006_S10ReceiptOfOtherDigest (internal/client/app/sync_test.go)` |
| `REQ-CLI-007` | §4.4, §4.12 | `internal/client/app`, `internal/client/domain`, `internal/core/ni`, `internal/core/pkgv` | `TestT_CLI_007_S11FileByteAltered (internal/client/app/sync_test.go)`, `TestT_CLI_007_S12ExtraFile (internal/client/app/sync_test.go)`, `TestT_CLI_007_S13VersionIncoherent (internal/client/app/sync_test.go)`, `TestT_CLI_007_PartialDownloadNeverUsed (internal/client/app/sync_test.go)` |
| `REQ-CLI-008` | §4.5, §4.12 | `internal/client/domain` | `TestT_CLI_008_StateMachineCells (internal/client/app/edge_test.go)`, `TestT_CLI_008_E2AtExactExpiry (internal/client/app/edge_test.go)` |
| `REQ-CLI-009` | §4.6, §4.12 | `internal/core/lifecycle` | `TestT_CLI_009_BoundariesWithLease (internal/core/lifecycle/lifecycle_test.go)`, `TestT_CLI_009_RevocationBoundaryWithLease (internal/core/lifecycle/lifecycle_test.go)` |
| `REQ-CLI-010` | §4.7, §4.12 | `internal/client/adapters/evaluator_poc`, `internal/client/app`, `internal/core/jsonld` | `TestT_CLI_010_ContextNotInPackageWithoutNetwork (internal/client/app/edge_test.go)` |
| `REQ-CLI-011` | §4.7, §4.12 | `internal/client/app`, `internal/client/domain` | `TestT_CLI_011_UnparseableAndNoVersion (internal/client/app/edge_test.go)`, `TestT_CLI_011_RecommendedViolationIsInfo (internal/client/app/edge_test.go)`, `TestT_CLI_011_F6Table (internal/client/app/f6_test.go)` |
| `REQ-CLI-012` | §4.7, §4.12 | `internal/client/app`, `internal/client/domain` | `TestT_CLI_012_ShadowForwardsWithEventObserveIsSilent (internal/client/app/edge_test.go)`, `TestT_CLI_012_ModeTimesClassification (internal/client/domain/domain_test.go)`, `TestT_CLI_012_CombinationUpToFourGates (internal/client/domain/domain_test.go)`, `TestT_CLI_012_EventsOnlyOutsideObserve (internal/client/domain/domain_test.go)` |
| `REQ-CLI-013` | §4.8, §4.12 | `internal/client/app`, `internal/client/domain` | `TestT_CLI_013_RejectBodiesPerMessageType (internal/client/app/edge_test.go)`, `TestT_CLI_013_CatalogErrorAndManagementProblem (internal/client/app/edge_test.go)`, `TestT_CLI_013_ForwardAddsNoHeader (internal/client/app/edge_test.go)`, `TestT_CLI_013_HeaderSerialization (internal/client/domain/domain_test.go)` |
| `REQ-CLI-014` | §4.8, §4.12 | `internal/client/adapters/proxy_dsp`, `internal/client/app` | `TestT_CLI_014_ResponseUnchangedExceptVerdict (internal/client/adapters/proxy_dsp/proxy_test.go)` |
| `REQ-CLI-015` | §4.9, §4.12 | `internal/client/app` | `TestT_CLI_015_CaptureOnlyOn2xxBeforeReply (internal/client/app/edge_test.go)`, `TestT_CLI_015_ShadowWithoutPassIsUnverifiedHead (internal/client/app/edge_test.go)`, `TestT_CLI_015_ProfileMismatch (internal/client/app/edge_test.go)` |
| `REQ-CLI-016` | §4.9, §4.12 | `internal/client/app`, `internal/client/domain` | `TestT_CLI_016_AdmissionRows (internal/client/app/edge_test.go)`, `TestT_CLI_016_RevocationScheduledPerAgreement (internal/client/app/edge_test.go)`, `TestT_CLI_016_F6AgreementAndRevocation (internal/client/app/f6_test.go)` |
| `REQ-CLI-017` | §4.9, §4.12 | `internal/client/adapters/store_fs`, `internal/client/app` | `TestT_CLI_017_RetiredVersionWithAgreementIsRetained (internal/client/app/edge_test.go)` |
| `REQ-CLI-018` | §4.5, §4.12 | `internal/client/domain` | `TestT_CLI_018_DegradedContinueAndReject (internal/client/app/edge_test.go)` |
| `REQ-CLI-019` | §4.11, §4.12 | `internal/client/adapters/proxy_dsp`, `internal/client/adapters/status_http` | `TestT_CLI_019_AdminNotServedOnDSP (internal/client/adapters/proxy_dsp/proxy_test.go)` |
| `REQ-CLI-020` | §4.11, §4.12 | `internal/client/adapters/metrics`, `internal/client/adapters/status_http`, `internal/client/app` | `TestT_CLI_020_StatusOverHTTP (internal/client/adapters/status_http/admin_test.go)`, `TestT_CLI_020_StatusFields (internal/client/app/edge_test.go)` |
| `REQ-CLI-021` | §4.11, §4.12 | `internal/client/adapters/events`, `internal/client/adapters/metrics`, `internal/client/app`, `internal/core/codes`, `internal/core/problem` | `TestT_CLI_021_VerdictEventsValidate (internal/client/app/edge_test.go)` |
| `REQ-CLI-022` | §4.7, §4.12 | `internal/client/adapters/proxy_dsp`, `internal/client/app`, `internal/client/domain` | `TestT_CLI_022_DefaultAction (internal/client/app/edge_test.go)` |
| `REQ-CLI-023` | §4.10, §4.12 | `internal/client/app` | `TestT_CLI_023_PassiveAudit (internal/client/app/edge_test.go)` |
| `REQ-CLI-024` | §4.4.1, §4.12 | `internal/client/app`, `internal/core/did`, `internal/core/sign` | `TestT_CLI_024_KeyRotation (internal/client/app/sync_test.go)`, `TestT_CLI_024_AlteredByteResolvesAtMostOnce (internal/client/app/sync_test.go)`, `TestT_CLI_024_RetiredKeyRejected (internal/client/app/sync_test.go)` |
| `REQ-CLI-025` | §4.11, §4.12 | `internal/client/app`, `internal/server/adapters/httpx` | `TestT_CLI_025_ClockSkewExceeded (internal/client/app/sync_test.go)`, `TestT_CLI_025_SmallSkewIsQuiet (internal/client/app/sync_test.go)` |
| `REQ-CLI-026` | §4.9, §4.11, §4.12 | `internal/client/adapters/status_http`, `internal/client/app` | `TestT_CLI_026_NotFoundOverHTTP (internal/client/adapters/status_http/admin_test.go)`, `TestT_CLI_026_ImportAndGetNotFound (internal/client/app/edge_test.go)` |
| `REQ-CLI-027` | §4.4.1, §4.12 | `internal/client/adapters/did_web`, `internal/client/adapters/http_hub`, `internal/client/adapters/sync_timer`, `internal/client/app` | `TestT_CLI_027_ResolveWhenCacheReachesDidTTL (internal/client/app/sync_test.go)`, `TestT_CLI_027_RefreshFailedKeepsCache (internal/client/app/sync_test.go)`, `TestT_CLI_027_HundredInvalidSignaturesOneResolution (internal/client/app/sync_test.go)` |
| `REQ-DOC-001` | Appendix A.1, §4.12 | `internal/core/schemas` | `TestT_DOC_001_ListingFromBinaryValidates (conformance/documents_test.go)` |
| `REQ-DOC-002` | Appendix A.2, §4.12 | `internal/core/schemas` | `TestT_DOC_002_RecordsFromBinaryValidate (conformance/documents_test.go)` |
| `REQ-DOC-003` | Appendix A.3, §4.12 | `internal/client/config`, `internal/core/schemas` | `TestT_DOC_003_ExampleConfigValidates (internal/client/config/config_test.go)`, `TestT_DOC_003_ConfigurationsValidate (conformance/documents_test.go)` |
| `REQ-DOC-004` | Appendix A.4, §4.12 | `internal/core/schemas` | `TestT_DOC_004_EmittedVerdictEventsValidate (conformance/documents_test.go)` |

## 2. Trazabilidad inversa Módulo → Requisitos (`§8.8`)

| Módulo | Requisitos |
| --- | --- |
| `cmd/vhub-client` | `REQ-CLI-001`, `REQ-SYS-002` |
| `cmd/vhub-server` | `REQ-HUB-008` |
| `internal/client` | `REQ-CLI-001`, `REQ-CLI-003`, `REQ-SYS-002` |
| `internal/client/adapters/did_web` | `REQ-CLI-027` |
| `internal/client/adapters/evaluator_poc` | `REQ-CLI-010` |
| `internal/client/adapters/events` | `REQ-CLI-021` |
| `internal/client/adapters/http_hub` | `REQ-CLI-003`, `REQ-CLI-027`, `REQ-SYS-002` |
| `internal/client/adapters/metrics` | `REQ-CLI-020`, `REQ-CLI-021` |
| `internal/client/adapters/proxy_dsp` | `REQ-CLI-014`, `REQ-CLI-019`, `REQ-CLI-022`, `REQ-SYS-003` |
| `internal/client/adapters/receipt_scitt_fake` | `REQ-CLI-006` |
| `internal/client/adapters/status_http` | `REQ-CLI-019`, `REQ-CLI-020`, `REQ-CLI-026` |
| `internal/client/adapters/store_fs` | `REQ-CLI-005`, `REQ-CLI-017`, `REQ-SYS-006` |
| `internal/client/adapters/sync_timer` | `REQ-CLI-003`, `REQ-CLI-027` |
| `internal/client/app` | `REQ-CLI-003`, `REQ-CLI-004`, `REQ-CLI-005`, `REQ-CLI-006`, `REQ-CLI-007`, `REQ-CLI-010`, `REQ-CLI-011`, `REQ-CLI-012`, `REQ-CLI-013`, `REQ-CLI-014`, `REQ-CLI-015`, `REQ-CLI-016`, `REQ-CLI-017`, `REQ-CLI-020`, `REQ-CLI-021`, `REQ-CLI-022`, `REQ-CLI-023`, `REQ-CLI-024`, `REQ-CLI-025`, `REQ-CLI-026`, `REQ-CLI-027`, `REQ-SYS-001`, `REQ-SYS-002`, `REQ-SYS-003` |
| `internal/client/config` | `REQ-CLI-001`, `REQ-DOC-003` |
| `internal/client/domain` | `REQ-CLI-001`, `REQ-CLI-002`, `REQ-CLI-007`, `REQ-CLI-008`, `REQ-CLI-011`, `REQ-CLI-012`, `REQ-CLI-013`, `REQ-CLI-016`, `REQ-CLI-018`, `REQ-CLI-022`, `REQ-SYS-001`, `REQ-SYS-007`, `REQ-SYS-008` |
| `internal/clock` | `REQ-SYS-001` |
| `internal/core/canon` | `REQ-HUB-010` |
| `internal/core/codes` | `REQ-CLI-021`, `REQ-HUB-017`, `REQ-HUB-019` |
| `internal/core/did` | `REQ-CLI-024`, `REQ-HUB-010`, `REQ-SYS-009` |
| `internal/core/jsonld` | `REQ-CLI-010`, `REQ-SYS-001` |
| `internal/core/lifecycle` | `REQ-CLI-004`, `REQ-CLI-009`, `REQ-HUB-001`, `REQ-HUB-005`, `REQ-HUB-006`, `REQ-HUB-007`, `REQ-HUB-008`, `REQ-HUB-009`, `REQ-HUB-014`, `REQ-SYS-004`, `REQ-SYS-007` |
| `internal/core/ni` | `REQ-CLI-006`, `REQ-CLI-007` |
| `internal/core/pkgv` | `REQ-CLI-006`, `REQ-CLI-007` |
| `internal/core/problem` | `REQ-CLI-021`, `REQ-HUB-017`, `REQ-HUB-019` |
| `internal/core/schemas` | `REQ-DOC-001`, `REQ-DOC-002`, `REQ-DOC-003`, `REQ-DOC-004` |
| `internal/core/semver` | `REQ-SYS-001` |
| `internal/core/sign` | `REQ-CLI-024`, `REQ-HUB-010`, `REQ-SYS-009` |
| `internal/core/timex` | `REQ-SYS-001` |
| `internal/server` | `REQ-HUB-008` |
| `internal/server/adapters/auth_static` | `REQ-HUB-017` |
| `internal/server/adapters/blob_fs` | `REQ-HUB-011`, `REQ-SYS-005` |
| `internal/server/adapters/compiler_dsl` | `REQ-HUB-001`, `REQ-HUB-004` |
| `internal/server/adapters/compiler_poc` | `REQ-HUB-001` |
| `internal/server/adapters/http_did` | `REQ-HUB-010`, `REQ-HUB-015` |
| `internal/server/adapters/http_pub` | `REQ-HUB-011`, `REQ-HUB-016`, `REQ-SYS-005` |
| `internal/server/adapters/http_v1` | `REQ-HUB-012`, `REQ-HUB-017`, `REQ-HUB-018`, `REQ-HUB-019` |
| `internal/server/adapters/httpx` | `REQ-CLI-025`, `REQ-HUB-017`, `REQ-HUB-019` |
| `internal/server/adapters/scheduler` | `REQ-HUB-008` |
| `internal/server/adapters/scitt_fake` | `REQ-HUB-004` |
| `internal/server/adapters/signer_file` | `REQ-HUB-010`, `REQ-HUB-015` |
| `internal/server/adapters/sqlite` | `REQ-HUB-013`, `REQ-HUB-020`, `REQ-SYS-005`, `REQ-SYS-006` |
| `internal/server/app` | `REQ-HUB-001`, `REQ-HUB-002`, `REQ-HUB-003`, `REQ-HUB-004`, `REQ-HUB-005`, `REQ-HUB-006`, `REQ-HUB-007`, `REQ-HUB-008`, `REQ-HUB-009`, `REQ-HUB-010`, `REQ-HUB-011`, `REQ-HUB-013`, `REQ-HUB-014`, `REQ-HUB-015`, `REQ-HUB-016`, `REQ-HUB-017`, `REQ-HUB-018`, `REQ-HUB-019`, `REQ-HUB-020`, `REQ-HUB-021`, `REQ-SYS-005` |
| `internal/server/config` | `REQ-HUB-008`, `REQ-HUB-010` |
| `internal/server/domain` | `REQ-HUB-002`, `REQ-HUB-003`, `REQ-HUB-017` |

## 3. Cobertura del catálogo cerrado de códigos F5 (`N-<CÓDIGO>`)

| Código | Emisor | Canal | Severidad | Requisito | Caso `N-<CÓDIGO>` |
| --- | --- | --- | --- | --- | --- |
| `API_FORBIDDEN_ROLE` | C-HUB | problem | `error` | REQ-HUB-017 | `N-API_FORBIDDEN_ROLE` (`conformance/negative_hub_test.go`) |
| `API_PRECONDITION_REQUIRED` | C-HUB | problem | `error` | REQ-HUB-018 | `N-API_PRECONDITION_REQUIRED` (`conformance/negative_hub_test.go`) |
| `API_PRECONDITION_FAILED` | C-HUB | problem | `error` | REQ-HUB-018 | `N-API_PRECONDITION_FAILED` (`conformance/negative_hub_test.go`) |
| `API_NOT_FOUND` | C-HUB,C-CLI | problem | `error` | REQ-HUB-019,REQ-CLI-026 | `N-API_NOT_FOUND` (`conformance/negative_hub_test.go`) |
| `API_INVALID_STATE` | C-HUB | problem | `error` | REQ-HUB-002 | `N-API_INVALID_STATE` (`conformance/negative_hub_test.go`) |
| `RELEASE_ABORTED` | C-HUB | problem | `error` | REQ-HUB-004 | `N-RELEASE_ABORTED` (`conformance/negative_hub_test.go`) |
| `DATES_GRACE_ON_COMPATIBLE` | C-HUB | problem | `error` | REQ-HUB-005 | `N-DATES_GRACE_ON_COMPATIBLE` (`conformance/negative_hub_test.go`) |
| `DATES_EMERGENCY_ON_COMPATIBLE` | C-HUB | problem | `error` | REQ-HUB-005 | `N-DATES_EMERGENCY_ON_COMPATIBLE` (`conformance/negative_hub_test.go`) |
| `DATES_ZERO_GRACE_WITHOUT_EMERGENCY` | C-HUB | problem | `error` | REQ-HUB-005 | `N-DATES_ZERO_GRACE_WITHOUT_EMERGENCY` (`conformance/negative_hub_test.go`) |
| `DATES_EMERGENCY_WITH_GRACE` | C-HUB | problem | `error` | REQ-HUB-005 | `N-DATES_EMERGENCY_WITH_GRACE` (`conformance/negative_hub_test.go`) |
| `DATES_JUSTIFICATION_REQUIRED` | C-HUB | problem | `error` | REQ-HUB-005,REQ-HUB-014 | `N-DATES_JUSTIFICATION_REQUIRED` (`conformance/negative_hub_test.go`) |
| `DATES_NON_MONOTONIC` | C-HUB | problem | `error` | REQ-HUB-006 | `N-DATES_NON_MONOTONIC` (`conformance/negative_hub_test.go`) |
| `DATES_TOO_MANY_VERSIONS` | C-HUB | problem | `error` | REQ-HUB-006 | `N-DATES_TOO_MANY_VERSIONS` (`conformance/negative_hub_test.go`) |
| `DATES_REVOCATION_TOO_EARLY` | C-HUB | problem | `error` | REQ-HUB-014 | `N-DATES_REVOCATION_TOO_EARLY` (`conformance/negative_hub_test.go`) |
| `DATES_REVOKE_LEAVES_NO_HEAD` | C-HUB | problem | `error` | REQ-HUB-014 | `N-DATES_REVOKE_LEAVES_NO_HEAD` (`conformance/negative_hub_test.go`) |
| `DATES_ALREADY_REVOKED` | C-HUB | problem | `error` | REQ-HUB-014 | `N-DATES_ALREADY_REVOKED` (`conformance/negative_hub_test.go`) |
| `LIFECYCLE_STALENESS_LT_TTL` | C-HUB | problem | `error` | N2 | `N-LIFECYCLE_STALENESS_LT_TTL` (`conformance/corpus/drafts/LIFECYCLE_STALENESS_LT_TTL.json`) |
| `LIFECYCLE_GRACE_ZERO` | C-HUB | problem | `error` | REQ-HUB-001 | `N-LIFECYCLE_GRACE_ZERO` (`conformance/corpus/drafts/LIFECYCLE_GRACE_ZERO.json`) |
| `LIFECYCLE_SYNC_INTERVAL` | C-HUB | problem | `error` | REQ-HUB-001 | `N-LIFECYCLE_SYNC_INTERVAL` (`conformance/corpus/drafts/LIFECYCLE_SYNC_INTERVAL.json`) |
| `LIFECYCLE_CLOCK_SKEW` | C-HUB | problem | `error` | REQ-HUB-001 | `N-LIFECYCLE_CLOCK_SKEW` (`conformance/corpus/drafts/LIFECYCLE_CLOCK_SKEW.json`) |
| `LIFECYCLE_LEASE` | C-HUB | problem | `error` | REQ-HUB-001 | `N-LIFECYCLE_LEASE` (`conformance/corpus/drafts/LIFECYCLE_LEASE.json`) |
| `LIFECYCLE_PROPAGATION` | C-HUB | problem | `error` | REQ-HUB-001 | `N-LIFECYCLE_PROPAGATION` (`conformance/corpus/drafts/LIFECYCLE_PROPAGATION.json`) |
| `LIFECYCLE_MAX_VERSIONS` | C-HUB | problem | `error` | REQ-HUB-001 | `N-LIFECYCLE_MAX_VERSIONS` (`conformance/corpus/drafts/LIFECYCLE_MAX_VERSIONS.json`) |
| `CONFIG_SCHEMA` | C-CLI | arranque | `error` | REQ-CLI-001 | `N-CONFIG_SCHEMA` (`conformance/corpus/config/CONFIG_SCHEMA.json`, `conformance/negative_client_test.go`) |
| `CONFIG_ROUTE_OVERLAP` | C-CLI | arranque | `error` | REQ-CLI-001 | `N-CONFIG_ROUTE_OVERLAP` (`conformance/corpus/config/CONFIG_ROUTE_OVERLAP.json`, `conformance/negative_client_test.go`) |
| `CONFIG_GATE_MESSAGE_MISMATCH` | C-CLI | arranque | `error` | REQ-CLI-001 | `N-CONFIG_GATE_MESSAGE_MISMATCH` (`conformance/corpus/config/CONFIG_GATE_MESSAGE_MISMATCH.json`, `conformance/negative_client_test.go`) |
| `CONFIG_CREDENTIALS_UNREADABLE` | C-CLI | arranque | `error` | REQ-CLI-001 | `N-CONFIG_CREDENTIALS_UNREADABLE` (`conformance/corpus/config/CONFIG_CREDENTIALS_UNREADABLE.json`, `conformance/negative_client_test.go`) |
| `CONFIG_UPSTREAM_MISSING` | C-CLI | arranque | `error` | REQ-CLI-001 | `N-CONFIG_UPSTREAM_MISSING` (`conformance/corpus/config/CONFIG_UPSTREAM_MISSING.json`, `conformance/negative_client_test.go`) |
| `SYNC_UNREACHABLE` | C-CLI | sync | `error` | REQ-CLI-004 | `N-SYNC_UNREACHABLE` (`conformance/negative_client_test.go`) |
| `SYNC_UNAUTHORIZED` | C-CLI | sync | `error` | REQ-CLI-004 | `N-SYNC_UNAUTHORIZED` (`conformance/negative_client_test.go`) |
| `SYNC_SIGNATURE_INVALID` | C-CLI | sync | `error` | REQ-CLI-004,REQ-CLI-024 | `N-SYNC_SIGNATURE_INVALID` (`conformance/negative_client_test.go`) |
| `SYNC_DID_UNRESOLVABLE` | C-CLI | sync | `error` | REQ-CLI-027 | `N-SYNC_DID_UNRESOLVABLE` (`conformance/negative_client_test.go`) |
| `DID_KEY_ROTATED` | C-CLI | sync | `info` | REQ-CLI-024 | `N-DID_KEY_ROTATED` (`conformance/negative_client_test.go`) |
| `DID_REFRESH_FAILED` | C-CLI | sync | `info` | REQ-CLI-027 | `N-DID_REFRESH_FAILED` (`conformance/negative_client_test.go`) |
| `SYNC_EXPIRED` | C-CLI | sync | `error` | REQ-CLI-004 | `N-SYNC_EXPIRED` (`conformance/negative_client_test.go`) |
| `SYNC_ROLLBACK` | C-CLI | sync | `error` | REQ-CLI-004 | `N-SYNC_ROLLBACK` (`conformance/negative_client_test.go`) |
| `SYNC_SEQUENCE_CONFLICT` | C-CLI | sync | `error` | REQ-CLI-004 | `N-SYNC_SEQUENCE_CONFLICT` (`conformance/negative_client_test.go`) |
| `SYNC_REVOCATION_DROPPED` | C-CLI | sync | `error` | REQ-CLI-004 | `N-SYNC_REVOCATION_DROPPED` (`conformance/negative_client_test.go`) |
| `SYNC_DATES_CHANGED` | C-CLI | sync | `error` | REQ-CLI-004 | `N-SYNC_DATES_CHANGED` (`conformance/negative_client_test.go`) |
| `SYNC_DATES_INVALID` | C-CLI | sync | `error` | REQ-CLI-004 | `N-SYNC_DATES_INVALID` (`conformance/negative_client_test.go`) |
| `SYNC_MANIFEST_DIGEST_MISMATCH` | C-CLI | sync | `error` | REQ-CLI-006 | `N-SYNC_MANIFEST_DIGEST_MISMATCH` (`conformance/negative_client_test.go`) |
| `SYNC_MANIFEST_SIGNATURE_INVALID` | C-CLI | sync | `error` | REQ-CLI-006,REQ-CLI-024 | `N-SYNC_MANIFEST_SIGNATURE_INVALID` (`conformance/negative_client_test.go`) |
| `SYNC_RECEIPT_INVALID` | C-CLI | sync | `error` | REQ-CLI-006 | `N-SYNC_RECEIPT_INVALID` (`conformance/negative_client_test.go`) |
| `SYNC_FILE_HASH_MISMATCH` | C-CLI | sync | `error` | REQ-CLI-007 | `N-SYNC_FILE_HASH_MISMATCH` (`conformance/negative_client_test.go`) |
| `SYNC_FILE_SET_MISMATCH` | C-CLI | sync | `error` | REQ-CLI-007 | `N-SYNC_FILE_SET_MISMATCH` (`conformance/negative_client_test.go`) |
| `SYNC_INCOHERENT` | C-CLI | sync | `error` | REQ-CLI-007 | `N-SYNC_INCOHERENT` (`conformance/negative_client_test.go`) |
| `CLIENT_NOT_READY` | C-CLI | clasificación | `error` | REQ-CLI-002 | `N-CLIENT_NOT_READY` (`conformance/negative_client_test.go`) |
| `CLIENT_DEGRADED_REJECT` | C-CLI | clasificación | `error` | REQ-CLI-018 | `N-CLIENT_DEGRADED_REJECT` (`conformance/negative_client_test.go`) |
| `CLIENT_FAIL_CLOSED` | C-CLI | clasificación | `error` | REQ-CLI-018 | `N-CLIENT_FAIL_CLOSED` (`conformance/negative_client_test.go`) |
| `NO_VERSION_IN_FORCE` | C-CLI | clasificación | `error` | REQ-CLI-011 | `N-NO_VERSION_IN_FORCE` (`conformance/negative_client_test.go`) |
| `MESSAGE_UNPARSEABLE` | C-CLI | clasificación | `error` | REQ-CLI-011 | `N-MESSAGE_UNPARSEABLE` (`conformance/negative_client_test.go`) |
| `CONTEXT_NOT_IN_PACKAGE` | C-CLI | clasificación | `error` | REQ-CLI-010 | `N-CONTEXT_NOT_IN_PACKAGE` (`conformance/negative_client_test.go`) |
| `VERDICT_VIOLATION` | C-CLI | clasificación | `error` | REQ-CLI-011 | `N-VERDICT_VIOLATION` (`conformance/negative_client_test.go`) |
| `VERDICT_NOT_IN_HEAD` | C-CLI | clasificación | `warning` | REQ-CLI-011 | `N-VERDICT_NOT_IN_HEAD` (`conformance/negative_client_test.go`) |
| `VERDICT_RECOMMENDED_NONCONFORMANT` | C-CLI | clasificación | `info` | REQ-CLI-011 | `N-VERDICT_RECOMMENDED_NONCONFORMANT` (`conformance/negative_client_test.go`) |
| `VERDICT_SHACL_WARNING` | C-CLI | clasificación | `info` | REQ-CLI-011 | _N/A en PoC_: Unreachable in PoC by DP-7 (evaluator_poc ignores shacl artefacts with POC_EVAL_UNSUPPORTED_ROLE) |
| `ROUTE_DEFAULT_REJECT` | C-CLI | clasificación | `error` | REQ-CLI-022 | `N-ROUTE_DEFAULT_REJECT` (`conformance/negative_client_test.go`) |
| `AGREEMENT_VERSION_REVOKED` | C-CLI | clasificación | `error` | REQ-CLI-016 | `N-AGREEMENT_VERSION_REVOKED` (`conformance/negative_client_test.go`) |
| `AGREEMENT_UNKNOWN` | C-CLI | clasificación | `error|info` | REQ-CLI-016 | `N-AGREEMENT_UNKNOWN` (`conformance/negative_client_test.go`) |
| `AGREEMENT_PROFILE_MISMATCH` | C-CLI | evento | `info` | REQ-CLI-015 | `N-AGREEMENT_PROFILE_MISMATCH` (`conformance/negative_client_test.go`) |
| `AGREEMENT_REVOCATION_SCHEDULED` | C-CLI | evento | `info` | REQ-CLI-016 | `N-AGREEMENT_REVOCATION_SCHEDULED` (`conformance/negative_client_test.go`) |
| `AUDIT_WILL_WARN` | C-CLI | evento | `info` | REQ-CLI-023 | `N-AUDIT_WILL_WARN` (`conformance/negative_client_test.go`) |
| `AUDIT_WILL_REJECT` | C-CLI | evento | `info` | REQ-CLI-023 | `N-AUDIT_WILL_REJECT` (`conformance/negative_client_test.go`) |
| `CLOCK_SKEW_EXCEEDED` | C-CLI | evento | `info` | REQ-CLI-025 | `N-CLOCK_SKEW_EXCEEDED` (`conformance/negative_client_test.go`) |
