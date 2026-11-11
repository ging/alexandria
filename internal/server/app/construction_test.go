// Construction guarantee of F8 §8.6 on the server: the read side of vocabularies and /pub are built without the upstream fetcher.
// An internal test, so it can inspect the unexported dependencies (Part 4 M6, REQ-HUB-027).
package app

import "testing"

// fetcher is a non-nil UpstreamFetcher.
type fetcher struct{}

// Fetch never runs.
func (fetcher) Fetch(string, int64) ([]byte, string, error) { return nil, "", nil }

func TestReadSideHasNoFetcher(t *testing.T) {
	h := New(Deps{Fetcher: fetcher{}})
	if h.VocabReader.d.Fetcher != nil || h.Publication.d.Fetcher != nil || h.Authoring.reader.d.Fetcher != nil {
		t.Fatal("a read-only use case holds the upstream fetcher")
	}
	if h.Vocabularies.d.Fetcher == nil {
		t.Fatal("the write side lost the fetcher")
	}
}
