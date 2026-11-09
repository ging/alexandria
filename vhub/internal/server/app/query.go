// Query use case: event log pagination and candidate lookup for releasers.
// Event ids are strictly increasing and pagination uses the after cursor (REQ-HUB-019, REQ-HUB-020).
package app

import "github.com/ging/alexandria/vhub/internal/server/domain"

// Query implements /v1/events and /v1/candidates/{id}.
type Query struct{ d Deps }

// Events returns up to limit events after the given id.
func (q *Query) Events(p domain.Principal, after int64, limit int) ([]domain.Event, error) {
	if err := authorize(p, domain.EventsList); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	ev, err := q.d.Events.After(after, limit)
	if ev == nil {
		ev = []domain.Event{}
	}
	return ev, err
}

// Candidate returns a candidate by id.
func (q *Query) Candidate(p domain.Principal, id string) (domain.Candidate, error) {
	if err := authorize(p, domain.CandidateGet); err != nil {
		return domain.Candidate{}, err
	}
	c, err := q.d.Candidates.Get(id)
	return c, notFound(err, "candidate "+id)
}
