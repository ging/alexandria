// Package app implements HubTrust: cached hub DID document and signature verification following V1–V4 of §4.4.1.
// A failed verification triggers at most one re-resolution per minute; the latest resolution always replaces the cache (REQ-CLI-024, REQ-CLI-027).
package app

import (
	"sync"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/core/timex"
)

// HubTrust verifies hub signatures against the cached DID document.
type HubTrust struct {
	mu          sync.Mutex
	did         did.DID
	resolver    DIDResolver
	clock       timex.Clock
	ttl         timex.Duration
	events      EventSink
	space       string
	cache       *did.Document
	resolvedAt  timex.Instant
	lastAttempt timex.Instant
	Resolutions int
}

// NewHubTrust builds the verifier with an optional persisted cache.
func NewHubTrust(d did.DID, r DIDResolver, c timex.Clock, ttl timex.Duration, ev EventSink, space string) *HubTrust {
	return &HubTrust{did: d, resolver: r, clock: c, ttl: ttl, events: ev, space: space}
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

// Keys returns the assertion keys of the cache.
func (h *HubTrust) Keys() []did.PublicKey {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cache == nil {
		return nil
	}
	return h.cache.AssertionKeys()
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

// Verify checks sig over msg; invalid selects SYNC_SIGNATURE_INVALID or SYNC_MANIFEST_SIGNATURE_INVALID.
func (h *HubTrust) Verify(msg []byte, sig sign.Signature, invalid codes.Code) *codes.Error {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.clock.Now()
	if h.cache == nil || now.Sub(h.resolvedAt) >= h.ttl {
		if err := h.resolve(now); err != nil {
			if h.cache == nil {
				return codes.New(codes.SyncDidUnresolvable, "%s: %v", h.did, err)
			}
			h.events.Emit(notice(EvSync, h.space, now, codes.DidRefreshFailed, err.Error()))
		}
	}
	if verifies(h.cache, msg, sig) {
		return nil
	}
	known := h.cache.HasKid(sig.Kid)
	if now.Sub(h.resolvedAt) >= timex.Minute && now.Sub(h.lastAttempt) >= timex.Minute {
		// V3: the new resolution replaces the cache whether or not the signature then verifies.
		if err := h.resolve(now); err == nil && verifies(h.cache, msg, sig) {
			if !known {
				h.events.Emit(notice(EvSync, h.space, now, codes.DidKeyRotated, sig.Kid))
			}
			return nil
		}
	}
	return codes.New(invalid, "signature by %s does not verify", sig.Kid)
}

// verifies is V2 against one document.
func verifies(doc *did.Document, msg []byte, sig sign.Signature) bool {
	if doc == nil {
		return false
	}
	k, ok := doc.AssertionKey(sig.Kid)
	return ok && sign.Verify(msg, sig, k) == nil
}
