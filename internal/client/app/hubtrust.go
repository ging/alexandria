// Package app implements HubTrust: the cached hub DID document and proof verification following V0–V4 of §4.4.1.
// A failing proof triggers at most one re-resolution per minute (did:web only) and the latest resolution always replaces the cache (REQ-CLI-024, REQ-CLI-027).
package app

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
)

// HubTrust verifies hub proofs against the cached DID document.
type HubTrust struct {
	mu          sync.Mutex
	did         did.DID
	resolver    DIDResolver
	clock       timex.Clock
	ttl         timex.Duration
	events      EventSink
	Logger      *slog.Logger
	ns          string
	cache       *did.Document
	resolvedAt  timex.Instant
	lastAttempt timex.Instant
	Resolutions int
}

// NewHubTrust builds the verifier for the configured hub DID.
func NewHubTrust(d did.DID, r DIDResolver, c timex.Clock, ttl timex.Duration, ev EventSink, ns string) *HubTrust {
	return &HubTrust{did: d, resolver: r, clock: c, ttl: ttl, events: ev, ns: ns}
}

// Restore installs a persisted DID document.
func (h *HubTrust) Restore(doc *did.Document, at timex.Instant) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache, h.resolvedAt = doc, at
}

// Cache returns the cached document and its resolution instant.
func (h *HubTrust) Cache() (*did.Document, timex.Instant) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cache, h.resolvedAt
}

// resolve fetches the DID document and replaces the cache on success.
func (h *HubTrust) resolve(now timex.Instant) error {
	h.lastAttempt = now
	h.Resolutions++
	doc, err := h.resolver.Resolve(h.did)
	if err != nil {
		return err
	}
	h.cache, h.resolvedAt = &doc, now
	return nil
}

// Verify applies V0–V4 to a secured document; invalid is SYNC_SIGNATURE_INVALID or SYNC_MANIFEST_SIGNATURE_INVALID.
func (h *HubTrust) Verify(secured []byte, invalid codes.Code) *codes.Error {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, err := vc.ParseProof(secured)
	var head struct {
		ValidFrom string `json:"validFrom"`
	}
	_ = json.Unmarshal(secured, &head)
	if err != nil || p.Type != vc.ProofType || p.Cryptosuite != vc.Cryptosuite || p.ProofPurpose != vc.AssertionMethod || p.Created != head.ValidFrom || did.OfURL(p.VerificationMethod) != h.did {
		return codes.New(invalid, "V0: proof options are not those of the hub")
	}
	now := h.clock.Now()
	if h.cache == nil || now.Sub(h.resolvedAt) >= h.ttl {
		if err := h.resolve(now); err != nil {
			if h.cache == nil {
				logx.Default(h.Logger).Warn("initial hub DID resolution failed", slog.String("hub_did", string(h.did)), logx.Err(err))
				return codes.New(codes.SyncDidUnresolvable, "%s: %v", h.did, err)
			}
			logx.Default(h.Logger).Warn("hub DID refresh failed; retaining cached document", slog.String("hub_did", string(h.did)), logx.Err(err))
			h.events.Emit(notice(EvSync, h.ns, now, codes.DidRefreshFailed, err.Error()))
		}
	}
	if verifies(h.cache, secured, p.VerificationMethod) {
		return nil
	}
	known := h.cache.HasKid(p.VerificationMethod)
	if h.did.Method() == "web" && now.Sub(h.resolvedAt) >= timex.Minute && now.Sub(h.lastAttempt) >= timex.Minute {
		// V3: the new resolution replaces the cache whether or not the proof then verifies.
		if err := h.resolve(now); err == nil && verifies(h.cache, secured, p.VerificationMethod) {
			if !known {
				logx.Default(h.Logger).Info("hub DID signing key rotated", slog.String("hub_did", string(h.did)), slog.String("verification_method", p.VerificationMethod))
				h.events.Emit(notice(EvSync, h.ns, now, codes.DidKeyRotated, p.VerificationMethod))
			}
			return nil
		}
	}
	return codes.New(invalid, "proof by %s does not verify", p.VerificationMethod)
}

// verifies is V2 against one document.
func verifies(doc *did.Document, secured []byte, vm string) bool {
	if doc == nil {
		return false
	}
	k, ok := doc.AssertionKey(vm)
	return ok && vc.Verify(secured, k.Key) == nil
}
