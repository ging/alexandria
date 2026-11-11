// Package domain defines the listeners, gates and modes of the client, shared by rules, decisions and events.
// Values follow §4.2, §2.4 and the enumerations of Appendix A.6 (REQ-CLI-012, REQ-CLI-030).
package domain

// Listener names a traffic listener (§4.2).
type Listener string

// Listeners of §4.2 that carry traffic.
const (
	DSPIngress  Listener = "dsp-ingress"
	MgmtIngress Listener = "mgmt-ingress"
	DSPEgress   Listener = "dsp-egress"
)

// Gate is a validation point.
type Gate string

// Gates of §2.4 and §4.9.
const (
	GateDSP       Gate = "dsp_message"
	GatePolicy    Gate = "policy_intake"
	GateCatalog   Gate = "catalog_entry"
	GateAdmission Gate = "agreement_admission"
)

// Mode is what a rule does with a classification.
type Mode string

// Modes of §2.4.
const (
	Observe Mode = "observe"
	Shadow  Mode = "shadow"
	Enforce Mode = "enforce"
)
