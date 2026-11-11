// Package app implements the Vocabularies use case: hosted uploads, proxied downloads on create and :refresh, and stable dereferencing.
// VocabReader, used by /pub and the compiler, is built without the fetcher, so no read can reach the network (REQ-HUB-026, REQ-HUB-027, REQ-HUB-028).
package app

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/server/domain"
)

// Vocabularies implements the write side of /v1/namespaces/{ns}/vocabularies.
type Vocabularies struct {
	d      Deps
	reader *VocabReader
}

// VocabInput is the body of POST .../vocabularies.
type VocabInput struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	MediaType   string `json:"media_type"`
	Upstream    string `json:"upstream,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// Create registers a vocabulary; a proxy downloads its revision 1 or nothing is created.
func (v *Vocabularies) Create(p *domain.Principal, ns string, in VocabInput) (domain.Vocabulary, error) {
	if err := v.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Vocabulary{}, err
	}
	if err := v.checkInput(in); err != nil {
		return domain.Vocabulary{}, err
	}
	if _, err := v.d.Store.GetVocabulary(ns, in.Name); err == nil {
		return domain.Vocabulary{}, codes.New(codes.VocabExists, "vocabulary %s already exists in %s", in.Name, ns)
	}
	voc := domain.Vocabulary{Name: in.Name, Kind: in.Kind, MediaType: in.MediaType, Upstream: in.Upstream, Title: in.Title, Description: in.Description, Revisions: []domain.Revision{}}
	var body []byte
	if in.Kind == domain.Proxy {
		b, err := v.fetch(voc)
		if err != nil {
			return domain.Vocabulary{}, err
		}
		body = b
	}
	if err := v.d.Store.CreateVocabulary(ns, voc); err != nil {
		if errors.Is(err, ErrExists) {
			return domain.Vocabulary{}, codes.New(codes.VocabExists, "vocabulary %s already exists in %s", in.Name, ns)
		}
		return domain.Vocabulary{}, err
	}
	if err := v.d.emit(p, domain.EvVocabularyCreated, ns, v.reader.stable(ns, in.Name)); err != nil {
		return domain.Vocabulary{}, err
	}
	if body != nil {
		if _, _, err := v.revise(p, ns, voc, body); err != nil {
			return domain.Vocabulary{}, err
		}
	}
	return v.d.Store.GetVocabulary(ns, in.Name)
}

// checkInput validates name, kind, media type and upstream (one code per field, agent decision on kind and upstream).
func (v *Vocabularies) checkInput(in VocabInput) error {
	switch {
	case !domain.ValidVocabName(in.Name):
		return codes.New(codes.VocabNameInvalid, "name %q does not match ^[a-z0-9][a-z0-9._-]{0,127}$", in.Name)
	case in.Kind != domain.Hosted && in.Kind != domain.Proxy:
		return codes.New(codes.VocabKindMismatch, "kind must be hosted or proxy, not %q", in.Kind)
	case strings.TrimSpace(in.MediaType) == "":
		return codes.New(codes.VocabMediaTypeMismatch, "media_type is required")
	case in.Kind == domain.Hosted && in.Upstream != "":
		return codes.New(codes.VocabKindMismatch, "a hosted vocabulary has no upstream")
	case in.Kind == domain.Proxy && !v.upstreamAllowed(in.Upstream):
		return codes.New(codes.VocabUpstreamFailed, "upstream %q must be an https URL", in.Upstream)
	}
	return nil
}

// upstreamAllowed accepts https, and http://localhost only when allowed (DP-13).
func (v *Vocabularies) upstreamAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" && v.d.AllowInsecureLocalhost && did.IsLocalhost(u.Host))
}

// PutContent uploads the bytes of a hosted vocabulary; returns the revision and whether it is new.
func (v *Vocabularies) PutContent(p *domain.Principal, ns, name, contentType, ifMatch string, body []byte) (domain.Revision, bool, error) {
	if err := v.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Revision{}, false, err
	}
	voc, err := v.d.Store.GetVocabulary(ns, name)
	if err != nil {
		return domain.Revision{}, false, notFound(err, "vocabulary "+name)
	}
	if voc.Kind != domain.Hosted {
		return domain.Revision{}, false, codes.New(codes.VocabKindMismatch, "%s is a %s vocabulary", name, voc.Kind)
	}
	if domain.Essence(contentType) != domain.Essence(voc.MediaType) {
		return domain.Revision{}, false, codes.New(codes.VocabMediaTypeMismatch, "Content-Type %s is not %s", contentType, voc.MediaType)
	}
	if last, ok := voc.Latest(); ifMatch != "" && (!ok || strings.Trim(ifMatch, `"`) != last.Digest.Hex()) {
		return domain.Revision{}, false, codes.New(codes.APIPreconditionFailed, "If-Match does not match the last revision")
	}
	if err := v.checkBody(voc, body); err != nil {
		return domain.Revision{}, false, err
	}
	return v.revise(p, ns, voc, body)
}

// Refresh downloads the upstream of a proxy and adds a revision when the bytes change.
func (v *Vocabularies) Refresh(p *domain.Principal, ns, name string) (domain.Revision, bool, error) {
	if err := v.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Revision{}, false, err
	}
	voc, err := v.d.Store.GetVocabulary(ns, name)
	if err != nil {
		return domain.Revision{}, false, notFound(err, "vocabulary "+name)
	}
	if voc.Kind != domain.Proxy {
		return domain.Revision{}, false, codes.New(codes.VocabKindMismatch, "%s is a %s vocabulary", name, voc.Kind)
	}
	body, err := v.fetch(voc)
	if err != nil {
		return domain.Revision{}, false, err
	}
	return v.revise(p, ns, voc, body)
}

// fetch downloads and checks the upstream bytes of a proxy.
func (v *Vocabularies) fetch(voc domain.Vocabulary) ([]byte, error) {
	if v.d.Fetcher == nil {
		return nil, codes.New(codes.VocabUpstreamFailed, "no upstream fetcher configured")
	}
	body, ct, err := v.d.Fetcher.Fetch(voc.Upstream, v.d.VocabMaxSize+1)
	if err != nil {
		return nil, codes.New(codes.VocabUpstreamFailed, "%s: %v", voc.Upstream, err)
	}
	if !domain.CompatibleMediaType(voc.MediaType, ct) {
		return nil, codes.New(codes.VocabMediaTypeMismatch, "upstream served %s, not %s", ct, voc.MediaType)
	}
	return body, v.checkBody(voc, body)
}

// checkBody applies the size and content checks of §3.6.
func (v *Vocabularies) checkBody(voc domain.Vocabulary, body []byte) error {
	if int64(len(body)) > v.d.VocabMaxSize {
		return codes.New(codes.VocabTooLarge, "%d bytes exceed the maximum of %d", len(body), v.d.VocabMaxSize)
	}
	if !domain.Parseable(voc.MediaType, body) {
		return codes.New(codes.VocabUnparseable, "content is not valid %s", voc.MediaType)
	}
	return nil
}

// revise appends a revision unless the bytes equal the last one (REQ-HUB-028).
func (v *Vocabularies) revise(p *domain.Principal, ns string, voc domain.Vocabulary, body []byte) (domain.Revision, bool, error) {
	d := ni.Sum(body)
	if last, ok := voc.Latest(); ok && last.Digest == d {
		return last, false, nil
	}
	if err := v.d.Blobs.Put(d.Hex(), body); err != nil {
		return domain.Revision{}, false, err
	}
	r := domain.Revision{N: len(voc.Revisions) + 1, Digest: d, Size: int64(len(body)), CreatedAt: v.d.Clock.Now(), CreatedBy: p.String()}
	if err := v.d.Store.AppendRevision(ns, voc.Name, r); err != nil {
		return domain.Revision{}, false, err
	}
	return r, true, v.d.emit(p, domain.EvVocabularyRevised, ns, v.reader.revisionURL(ns, voc.Name, r.N))
}

// VocabReader reads vocabularies and their revisions without any network access.
type VocabReader struct{ d Deps }

// List returns the vocabularies of a namespace (public).
func (r *VocabReader) List(ns string) ([]domain.Vocabulary, error) {
	if _, err := r.d.namespace(ns); err != nil {
		return nil, err
	}
	return r.d.Store.ListVocabularies(ns)
}

// Get returns one vocabulary (public).
func (r *VocabReader) Get(ns, name string) (domain.Vocabulary, error) {
	if _, err := r.d.namespace(ns); err != nil {
		return domain.Vocabulary{}, err
	}
	voc, err := r.d.Store.GetVocabulary(ns, name)
	return voc, notFound(err, "vocabulary "+name)
}

// Revision returns the bytes of revision n of a vocabulary.
func (r *VocabReader) Revision(ns, name string, n int) (domain.Vocabulary, []byte, error) {
	voc, err := r.Get(ns, name)
	if err != nil {
		return domain.Vocabulary{}, nil, err
	}
	if n < 1 || n > len(voc.Revisions) {
		return domain.Vocabulary{}, nil, codes.New(codes.APINotFound, "revision %d of %s not found", n, name)
	}
	b, err := r.d.Blobs.Get(voc.Revisions[n-1].Digest.Hex())
	return voc, b, err
}

// ByIRI resolves a revision IRI of namespace ns for the compiler (D-VOC-1); other namespaces are refused.
func (r *VocabReader) ByIRI(ns, iri string) ([]byte, error) {
	rest, ok := strings.CutPrefix(iri, r.d.Base+"/pub/"+ns+"/vocab/")
	if !ok {
		return nil, errors.New("not a revision of this namespace")
	}
	name, nstr, ok := strings.Cut(rest, "/r/")
	n, err := strconv.Atoi(nstr)
	if !ok || err != nil {
		return nil, errors.New("not a revision IRI")
	}
	_, b, err := r.Revision(ns, name, n)
	return b, err
}

// stable returns the stable URL of a vocabulary.
func (r *VocabReader) stable(ns, name string) string {
	return r.d.Base + "/pub/" + ns + "/vocab/" + name
}

// revisionURL returns the URL of revision n.
func (r *VocabReader) revisionURL(ns, name string, n int) string {
	return r.stable(ns, name) + "/r/" + strconv.Itoa(n)
}

// RevisionURL is the public form of revisionURL for adapters.
func (r *VocabReader) RevisionURL(ns, name string, n int) string { return r.revisionURL(ns, name, n) }
