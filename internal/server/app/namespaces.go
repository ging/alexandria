// Package app implements the Namespaces use case: creation by the admin and assignment, removal and listing of stewards.
// Slugs are validated, unique and immutable, and _vhub is reserved (REQ-SYS-010, REQ-HUB-023, REQ-HUB-024, REQ-HUB-025).
package app

import (
	"errors"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/server/domain"
)

// Namespaces implements /v1/namespaces and /v1/namespaces/{ns}/stewards.
type Namespaces struct{ d Deps }

// NamespaceInput is the body of POST /v1/namespaces.
type NamespaceInput struct {
	Slug        string `json:"slug"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// StewardInput is the body of POST /v1/namespaces/{ns}/stewards.
type StewardInput struct {
	Iss   string `json:"iss"`
	Sub   string `json:"sub"`
	Label string `json:"label,omitempty"`
}

// Create adds a namespace; the reserved slug _vhub reports NS_EXISTS (F5).
func (n *Namespaces) Create(p *domain.Principal, in NamespaceInput) (domain.Namespace, error) {
	if err := n.d.authorize(domain.OpAdmin, p, ""); err != nil {
		return domain.Namespace{}, err
	}
	if in.Slug == domain.ReservedSlug {
		return domain.Namespace{}, codes.New(codes.NSExists, "slug %s is reserved", in.Slug)
	}
	if !domain.ValidSlug(in.Slug) {
		return domain.Namespace{}, codes.New(codes.NSSlugInvalid, "slug %q does not match ^[a-z0-9][a-z0-9-]{1,62}$", in.Slug)
	}
	ns := domain.Namespace{Slug: in.Slug, ID: n.d.nsIRI(in.Slug), Title: in.Title, Description: in.Description, CreatedAt: n.d.Clock.Now()}
	if err := n.d.Store.CreateNamespace(ns); err != nil {
		if errors.Is(err, ErrExists) {
			return domain.Namespace{}, codes.New(codes.NSExists, "namespace %s already exists", in.Slug)
		}
		return domain.Namespace{}, err
	}
	return ns, n.d.emit(p, domain.EvNamespaceCreated, "", ns.ID)
}

// List returns every namespace (public).
func (n *Namespaces) List() ([]domain.Namespace, error) { return n.d.Store.ListNamespaces() }

// Get returns one namespace (public).
func (n *Namespaces) Get(slug string) (domain.Namespace, error) { return n.d.namespace(slug) }

// AssignSteward adds a (iss, sub) pair to a namespace.
func (n *Namespaces) AssignSteward(p *domain.Principal, ns string, in StewardInput) (domain.Steward, error) {
	if err := n.d.authorize(domain.OpAdmin, p, ns); err != nil {
		return domain.Steward{}, err
	}
	s := domain.Steward{ID: newID(), Iss: in.Iss, Sub: in.Sub, Label: in.Label, AssignedAt: n.d.Clock.Now(), AssignedBy: p.String()}
	if err := n.d.Store.AddSteward(ns, s); err != nil {
		if errors.Is(err, ErrExists) {
			return domain.Steward{}, codes.New(codes.StewardExists, "(%s, %s) is already a steward of %s", in.Iss, in.Sub, ns)
		}
		return domain.Steward{}, err
	}
	return s, n.d.emit(p, domain.EvStewardAssigned, "", n.d.nsIRI(ns)+"/stewards/"+s.ID)
}

// RemoveSteward withdraws a steward; its past actions remain.
func (n *Namespaces) RemoveSteward(p *domain.Principal, ns, id string) error {
	if err := n.d.authorize(domain.OpAdmin, p, ns); err != nil {
		return err
	}
	if err := n.d.Store.RemoveSteward(ns, id); err != nil {
		return notFound(err, "steward "+id)
	}
	return n.d.emit(p, domain.EvStewardRemoved, "", n.d.nsIRI(ns)+"/stewards/"+id)
}

// ListStewards returns the stewards of a namespace (steward of ns or admin).
func (n *Namespaces) ListStewards(p *domain.Principal, ns string) ([]domain.Steward, error) {
	if err := n.d.authorize(domain.OpListStewards, p, ns); err != nil {
		return nil, err
	}
	return n.d.Store.ListStewards(ns)
}
