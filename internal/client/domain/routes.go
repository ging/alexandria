// Package domain provides the route table of the client: listeners, gates, modes, segment matching and the CFG-2, CFG-3 and CFG-5 invariants.
// Includes the default DSP route annex in observe mode (§4.3, Anexo de rutas DSP, REQ-CLI-001, REQ-CLI-022, DP-10).
package domain

import (
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
)

// Listener names a client listener (§4.2).
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

// Mode is the local mode of a gate.
type Mode string

// Modes of §2.4.
const (
	Observe Mode = "observe"
	Shadow  Mode = "shadow"
	Enforce Mode = "enforce"
)

// ValidateResponse configures passive validation of catalog responses.
type ValidateResponse struct {
	Gate Gate `json:"gate"`
	Mode Mode `json:"mode,omitempty"`
}

// Match is the method and path template of a route.
type Match struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// Body configures the JSONPath selection of the evaluated body.
type Body struct {
	Select string `json:"select,omitempty"`
}

// Route is one entry of routes[].
type Route struct {
	Listener         Listener          `json:"listener"`
	Match            Match             `json:"match"`
	Message          string            `json:"message"`
	Gates            []Gate            `json:"gates"`
	Mode             Mode              `json:"mode,omitempty"`
	Body             *Body             `json:"body,omitempty"`
	ValidateResponse *ValidateResponse `json:"validate_response,omitempty"`
}

// Name renders the route as "METHOD /path" for events.
func (r Route) Name() string { return r.Match.Method + " " + r.Match.Path }

// EffectiveMode returns the route mode with its observe default.
func (r Route) EffectiveMode() Mode {
	if r.Mode == "" {
		return Observe
	}
	return r.Mode
}

// Select returns the JSONPath with its $ default.
func (r Route) Select() string {
	if r.Body == nil || r.Body.Select == "" {
		return "$"
	}
	return r.Body.Select
}

// segments splits a path into its non-empty segments.
func segments(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// matches reports whether a template matches a concrete path segment by segment.
func matches(tpl, path []string) bool {
	if len(tpl) != len(path) {
		return false
	}
	for i := range tpl {
		if !strings.HasPrefix(tpl[i], ":") && tpl[i] != path[i] {
			return false
		}
	}
	return true
}

// Find returns the route of a listener, method and path.
func Find(routes []Route, l Listener, method, path string) (Route, bool) {
	p := segments(path)
	for _, r := range routes {
		if r.Listener == l && r.Match.Method == method && matches(segments(r.Match.Path), p) {
			return r, true
		}
	}
	return Route{}, false
}

// overlap reports whether two templates can match the same concrete path.
func overlap(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] && !strings.HasPrefix(a[i], ":") && !strings.HasPrefix(b[i], ":") {
			return false
		}
	}
	return true
}

// CheckOverlap is CFG-2.
func CheckOverlap(routes []Route) *codes.Error {
	for i := range routes {
		for j := i + 1; j < len(routes); j++ {
			a, b := routes[i], routes[j]
			if a.Listener == b.Listener && a.Match.Method == b.Match.Method && overlap(segments(a.Match.Path), segments(b.Match.Path)) {
				return codes.New(codes.ConfigRouteOverlap, "%s and %s overlap on %s", a.Name(), b.Name(), a.Listener)
			}
		}
	}
	return nil
}

// CheckGateMessage is CFG-3.
func CheckGateMessage(routes []Route) *codes.Error {
	for _, r := range routes {
		for _, g := range r.Gates {
			if g == GateAdmission && r.Message != "TransferRequestMessage" {
				return codes.New(codes.ConfigGateMessageMismatch, "agreement_admission on %s (%s)", r.Name(), r.Message)
			}
		}
		if r.ValidateResponse != nil && r.Message != "CatalogRequestMessage" && r.Message != "DatasetRequestMessage" {
			return codes.New(codes.ConfigGateMessageMismatch, "validate_response on %s (%s)", r.Name(), r.Message)
		}
	}
	return nil
}

// CheckUpstream is CFG-5.
func CheckUpstream(routes []Route, hasManagement bool) *codes.Error {
	for _, r := range routes {
		if r.Listener == MgmtIngress && !hasManagement {
			return codes.New(codes.ConfigUpstreamMissing, "%s uses mgmt-ingress without upstreams.management", r.Name())
		}
	}
	return nil
}

// DefaultRoutes returns the annex table for DSP 2025-1 in observe mode (subject to Q3).
func DefaultRoutes() []Route {
	cat := &ValidateResponse{Gate: GateCatalog, Mode: Shadow}
	r := func(l Listener, m, p, msg string, vr *ValidateResponse, g ...Gate) Route {
		return Route{Listener: l, Match: Match{m, p}, Message: msg, Gates: g, Mode: Observe, ValidateResponse: vr}
	}
	return []Route{
		r(DSPIngress, "POST", "/catalog/request", "CatalogRequestMessage", cat, GateDSP),
		r(DSPIngress, "GET", "/catalog/datasets/:id", "DatasetRequestMessage", cat, GateDSP),
		r(DSPIngress, "POST", "/negotiations/request", "ContractRequestMessage", nil, GateDSP, GatePolicy),
		r(DSPIngress, "POST", "/negotiations/:providerPid/request", "ContractRequestMessage", nil, GateDSP, GatePolicy),
		r(DSPIngress, "POST", "/negotiations/:pid/events", "ContractNegotiationEventMessage", nil, GateDSP),
		r(DSPIngress, "POST", "/negotiations/:providerPid/agreement/verification", "ContractAgreementVerificationMessage", nil, GateDSP),
		r(DSPIngress, "POST", "/negotiations/:pid/termination", "ContractNegotiationTerminationMessage", nil, GateDSP),
		r(DSPIngress, "POST", "/negotiations/offers", "ContractOfferMessage", nil, GateDSP, GatePolicy),
		r(DSPIngress, "POST", "/negotiations/:consumerPid/offers", "ContractOfferMessage", nil, GateDSP, GatePolicy),
		r(DSPIngress, "POST", "/negotiations/:consumerPid/agreement", "ContractAgreementMessage", nil, GateDSP, GatePolicy),
		r(DSPEgress, "POST", "/negotiations/:consumerPid/agreement", "ContractAgreementMessage", nil, GateDSP, GatePolicy),
		r(DSPIngress, "POST", "/transfers/request", "TransferRequestMessage", nil, GateDSP, GateAdmission),
		r(DSPIngress, "POST", "/transfers/:pid/start", "TransferStartMessage", nil, GateDSP),
		r(DSPIngress, "POST", "/transfers/:pid/completion", "TransferCompletionMessage", nil, GateDSP),
		r(DSPIngress, "POST", "/transfers/:pid/suspension", "TransferSuspensionMessage", nil, GateDSP),
		r(DSPIngress, "POST", "/transfers/:pid/termination", "TransferTerminationMessage", nil, GateDSP),
	}
}
