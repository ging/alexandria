// Package codes holds the closed catalog of error, warning and info codes loaded from the F5 table (catalog.tsv).
// A code outside this catalog does not exist; POC_ codes live apart in compiler_poc and evaluator_poc (F5, REQ-CLI-021).
package codes

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
)

// Code is a catalog identifier in UPPER_SNAKE_CASE.
type Code string

// Severity is error, warning or info (F5 §5.1).
type Severity string

// Severities of F5 §5.1.
const (
	SevError   Severity = "error"
	SevWarning Severity = "warning"
	SevInfo    Severity = "info"
)

// Entry is one row of the F5 catalog.
type Entry struct {
	Code        Code
	Emitter     string
	Channel     string
	Severity    string
	Requirement string
	Negative    string
}

// Catalog codes, one constant per F5 row.
const (
	APIForbiddenRole               Code = "API_FORBIDDEN_ROLE"
	APIPreconditionRequired        Code = "API_PRECONDITION_REQUIRED"
	APIPreconditionFailed          Code = "API_PRECONDITION_FAILED"
	APINotFound                    Code = "API_NOT_FOUND"
	APIInvalidState                Code = "API_INVALID_STATE"
	ReleaseAborted                 Code = "RELEASE_ABORTED"
	DatesGraceOnCompatible         Code = "DATES_GRACE_ON_COMPATIBLE"
	DatesEmergencyOnCompatible     Code = "DATES_EMERGENCY_ON_COMPATIBLE"
	DatesZeroGraceWithoutEmergency Code = "DATES_ZERO_GRACE_WITHOUT_EMERGENCY"
	DatesEmergencyWithGrace        Code = "DATES_EMERGENCY_WITH_GRACE"
	DatesJustificationRequired     Code = "DATES_JUSTIFICATION_REQUIRED"
	DatesNonMonotonic              Code = "DATES_NON_MONOTONIC"
	DatesTooManyVersions           Code = "DATES_TOO_MANY_VERSIONS"
	DatesRevocationTooEarly        Code = "DATES_REVOCATION_TOO_EARLY"
	DatesRevokeLeavesNoHead        Code = "DATES_REVOKE_LEAVES_NO_HEAD"
	DatesAlreadyRevoked            Code = "DATES_ALREADY_REVOKED"
	LifecycleStalenessLtTTL        Code = "LIFECYCLE_STALENESS_LT_TTL"
	LifecycleGraceZero             Code = "LIFECYCLE_GRACE_ZERO"
	LifecycleSyncInterval          Code = "LIFECYCLE_SYNC_INTERVAL"
	LifecycleClockSkew             Code = "LIFECYCLE_CLOCK_SKEW"
	LifecycleLease                 Code = "LIFECYCLE_LEASE"
	LifecyclePropagation           Code = "LIFECYCLE_PROPAGATION"
	LifecycleMaxVersions           Code = "LIFECYCLE_MAX_VERSIONS"
	ConfigSchema                   Code = "CONFIG_SCHEMA"
	ConfigRouteOverlap             Code = "CONFIG_ROUTE_OVERLAP"
	ConfigGateMessageMismatch      Code = "CONFIG_GATE_MESSAGE_MISMATCH"
	// #nosec G101 -- a code name, not a credential.
	ConfigCredentialsUnreadable     Code = "CONFIG_CREDENTIALS_UNREADABLE"
	ConfigUpstreamMissing           Code = "CONFIG_UPSTREAM_MISSING"
	SyncUnreachable                 Code = "SYNC_UNREACHABLE"
	SyncUnauthorized                Code = "SYNC_UNAUTHORIZED"
	SyncSignatureInvalid            Code = "SYNC_SIGNATURE_INVALID"
	SyncDidUnresolvable             Code = "SYNC_DID_UNRESOLVABLE"
	DidKeyRotated                   Code = "DID_KEY_ROTATED"
	DidRefreshFailed                Code = "DID_REFRESH_FAILED"
	SyncExpired                     Code = "SYNC_EXPIRED"
	SyncRollback                    Code = "SYNC_ROLLBACK"
	SyncSequenceConflict            Code = "SYNC_SEQUENCE_CONFLICT"
	SyncRevocationDropped           Code = "SYNC_REVOCATION_DROPPED"
	SyncDatesChanged                Code = "SYNC_DATES_CHANGED"
	SyncDatesInvalid                Code = "SYNC_DATES_INVALID"
	SyncManifestDigestMismatch      Code = "SYNC_MANIFEST_DIGEST_MISMATCH"
	SyncManifestSignatureInvalid    Code = "SYNC_MANIFEST_SIGNATURE_INVALID"
	SyncReceiptInvalid              Code = "SYNC_RECEIPT_INVALID"
	SyncFileHashMismatch            Code = "SYNC_FILE_HASH_MISMATCH"
	SyncFileSetMismatch             Code = "SYNC_FILE_SET_MISMATCH"
	SyncIncoherent                  Code = "SYNC_INCOHERENT"
	ClientNotReady                  Code = "CLIENT_NOT_READY"
	ClientDegradedReject            Code = "CLIENT_DEGRADED_REJECT"
	ClientFailClosed                Code = "CLIENT_FAIL_CLOSED"
	NoVersionInForce                Code = "NO_VERSION_IN_FORCE"
	MessageUnparseable              Code = "MESSAGE_UNPARSEABLE"
	ContextNotInPackage             Code = "CONTEXT_NOT_IN_PACKAGE"
	VerdictViolation                Code = "VERDICT_VIOLATION"
	VerdictNotInHead                Code = "VERDICT_NOT_IN_HEAD"
	VerdictRecommendedNonconformant Code = "VERDICT_RECOMMENDED_NONCONFORMANT"
	VerdictShaclWarning             Code = "VERDICT_SHACL_WARNING"
	RouteDefaultReject              Code = "ROUTE_DEFAULT_REJECT"
	AgreementVersionRevoked         Code = "AGREEMENT_VERSION_REVOKED"
	AgreementUnknown                Code = "AGREEMENT_UNKNOWN"
	AgreementProfileMismatch        Code = "AGREEMENT_PROFILE_MISMATCH"
	AgreementRevocationScheduled    Code = "AGREEMENT_REVOCATION_SCHEDULED"
	AuditWillWarn                   Code = "AUDIT_WILL_WARN"
	AuditWillReject                 Code = "AUDIT_WILL_REJECT"
	ClockSkewExceeded               Code = "CLOCK_SKEW_EXCEEDED"
)

//go:embed catalog.tsv
var catalogTSV string

var catalog = parse(catalogTSV)

// parse loads the embedded F5 table.
func parse(tsv string) map[Code]Entry {
	out := map[Code]Entry{}
	for _, line := range strings.Split(tsv, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			panic(fmt.Sprintf("codes: malformed catalog row %q", line))
		}
		out[Code(f[0])] = Entry{Code(f[0]), f[1], f[2], f[3], f[4], f[5]}
	}
	return out
}

// All returns the catalog entries.
func All() map[Code]Entry { return catalog }

// Lookup returns the catalog entry of c.
func Lookup(c Code) (Entry, bool) { e, ok := catalog[c]; return e, ok }

// SeverityOf returns the base severity of c; POC_ codes are errors unless they are info events.
func SeverityOf(c Code) Severity {
	e, ok := catalog[c]
	if !ok {
		return SevError
	}
	s, _, _ := strings.Cut(e.Severity, "|")
	return Severity(s)
}

// Error is a coded failure with optional location.
type Error struct {
	Code   Code
	Detail string
	Path   string
	Line   int
}

// Error implements error.
func (e *Error) Error() string {
	if e.Detail == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Detail
}

// New builds a coded error with a formatted detail.
func New(c Code, format string, args ...any) *Error {
	return &Error{Code: c, Detail: fmt.Sprintf(format, args...)}
}

// As extracts a coded error from err, if any.
func As(err error) (*Error, bool) {
	var ce *Error
	ok := errors.As(err, &ce)
	return ce, ok
}
