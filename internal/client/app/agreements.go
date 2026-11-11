// Package app implements Agreements: capture of accepted agreements, lookup with usability, revocation notices and retained manifests.
// There is no import: the client is deployed before any agreement exists (§4.9, PRE-04, Q6, REQ-CLI-015, REQ-CLI-016, REQ-CLI-026).
package app

import (
	"encoding/json"
	"sync"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// Agreements manages the registry.
type Agreements struct {
	Repo      AgreementRepo
	Events    EventSink
	Clock     timex.Clock
	Namespace string
	mu        sync.Mutex
}

// agreementMessage is the part of a ContractAgreementMessage the registry needs.
type agreementMessage struct {
	ProviderPID string         `json:"providerPid"`
	ConsumerPID string         `json:"consumerPid"`
	Agreement   map[string]any `json:"agreement"`
}

// Capture records an agreement for version before the response is returned; a body without agreement @id is skipped.
func (a *Agreements) Capture(body []byte, role, source, version string, snap *domain.Snapshot, tL timex.Instant) {
	var m agreementMessage
	if json.Unmarshal(body, &m) != nil || m.Agreement == nil {
		return
	}
	id, _ := m.Agreement["@id"].(string)
	if id == "" {
		return
	}
	rec, _ := snap.KnownRecord(version)
	e := domain.Entry{AgreementID: id, ProviderPID: m.ProviderPID, ConsumerPID: m.ConsumerPID, Role: role, Version: version, Manifest: rec.Manifest, RecordedAt: tL, Source: source}
	for _, k := range []string{"odrl:profile", "profile", "http://www.w3.org/ns/odrl/2/profile"} {
		if p, ok := m.Agreement[k].(string); ok {
			e.DeclaredProfile = &p
		}
	}
	a.mu.Lock()
	err := a.Repo.Put(e)
	a.mu.Unlock()
	if err != nil {
		return
	}
	if e.DeclaredProfile != nil {
		if p := snap.Packages[rec.Manifest]; p == nil || !p.Manifest.HasProfile(*e.DeclaredProfile) {
			n := notice(EvAgreement, a.Namespace, tL, codes.AgreementProfileMismatch, *e.DeclaredProfile)
			n.Agreement, n.Version = id, version
			a.Events.Emit(n)
		}
	}
}

// AgreementView is GET /agreements/{id}.
type AgreementView struct {
	domain.Entry
	Usable bool `json:"usable"`
}

// Get returns an entry and whether it is usable at tL, or API_NOT_FOUND.
func (a *Agreements) Get(id string, snap *domain.Snapshot, tL timex.Instant) (AgreementView, error) {
	e, ok, err := a.Repo.Get(id)
	if err != nil {
		return AgreementView{}, err
	}
	if !ok {
		return AgreementView{}, codes.New(codes.APINotFound, "agreement %s not found", id)
	}
	usable := true
	if snap != nil {
		rec, _ := snap.KnownRecord(e.Version)
		usable = domain.Usable(rec, snap.Params.LeaseMargin, tL)
	}
	return AgreementView{e, usable}, nil
}

// OnRevocations emits one AGREEMENT_REVOCATION_SCHEDULED per agreement of each newly revoked version.
func (a *Agreements) OnRevocations(versions []semver.Version, snap *domain.Snapshot) {
	all, _ := a.Repo.All()
	for _, v := range versions {
		for _, e := range all {
			if e.Version == v.String() {
				rec, _ := snap.KnownRecord(e.Version)
				n := notice(EvAgreement, a.Namespace, a.Clock.Now(), codes.AgreementRevocationScheduled, "revoked_from "+rec.RevokedFrom.String())
				n.Agreement, n.Version = e.AgreementID, e.Version
				a.Events.Emit(n)
			}
		}
	}
}

// Retained returns the manifests referenced by registered agreements (REQ-CLI-017).
func (a *Agreements) Retained() []ni.Digest {
	all, _ := a.Repo.All()
	out := make([]ni.Digest, 0, len(all))
	for _, e := range all {
		out = append(out, e.Manifest)
	}
	return out
}
