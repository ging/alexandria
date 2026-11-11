// Package app implements the Publication use case behind /pub: head redirect, versioned artefacts, manifests, blobs and vocabularies.
// It is built without the upstream fetcher, so an anonymous request never causes outbound traffic (REQ-SYS-005, REQ-HUB-011, REQ-HUB-016, REQ-HUB-027).
package app

import (
	"strconv"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/server/domain"
)

// Publication serves the public /pub content.
type Publication struct {
	d      Deps
	reader *VocabReader
}

// Artifact is a file served with its media type.
type Artifact struct {
	Bytes     []byte
	MediaType string
}

// Head returns the 303 target of /pub/{ns}: the first context file of the head, or its manifest (DP-12).
func (p *Publication) Head(ns string) (string, error) {
	recs, rels, err := p.d.records(ns)
	if err != nil {
		return "", err
	}
	vt := lifecycle.InForceSet(recs, p.d.Clock.Now(), 0)
	if len(vt) == 0 {
		return "", codes.New(codes.APINotFound, "namespace %s has no version in force", ns)
	}
	for _, r := range rels {
		if semver.Compare(r.Record.Version, vt[0].Version) != 0 {
			continue
		}
		for _, e := range r.Entries {
			if e.Role == pkgv.RoleContext {
				return "/pub/" + ns + "/" + r.Record.Version.String() + "/" + e.Path, nil
			}
		}
		return "/pub/packages/" + r.Record.Manifest.Segment() + "/manifest.json", nil
	}
	return "", codes.New(codes.APINotFound, "head of %s not found", ns)
}

// VersionFile returns an artefact of a version by its package path.
func (p *Publication) VersionFile(ns, version, path string) (Artifact, error) {
	v, err := semver.Parse(version)
	if err != nil {
		return Artifact{}, codes.New(codes.APINotFound, "version %s not found", version)
	}
	_, rels, err := p.d.records(ns)
	if err != nil {
		return Artifact{}, err
	}
	for _, r := range rels {
		if semver.Compare(r.Record.Version, v) != 0 {
			continue
		}
		for _, e := range r.Entries {
			if e.Path == path {
				b, err := p.d.Blobs.Get(e.Digest.Hex())
				return Artifact{Bytes: b, MediaType: e.MediaType}, notFound(err, path)
			}
		}
	}
	return Artifact{}, codes.New(codes.APINotFound, "%s/%s not found", version, path)
}

// Manifest returns the signed manifest.json of a package by its digest segment.
func (p *Publication) Manifest(segment string) ([]byte, error) {
	d, err := ni.FromSegment(segment)
	if err != nil {
		return nil, codes.New(codes.APINotFound, "package %s not found", segment)
	}
	r, err := p.d.Store.ByManifest(d)
	if err != nil {
		return nil, notFound(err, "package "+segment)
	}
	return r.Manifest, nil
}

// Blob returns a content-addressed file by its SHA-256 hex.
func (p *Publication) Blob(hex string) ([]byte, error) {
	b, err := p.d.Blobs.Get(hex)
	if err != nil || ni.HexSum(b) != hex {
		return nil, codes.New(codes.APINotFound, "blob %s not found", hex)
	}
	return b, nil
}

// VocabLatest returns the URL of the last revision of a vocabulary.
func (p *Publication) VocabLatest(ns, name string) (string, error) {
	voc, err := p.reader.Get(ns, name)
	if err != nil {
		return "", err
	}
	last, ok := voc.Latest()
	if !ok {
		return "", codes.New(codes.APINotFound, "vocabulary %s has no revision", name)
	}
	return "/pub/" + ns + "/vocab/" + name + "/r/" + strconv.Itoa(last.N), nil
}

// VocabRevision returns the bytes of a revision with its vocabulary.
func (p *Publication) VocabRevision(ns, name string, n int) (domain.Vocabulary, []byte, error) {
	return p.reader.Revision(ns, name, n)
}
