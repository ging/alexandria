// Validation findings of the language with severity, code, document, JSON pointer and source line (§12).
// Codes are those of the language spec, not the closed vhub catalog of F5.
package dsl

import (
	"fmt"
	"sort"
)

// Severities of a finding; warnings never block a compilation.
const (
	Error   = "error"
	Warning = "warning"
)

// Codes of the five validation levels of §12 (plus the agent additions listed in docs/DSL.md).
const (
	CodeYAML                  = "YAML"
	CodeSchema                = "SCHEMA"
	CodeTermDupID             = "TERM_DUP_ID"
	CodeTermAdoptedCollision  = "TERM_ADOPTED_COLLISION"
	CodeAliasCollision        = "ALIAS_COLLISION"
	CodePrefixUndeclared      = "PREFIX_UNDECLARED"
	CodeProfileIDIsNamespace  = "PROFILE_ID_IS_NAMESPACE"
	CodeSigOperatorType       = "SIG_OPERATOR_TYPE"
	CodeSigOperatorUnknown    = "SIG_OPERATOR_UNKNOWN"
	CodeSigValueKind          = "SIG_VALUE_KIND"
	CodeSigScheme             = "SIG_SCHEME_UNSUPPORTED"
	CodeEnforcementNoSig      = "ENFORCEMENT_WITHOUT_SIGNATURE"
	CodeIncludedInMissing     = "ACTION_INCLUDED_IN_MISSING"
	CodeNoReplacement         = "TERM_NO_REPLACEMENT"
	CodeExtraTurtle           = "EXTRA_TURTLE_UNCHECKED"
	CodeSlotDup               = "SLOT_DUP"
	CodeMandatoryUnverifiable = "MANDATORY_UNVERIFIABLE"
	CodeViaCycle              = "VIA_CYCLE"
	CodeComplianceIgnored     = "CONFORMANCE_REQUIRED_IGNORED"
	CodeAdoptedKindMismatch   = "ADOPTED_KIND_MISMATCH"
	CodeAdoptedUnknown        = "ADOPTED_UNKNOWN"
	CodeBindingUnresolved     = "BINDING_UNRESOLVED"
	CodeBindingBaseMismatch   = "BINDING_BASE_MISMATCH"
	CodeArtifactMissing       = "ARTIFACT_MISSING"
	CodeProfileVersionChanged = "PROFILE_VERSION_CHANGED"
	CodeVersionBumpTooLow     = "VERSION_BUMP_TOO_LOW"
	CodeTermRemoved           = "TERM_REMOVED"
	CodeRetiredNoDeprecation  = "TERM_RETIRED_WITHOUT_DEPRECATION"
	CodeNamespaceChanged      = "NAMESPACE_CHANGED"
	CodeOutputInvalid         = "OUTPUT_INVALID"
	CodeShaclUnionConflict    = "SHACL_UNION_CONFLICT"
)

// Finding is one validation result.
type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Level    string `json:"level"`
	Document string `json:"document,omitempty"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
}

// String renders a finding as "severity CODE doc:line path: message".
func (f Finding) String() string {
	return fmt.Sprintf("%-7s %-34s %s:%d %s %s", f.Severity, f.Code, f.Document, f.Line, f.Path, f.Message)
}

// Findings is an ordered list of results.
type Findings []Finding

// add appends a finding located in document d at pointer ptr.
func (fs *Findings) add(d *Document, level, sev, code, ptr, format string, args ...any) {
	f := Finding{Severity: sev, Code: code, Level: level, Path: ptr, Message: fmt.Sprintf(format, args...)}
	if d != nil {
		f.Document, f.Line = d.Label(), d.Line(ptr)
	}
	*fs = append(*fs, f)
}

// Errors returns the findings of severity error.
func (fs Findings) Errors() Findings {
	var out Findings
	for _, f := range fs {
		if f.Severity == Error {
			out = append(out, f)
		}
	}
	return out
}

// Sorted returns the findings ordered by document, line and code, for deterministic reports.
func (fs Findings) Sorted() Findings {
	out := append(Findings(nil), fs...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Document != out[j].Document {
			return out[i].Document < out[j].Document
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Code < out[j].Code
	})
	return out
}
