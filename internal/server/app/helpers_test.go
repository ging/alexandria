// Shared fixtures of the server use-case tests: the in-memory hub, code extraction and listing decoding.
// Every T-HUB-* test of this package runs against testkit doubles, without HTTP nor SQLite (Part 4, M2).
package app_test

import (
	"encoding/json"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// t0 is the instant before the F6 publications.
const t0 = "2026-08-01T00:00:00Z"

// newHub returns an F6 hub with namespace demo and steward Sergio.
func newHub(t *testing.T) *testkit.Server { return testkit.NewServer(t, t0) }

// codeOf returns the code of a coded error, the first finding of a Findings, or "".
func codeOf(err error) codes.Code {
	if f, ok := err.(app.Findings); ok {
		return f[0].Code
	}
	if ce, ok := codes.As(err); ok {
		return ce.Code
	}
	return ""
}

// listing decodes the current listing of demo.
func listing(t *testing.T, s *testkit.Server) vc.Credential[lifecycle.ListingSubject] {
	t.Helper()
	raw, err := s.Hub.Listing.Latest(testkit.NS)
	if err != nil {
		t.Fatal(err)
	}
	var c vc.Credential[lifecycle.ListingSubject]
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// versionsOf returns the versions of a listing subject.
func versionsOf(l lifecycle.ListingSubject) []string {
	var out []string
	for _, r := range l.Versions {
		out = append(out, r.Version.String())
	}
	return out
}

// propose creates a draft and proposes it as Sergio.
func propose(t *testing.T, s *testkit.Server, doc []byte) string {
	t.Helper()
	return s.Propose(t, doc)
}
