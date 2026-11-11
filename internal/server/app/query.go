// Package app implements the Query use case: global and per-namespace events paged by after, and candidate lookup.
// Event ids are strictly increasing across the hub (REQ-HUB-019, REQ-HUB-020).
package app

import (
	"github.com/ging/alexandria/internal/server/domain"
)

// Query implements /v1/events, .../events and .../candidates/{id}.
type Query struct{ d Deps }

// Events returns global events (admin) when ns is empty, or the events of ns (steward of ns).
func (q *Query) Events(p *domain.Principal, ns string, after int64, limit int) ([]domain.Event, error) {
	op := domain.OpNamespace
	if ns == "" {
		op = domain.OpAdmin
	}
	if err := q.d.authorize(op, p, ns); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return q.d.Store.After(ns, after, limit)
}

// Candidate returns a candidate of ns.
func (q *Query) Candidate(p *domain.Principal, ns, id string) (domain.Candidate, error) {
	if err := q.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Candidate{}, err
	}
	c, err := q.d.Store.GetCandidate(ns, id)
	return c, notFound(err, "candidate "+id)
}
