// Package app implements Status: GET /status and /versions, the clock-skew estimate and the state watcher that emits vhub.state.
// The skew estimate only warns (CLOCK_SKEW_EXCEEDED) and never changes a decision (§4.12, REQ-CLI-008, REQ-CLI-020, REQ-CLI-025).
package app

import (
	"sync"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/timex"
)

// Status reports the client state.
type Status struct {
	Cell      *Cell
	Rules     *RulesCell
	Trust     *HubTrust
	Packages  PackageStore
	Events    EventSink
	Clock     timex.Clock
	Namespace string
	HubDID    did.DID

	mu      sync.Mutex
	skewMS  int64
	last    domain.State
	watched bool
}

// StatusVersion is one entry of /status versions[].
type StatusVersion struct {
	Version   string    `json:"version"`
	Manifest  ni.Digest `json:"manifest"`
	LocalRole string    `json:"local_role"`
}

// StatusDoc is GET /status (§4.12).
type StatusDoc struct {
	Namespace           string          `json:"namespace"`
	State               domain.State    `json:"state"`
	Versions            []StatusVersion `json:"versions"`
	LastAcceptedListing *ListingInfo    `json:"last_accepted_listing"`
	HubDID              HubDIDInfo      `json:"hub_did"`
	Rules               RulesInfo       `json:"rules"`
	ClockSkewEstimateMS int64           `json:"clock_skew_estimate_ms"`
}

// ListingInfo describes the last accepted listing.
type ListingInfo struct {
	Sequence   uint64        `json:"sequence"`
	ValidFrom  timex.Instant `json:"valid_from"`
	ValidUntil timex.Instant `json:"valid_until"`
}

// HubDIDInfo describes the cached DID document.
type HubDIDInfo struct {
	DID        did.DID        `json:"did"`
	ResolvedAt *timex.Instant `json:"resolved_at"`
}

// RulesInfo describes the current rule set.
type RulesInfo struct {
	Revision int `json:"revision"`
	Count    int `json:"count"`
}

// ObserveSkew records the hub Date against the local instant and emits CLOCK_SKEW_EXCEEDED beyond clock_skew.
func (s *Status) ObserveSkew(hubDate, local timex.Instant) {
	s.mu.Lock()
	s.skewMS = int64(local.Sub(hubDate)) * 1000
	skew := s.skewMS
	s.mu.Unlock()
	limit := lifecycle.Defaults().ClockSkew
	if snap := s.Cell.Load(); snap != nil {
		limit = snap.Params.ClockSkew
	}
	if abs(skew) > int64(limit)*1000 {
		s.Events.Emit(notice(EvState, s.Namespace, local, codes.ClockSkewExceeded, "estimated skew "+timex.Duration(abs(skew)/1000).String()))
	}
}

// abs returns |n|.
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// Get builds the /status document at the current local instant.
func (s *Status) Get() StatusDoc {
	snap, tL := s.Cell.Load(), s.Clock.Now()
	rs := s.Rules.Load()
	doc := StatusDoc{Namespace: s.Namespace, State: domain.StateAt(snap, tL), Versions: []StatusVersion{}, Rules: RulesInfo{rs.Revision, len(rs.Rules)}}
	cache, at := s.Trust.Cache()
	doc.HubDID.DID = s.HubDID
	if cache != nil {
		doc.HubDID.ResolvedAt = &at
	}
	s.mu.Lock()
	doc.ClockSkewEstimateMS = s.skewMS
	s.mu.Unlock()
	if snap == nil {
		return doc
	}
	doc.LastAcceptedListing = &ListingInfo{snap.Sequence, snap.ValidFrom, snap.ValidUntil}
	vt := snap.InForce(tL)
	listed := map[ni.Digest]bool{}
	for _, r := range snap.Records {
		listed[r.Manifest] = true
		doc.Versions = append(doc.Versions, StatusVersion{r.Version.String(), r.Manifest, role(r, vt, snap, tL)})
	}
	for _, r := range lifecycle.KnownSlice(snap.Known) {
		if !listed[r.Manifest] && snap.Packages[r.Manifest] != nil {
			doc.Versions = append(doc.Versions, StatusVersion{r.Version.String(), r.Manifest, "retained"})
		}
	}
	return doc
}

// role computes local_role: head, in_force, scheduled or retained.
func role(r lifecycle.Record, vt []lifecycle.Record, snap *domain.Snapshot, tL timex.Instant) string {
	for i, v := range vt {
		if v.Version == r.Version {
			if i == 0 {
				return "head"
			}
			return "in_force"
		}
	}
	if r.NewContentFrom.Add(-snap.Params.LeaseMargin) > tL {
		return "scheduled"
	}
	return "retained"
}

// VersionView is one entry of GET /versions.
type VersionView struct {
	lifecycle.Record
	State    lifecycle.State `json:"state"`
	Retained bool            `json:"retained"`
}

// Versions lists known versions with their dates, reference state and whether their package is held.
func (s *Status) Versions() []VersionView {
	snap, tL := s.Cell.Load(), s.Clock.Now()
	out := []VersionView{}
	if snap == nil {
		return out
	}
	known := lifecycle.KnownSlice(snap.Known)
	for _, r := range known {
		out = append(out, VersionView{Record: r, State: lifecycle.StateAt(known, r, tL), Retained: snap.Packages[r.Manifest] != nil})
	}
	return out
}

// Watch emits vhub.state when StateAt changes; the state watcher calls it every second.
func (s *Status) Watch() {
	st := domain.StateAt(s.Cell.Load(), s.Clock.Now())
	s.mu.Lock()
	prev, watched := s.last, s.watched
	s.last, s.watched = st, true
	s.mu.Unlock()
	if watched && prev != st {
		n := notice(EvState, s.Namespace, s.Clock.Now(), "", "state changed")
		n.From, n.To = string(prev), string(st)
		s.Events.Emit(n)
	}
}
