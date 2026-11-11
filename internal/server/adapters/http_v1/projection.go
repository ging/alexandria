// Package http_v1 builds the compact JSON-LD projections of namespaces, stewards, vocabularies, candidates and events.
// Lists are @graph documents; every node has an IRI id so the responses expand with the Appendix A.5 context (§3.4, REQ-HUB-029).
package http_v1

import (
	"strconv"

	"github.com/ging/alexandria/internal/server/domain"
)

// doc adds the @context C to a node.
func (h Handler) doc(node map[string]any) map[string]any {
	node["@context"] = h.Context
	return node
}

// graph wraps nodes in a compact @graph document.
func (h Handler) graph(items []any) map[string]any {
	return map[string]any{"@context": h.Context, "@graph": items}
}

// put sets key only when the value is not empty.
func put(m map[string]any, k, v string) {
	if v != "" {
		m[k] = v
	}
}

// namespace projects a namespace.
func (h Handler) namespace(n domain.Namespace) map[string]any {
	m := map[string]any{"id": n.ID, "type": "Namespace", "slug": n.Slug, "created_at": n.CreatedAt}
	put(m, "title", n.Title)
	put(m, "description", n.Description)
	return m
}

// steward projects a steward with its IRI.
func (h Handler) steward(ns string, s domain.Steward) map[string]any {
	m := map[string]any{"id": h.nsIRI(ns) + "/stewards/" + s.ID, "type": "Steward", "iss": s.Iss, "sub": s.Sub, "assigned_at": s.AssignedAt, "assigned_by": s.AssignedBy}
	put(m, "label", s.Label)
	return m
}

// revision projects a revision with its dereferenceable URL as id.
func (h Handler) revision(ns, name string, r domain.Revision) map[string]any {
	return map[string]any{
		"id": h.Base + "/pub/" + ns + "/vocab/" + name + "/r/" + strconv.Itoa(r.N), "type": "Revision",
		"n": r.N, "digest": string(r.Digest), "size": r.Size, "created_at": r.CreatedAt, "created_by": r.CreatedBy,
	}
}

// vocabulary projects a vocabulary and its revisions.
func (h Handler) vocabulary(ns string, v domain.Vocabulary) map[string]any {
	revs := make([]any, 0, len(v.Revisions))
	for _, r := range v.Revisions {
		revs = append(revs, h.revision(ns, v.Name, r))
	}
	m := map[string]any{"id": h.nsIRI(ns) + "/vocabularies/" + v.Name, "type": "Vocabulary", "name": v.Name, "kind": v.Kind, "media_type": v.MediaType, "revisions": revs}
	put(m, "upstream", v.Upstream)
	put(m, "title", v.Title)
	put(m, "description", v.Description)
	return m
}

// candidate projects a candidate with its planned record.
func (h Handler) candidate(c domain.Candidate) map[string]any {
	return map[string]any{
		"id": h.nsIRI(c.Namespace) + "/candidates/" + c.ID, "namespace": h.nsIRI(c.Namespace), "state": c.State,
		"version": c.Version.String(), "class": c.Class, "diff": c.Diff, "planned": c.Planned, "proposed_at": c.ProposedAt, "lifecycle": c.Params,
	}
}

// event projects an event; its id is an IRI ending in the numeric event id.
func (h Handler) event(e domain.Event) map[string]any {
	m := map[string]any{"id": h.Base + "/v1/events/" + strconv.FormatInt(e.ID, 10), "type": e.Type, "ts": e.TS, "principal": e.Principal, "subject": e.Subject}
	if e.Namespace != "" {
		m["namespace"] = h.nsIRI(e.Namespace)
	}
	return m
}
