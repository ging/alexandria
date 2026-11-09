// Status: /status and /versions views, readiness, the vhub.state watcher and the clock-skew estimate.
// CLOCK_SKEW_EXCEEDED is informative and never changes a decision (§4.11, REQ-CLI-020, REQ-CLI-025).
package app

import (
	"strconv"
	"sync"

	"github.com/ging/alexandria/vhub/internal/client/domain"
	"github.com/ging/alexandria/vhub/internal/core/codes"
	"github.com/ging/alexandria/vhub/internal/core/lifecycle"
	"github.com/ging/alexandria/vhub/internal/core/ni"
	"github.com/ging/alexandria/vhub/internal/core/timex"
)

// Status reports the client state.
type Status struct {
	Cell     *Cell
	Clock    timex.Clock
	Trust    *HubTrust
	Packages PackageStore
	Events   EventSink
	Space    string
	HubDID   string
	Gates    map[domain.Gate]domain.Mode
	mu       sync.Mutex
	skewMs   int64
	last     domain.State
}

// VersionRole is one element of /status versions[].
type VersionRole struct {
	Version   string    `json:"version"`
	Manifest  ni.Digest `json:"manifest"`
	LocalRole string    `json:"local_role"`
}

// ListingInfo describes the last accepted listing.
type ListingInfo struct {
	Sequence  uint64        `json:"sequence"`
	IssuedAt  timex.Instant `json:"issued_at"`
	ExpiresAt timex.Instant `json:"expires_at"`
}

// HubDIDInfo describes the cached DID document.
type HubDIDInfo struct {
	DID        string         `json:"did"`
	ResolvedAt *timex.Instant `json:"resolved_at"`
}

// StatusDoc is GET /status.
type StatusDoc struct {
	Space               string                      `json:"space"`
	State               domain.State                `json:"state"`
	Versions            []VersionRole               `json:"versions"`
	LastAcceptedListing *ListingInfo                `json:"last_accepted_listing"`
	HubDID              HubDIDInfo                  `json:"hub_did"`
	ClockSkewEstimateMs int64                       `json:"clock_skew_estimate_ms"`
	Gates               map[domain.Gate]domain.Mode `json:"gates"`
}

// VersionView is one element of GET /versions.
type VersionView struct {
	lifecycle.Record
	State    lifecycle.State `json:"state"`
	Retained bool            `json:"retained"`
}

// Report builds /status.
func (s *Status) Report() StatusDoc {
	snap, tL := s.Cell.Load(), s.Clock.Now()
	doc := StatusDoc{Space: s.Space, State: domain.StateAt(snap, tL), Versions: []VersionRole{}, Gates: s.Gates}
	s.mu.Lock()
	doc.ClockSkewEstimateMs = s.skewMs
	s.mu.Unlock()
	doc.HubDID.DID = s.HubDID
	if _, at := s.Trust.Cache(); at != 0 {
		doc.HubDID.ResolvedAt = &at
	}
	if snap == nil {
		return doc
	}
	doc.LastAcceptedListing = &ListingInfo{snap.Sequence, snap.IssuedAt, snap.ExpiresAt}
	vt := snap.InForce(tL)
	roles := map[string]string{}
	for i, r := range vt {
		roles[r.Version.String()] = map[bool]string{true: "head", false: "in_force"}[i == 0]
	}
	for _, r := range lifecycle.KnownSlice(snap.Known) {
		role, ok := roles[r.Version.String()]
		inListing := false
		for _, l := range snap.Records {
			inListing = inListing || l.Version == r.Version
		}
		switch {
		case ok:
		case inListing && r.NewContentFrom.Add(-snap.Params.LeaseMargin) > tL:
			role = "scheduled"
		case snap.Packages[r.Manifest] != nil:
			role = "retained"
		default:
			continue
		}
		doc.Versions = append(doc.Versions, VersionRole{r.Version.String(), r.Manifest, role})
	}
	return doc
}

// Versions builds /versions with reference states and retention.
func (s *Status) Versions() []VersionView {
	snap, tL := s.Cell.Load(), s.Clock.Now()
	out := []VersionView{}
	if snap == nil {
		return out
	}
	known := lifecycle.KnownSlice(snap.Known)
	for _, r := range known {
		_, held, _ := s.Packages.Get(r.Manifest)
		out = append(out, VersionView{Record: r, State: lifecycle.StateAt(known, r, tL), Retained: held})
	}
	return out
}

// Ready reports whether /readyz answers 200.
func (s *Status) Ready() bool {
	st := domain.StateAt(s.Cell.Load(), s.Clock.Now())
	return st == domain.Active || st == domain.Transition || st == domain.Degraded
}

// ObserveSkew records Date − t_l and emits CLOCK_SKEW_EXCEEDED above clock_skew.
func (s *Status) ObserveSkew(date, tL timex.Instant) {
	skew := int64(date.Sub(tL)) * 1000
	s.mu.Lock()
	s.skewMs = skew
	s.mu.Unlock()
	limit := lifecycle.Defaults().ClockSkew
	if snap := s.Cell.Load(); snap != nil {
		limit = snap.Params.ClockSkew
	}
	if skew > int64(limit)*1000 || -skew > int64(limit)*1000 {
		s.Events.Emit(notice(EvSync, s.Space, tL, codes.ClockSkewExceeded, strconv.FormatInt(skew, 10)+" ms"))
	}
}

// Watch emits vhub.state when the computed state changes.
func (s *Status) Watch() {
	st := domain.StateAt(s.Cell.Load(), s.Clock.Now())
	s.mu.Lock()
	prev := s.last
	s.last = st
	s.mu.Unlock()
	if prev != st {
		n := notice(EvState, s.Space, s.Clock.Now(), "", "")
		n.From, n.To = string(prev), string(st)
		s.Events.Emit(n)
	}
}

// ListingAge returns t_l − issued_at of the last accepted listing in seconds, or -1.
func (s *Status) ListingAge() int64 {
	snap := s.Cell.Load()
	if snap == nil {
		return -1
	}
	return int64(s.Clock.Now().Sub(snap.IssuedAt))
}
