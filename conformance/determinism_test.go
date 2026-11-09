// Determinism checks: the same inputs produce the same signable bytes on two independent hubs.
// Covers compilation, listings and version records (§5.4, T-HUB-010, T-HUB-005).
package conformance

import (
	"bytes"
	"testing"

	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/testkit"
)

// signedPart returns JCS(signed) of an envelope.
func signedPart(t *testing.T, env []byte) []byte {
	t.Helper()
	r, err := sign.ParseRaw(env)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := r.Input()
	return b
}

func TestDeterminism_SameInputsSameSignableBytes(t *testing.T) {
	a, b := testkit.NewServer("2026-09-01T10:00:00Z"), testkit.NewServer("2026-09-01T10:00:00Z")
	a.PublishF6(t)
	b.PublishF6(t)
	a.Clock.Set("2026-10-10T12:00:00Z")
	b.Clock.Set("2026-10-10T12:00:00Z")
	la, _ := a.LatestListing(t)
	lb, _ := b.LatestListing(t)
	if !bytes.Equal(signedPart(t, la), signedPart(t, lb)) {
		t.Fatalf("listings differ:\n%s\n%s", signedPart(t, la), signedPart(t, lb))
	}
	for _, v := range []string{"1.0.0", "2.0.0"} {
		ra, _ := a.Hub.Listing.Record(testkit.Member, v)
		rb, _ := b.Hub.Listing.Record(testkit.Member, v)
		if !bytes.Equal(signedPart(t, ra), signedPart(t, rb)) {
			t.Errorf("records of %s differ", v)
		}
		ma, _, _ := a.Hub.Publication.VersionedFile("demo", v, "manifest.json")
		mb, _, _ := b.Hub.Publication.VersionedFile("demo", v, "manifest.json")
		if !bytes.Equal(ma, mb) {
			t.Errorf("manifests of %s differ", v)
		}
	}
}
