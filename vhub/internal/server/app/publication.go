// Publication use case behind /pub: head redirect, versioned files, package files and blobs by hash.
// Only content referenced by a stored release is served, so an aborted release is never visible (REQ-SYS-005, REQ-HUB-011, REQ-HUB-016, DP-15).
package app

import (
	"github.com/ging/alexandria/vhub/internal/core/codes"
	"github.com/ging/alexandria/vhub/internal/core/lifecycle"
	"github.com/ging/alexandria/vhub/internal/core/ni"
	"github.com/ging/alexandria/vhub/internal/core/semver"
	"github.com/ging/alexandria/vhub/internal/server/domain"
)

// Publication implements the read side of /pub.
type Publication struct{ d Deps }

// JSONType is the media type served for package documents.
const JSONType = "application/json"

// Head returns the greatest version in force now for a space slug (DP-15).
func (pb *Publication) Head(slug string) (semver.Version, error) {
	_, space, err := headParams(pb.d)
	if err != nil {
		return semver.Version{}, err
	}
	if space == "" || domain.SlugOf(space) != slug {
		return semver.Version{}, codes.New(codes.ApiNotFound, "space %s not found", slug)
	}
	recs, err := pb.d.Releases.Records()
	if err != nil {
		return semver.Version{}, err
	}
	set := lifecycle.InForceSet(recs, pb.d.Clock.Now(), 0)
	if len(set) == 0 {
		return semver.Version{}, codes.New(codes.ApiNotFound, "no version of %s is in force", slug)
	}
	return set[0].Version, nil
}

// VersionedFile returns a file of /pub/{slug}/{version}/{file} with its media type.
func (pb *Publication) VersionedFile(slug, version, path string) ([]byte, string, error) {
	v, err := semver.Parse(version)
	if err != nil {
		return nil, "", codes.New(codes.ApiNotFound, "version %s not found", version)
	}
	rel, err := pb.d.Releases.Release(v)
	if err != nil || domain.SlugOf(rel.Space) != slug {
		return nil, "", codes.New(codes.ApiNotFound, "version %s of %s not found", version, slug)
	}
	switch path {
	case "manifest.json":
		return rel.Manifest, JSONType, nil
	case "space-lock.json":
		return rel.SpaceLock, JSONType, nil
	}
	for _, e := range rel.Entries {
		if e.Path == path {
			b, err := pb.d.Blobs.Get(e.Digest.Hex())
			return b, e.MediaType, err
		}
	}
	return nil, "", codes.New(codes.ApiNotFound, "%s not in %s", path, version)
}

// PackageFile returns manifest.json, manifest.sig, manifest.receipt or space-lock.json of a package.
func (pb *Publication) PackageFile(segment, name string) ([]byte, error) {
	d, err := ni.FromSegment(segment)
	if err != nil {
		return nil, codes.New(codes.ApiNotFound, "package %s not found", segment)
	}
	rel, err := pb.d.Releases.ByManifest(d)
	if err != nil {
		return nil, notFound(err, "package "+segment)
	}
	switch name {
	case "manifest.json":
		return rel.Manifest, nil
	case "manifest.sig":
		return rel.ManifestSig, nil
	case "manifest.receipt":
		return rel.Receipt, nil
	case "space-lock.json":
		return rel.SpaceLock, nil
	}
	return nil, codes.New(codes.ApiNotFound, "%s not in package", name)
}

// Blob returns a file by its SHA-256 if a release references it.
func (pb *Publication) Blob(hex string) ([]byte, error) {
	ok, err := pb.d.Releases.HasBlob(hex)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, codes.New(codes.ApiNotFound, "blob %s not found", hex)
	}
	return pb.d.Blobs.Get(hex)
}
