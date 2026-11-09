// Unit tests for instants and ISO 8601 durations.
// Covers parsing, canonical rendering and saturation at infinity (§2.5).
package timex

import "testing"

// mustI parses an instant or fails the test.
func mustI(t *testing.T, s string) Instant {
	t.Helper()
	i, err := ParseInstant(s)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func TestParseInstantRoundTrip(t *testing.T) {
	if got := mustI(t, "2026-11-02T10:00:00Z"); int64(got) != 1793613600 || got.String() != "2026-11-02T10:00:00Z" {
		t.Fatalf("got %d %s", got, got)
	}
}

func TestParseInstantRejectsFractionsAndOffsets(t *testing.T) {
	for _, s := range []string{"2026-11-02T10:00:00.1Z", "2026-11-02T10:00:00+01:00", "2026-11-02"} {
		if _, err := ParseInstant(s); err == nil {
			t.Errorf("%s accepted", s)
		}
	}
}

func TestParseDurationUnits(t *testing.T) {
	cases := map[string]Duration{"P30D": 30 * Day, "PT6H": 6 * Hour, "PT1M": Minute, "PT5S": 5, "P0D": 0, "PT0S": 0, "P1DT2H3M4S": Day + 2*Hour + 3*Minute + 4}
	for s, want := range cases {
		if got, err := ParseDuration(s); err != nil || got != want {
			t.Errorf("%s: got %v %v", s, got, err)
		}
	}
}

func TestParseDurationRejectsInvalid(t *testing.T) {
	for _, s := range []string{"P", "PT", "P1M", "P1Y", "1D", "P1DT"} {
		if _, err := ParseDuration(s); err == nil {
			t.Errorf("%s accepted", s)
		}
	}
}

func TestDurationString(t *testing.T) {
	cases := map[Duration]string{0: "PT0S", 30 * Day: "P30D", 6 * Hour: "PT6H", Day + 10: "P1DT10S"}
	for d, want := range cases {
		if d.String() != want {
			t.Errorf("%d: got %s", d, d.String())
		}
	}
}

func TestAddSaturatesAtNever(t *testing.T) {
	if Never.Add(Day) != Never || Instant(Never-1).Add(Day) != Never {
		t.Fatal("no saturation")
	}
}
