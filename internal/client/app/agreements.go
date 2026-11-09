// Agreements: capture of accepted agreements, admission of transfers, import, lookup and package retention.
// Also announces AGREEMENT_REVOCATION_SCHEDULED for each agreement of a newly revoked version (§4.9, REQ-CLI-015..017, REQ-CLI-026).
package app

import (
	"encoding/json"
	"sync"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// Agreements manages the registry.
type Agreements struct {
	Repo   AgreementRepo
	Events EventSink
	Clock  timex.Clock
	Space  string
	mu     sync.Mutex
}

// agreementMessage is the part of a ContractAgreementMessage the registry needs.
type agreementMessage struct {
	ProviderPID string         `json:"providerPid"`
	ConsumerPID string         `json:"consumerPid"`
	Agreement   map[string]any `json:"agreement"`
}

// Capture records an agreement for version; errors in the body skip the capture.
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
	e := domain.Entry{AgreementID: id, ProviderPID: m.ProviderPID, ConsumerPID: m.ConsumerPID, Role: role, Version: version,
		Manifest: rec.Manifest, RecordedAt: tL, Source: source}
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
	if e.DeclaredProfile != nil && !a.profileFixed(*e.DeclaredProfile, rec.Manifest, snap) {
		n := notice(EvAgreement, a.Space, tL, codes.AgreementProfileMismatch, *e.DeclaredProfile)
		n.Agreement, n.Version = id, version
		a.Events.Emit(n)
	}
}

// profileFixed reports whether the space-lock of the version fixes the declared profile for slot odrl.
func (a *Agreements) profileFixed(profile string, d ni.Digest, snap *domain.Snapshot) bool {
	p := snap.Packages[d]
	if p == nil {
		return false
	}
	for _, x := range p.SpaceLock.Profiles["odrl"] {
		if x == profile {
			return true
		}
	}
	return false
}

// Admit classifies a TransferRequestMessage with the admission table.
func (a *Agreements) Admit(body []byte, snap *domain.Snapshot, tL timex.Instant, unknown string) domain.Classification {
	id := domain.Field(body, "agreementId", "dspace:agreementId")
	e, ok, _ := a.Repo.Get(id)
	if !ok || id == "" {
		return domain.Admit(nil, lifecycle.Record{}, 0, tL, unknown)
	}
	rec, _ := snap.KnownRecord(e.Version)
	return domain.Admit(&e, rec, snap.Params.LeaseMargin, tL, unknown)
}

// Import registers {agreement_id, version} pairs; an unknown version is API_NOT_FOUND.
func (a *Agreements) Import(pairs []ImportPair, snap *domain.Snapshot) error {
	for _, p := range pairs {
		if snap == nil {
			return codes.New(codes.ApiNotFound, "version %s is not known", p.Version)
		}
		rec, ok := snap.KnownRecord(p.Version)
		if !ok {
			return codes.New(codes.ApiNotFound, "version %s is not known", p.Version)
		}
		if err := a.Repo.Put(domain.Entry{AgreementID: p.AgreementID, Version: p.Version, Manifest: rec.Manifest, RecordedAt: a.Clock.Now(), Source: "import"}); err != nil {
			return err
		}
	}
	return nil
}

// ImportPair is one element of POST /agreements:import.
type ImportPair struct {
	AgreementID string `json:"agreement_id"`
	Version     string `json:"version"`
}

// AgreementView is GET /agreements/{id}.
type AgreementView struct {
	domain.Entry
	Usable bool `json:"usable"`
}

// Get returns an entry and whether it is usable at tL.
func (a *Agreements) Get(id string, snap *domain.Snapshot, tL timex.Instant) (AgreementView, error) {
	e, ok, err := a.Repo.Get(id)
	if err != nil {
		return AgreementView{}, err
	}
	if !ok {
		return AgreementView{}, codes.New(codes.ApiNotFound, "agreement %s not found", id)
	}
	usable := true
	if snap != nil {
		rec, _ := snap.KnownRecord(e.Version)
		usable = domain.Usable(rec, snap.Params.LeaseMargin, tL)
	}
	return AgreementView{e, usable}, nil
}

// OnRevocations emits one AGREEMENT_REVOCATION_SCHEDULED per agreement of each revoked version.
func (a *Agreements) OnRevocations(versions []semver.Version, snap *domain.Snapshot) {
	all, _ := a.Repo.All()
	for _, v := range versions {
		for _, e := range all {
			if e.Version == v.String() {
				rec, _ := snap.KnownRecord(e.Version)
				n := notice(EvAgreement, a.Space, a.Clock.Now(), codes.AgreementRevocationScheduled, "revoked_from "+rec.RevokedFrom.String())
				n.Agreement, n.Version = e.AgreementID, e.Version
				a.Events.Emit(n)
			}
		}
	}
}

// Retained returns the manifests referenced by registered agreements (REQ-CLI-017).
func (a *Agreements) Retained() []ni.Digest {
	all, _ := a.Repo.All()
	var out []ni.Digest
	for _, e := range all {
		if e.Manifest != "" {
			out = append(out, e.Manifest)
		}
	}
	return out
}
