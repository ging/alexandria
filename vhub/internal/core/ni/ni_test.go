// Unit tests for ni: digests.
// Covers the RFC 6920 form and its URL segment (§2.5).
package ni

import "testing"

func TestSumAndSegmentRoundTrip(t *testing.T) {
	d := Sum([]byte("hello"))
	back, err := FromSegment(d.Segment())
	if err != nil || back != d || len(d.Hex()) != 64 {
		t.Fatalf("round trip failed: %s %v", d, err)
	}
}

func TestParseRejectsShortDigest(t *testing.T) {
	if _, err := Parse("ni:///sha-256;q3RkZmFrZWRpZ2VzdA"); err == nil {
		t.Fatal("short digest accepted")
	}
}
