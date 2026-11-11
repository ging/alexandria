// Package timex provides instants, ISO 8601 durations and the Clock port, all with second precision in UTC.
// Shared by alexandria-server and alexandria-client so both compute identical dates (§2.5, REQ-SYS-001, DP-8).
package timex

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"
)

// Instant is a UTC instant expressed in whole seconds since the Unix epoch.
type Instant int64

// Never is the instant at infinity used for ends(W) = ∞ (§3.5).
const Never Instant = math.MaxInt64

// Clock is the only source of the current instant; system_clock and file_clock implement it.
type Clock interface{ Now() Instant }

const layout = "2006-01-02T15:04:05Z"

var instantRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)

// FromTime converts a time.Time into an Instant, truncating to the second.
func FromTime(t time.Time) Instant { return Instant(t.Unix()) }

// ParseInstant parses an RFC 3339 instant with the Z suffix and second precision.
func ParseInstant(s string) (Instant, error) {
	if !instantRe.MatchString(s) {
		return 0, fmt.Errorf("timex: %q is not an instant YYYY-MM-DDTHH:MM:SSZ", s)
	}
	t, err := time.Parse(layout, s)
	if err != nil {
		return 0, fmt.Errorf("timex: %w", err)
	}
	return FromTime(t), nil
}

// MustInstant parses an instant and panics on error; intended for fixtures.
func MustInstant(s string) Instant {
	i, err := ParseInstant(s)
	if err != nil {
		panic(err)
	}
	return i
}

// Time returns the instant as a UTC time.Time.
func (i Instant) Time() time.Time { return time.Unix(int64(i), 0).UTC() }

// String renders the instant in RFC 3339 with the Z suffix.
func (i Instant) String() string {
	if i == Never {
		return "infinity"
	}
	return i.Time().Format(layout)
}

// Add returns i + d, saturating at Never.
func (i Instant) Add(d Duration) Instant {
	if i == Never {
		return Never
	}
	if d > 0 && int64(i) > math.MaxInt64-int64(d) {
		return Never
	}
	return i + Instant(d)
}

// Sub returns the duration i − j.
func (i Instant) Sub(j Instant) Duration { return Duration(i - j) }

// Before reports whether i < j.
func (i Instant) Before(j Instant) bool { return i < j }

// MarshalJSON encodes the instant as an RFC 3339 string.
func (i Instant) MarshalJSON() ([]byte, error) { return json.Marshal(i.String()) }

// UnmarshalJSON decodes an RFC 3339 string with second precision.
func (i *Instant) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseInstant(s)
	if err != nil {
		return err
	}
	*i = v
	return nil
}

// Duration is an ISO 8601 duration restricted to days, hours, minutes and seconds, in seconds.
type Duration int64

// Common durations used across the system.
const (
	Second Duration = 1
	Minute Duration = 60
	Hour   Duration = 3600
	Day    Duration = 86400
)

var durationRe = regexp.MustCompile(`^P(?:([0-9]+)D)?(?:T(?:([0-9]+)H)?(?:([0-9]+)M)?(?:([0-9]+)S)?)?$`)

// ParseDuration parses an ISO 8601 duration using only the D, H, M and S designators.
func ParseDuration(s string) (Duration, error) {
	m := durationRe.FindStringSubmatch(s)
	if m == nil || s == "P" || s == "PT" || (len(s) > 1 && s[len(s)-1] == 'T') {
		return 0, fmt.Errorf("timex: %q is not a duration with D, H, M or S", s)
	}
	var total Duration
	for idx, unit := range []Duration{Day, Hour, Minute, Second} {
		if m[idx+1] == "" {
			continue
		}
		n, err := strconv.ParseInt(m[idx+1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("timex: %w", err)
		}
		total += Duration(n) * unit
	}
	return total, nil
}

// MustDuration parses a duration and panics on error; intended for fixtures and defaults.
func MustDuration(s string) Duration {
	d, err := ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

// String renders the duration in canonical ISO 8601 form (PT0S for zero).
func (d Duration) String() string {
	if d == 0 {
		return "PT0S"
	}
	neg := ""
	if d < 0 {
		neg, d = "-", -d
	}
	out := neg + "P"
	if days := d / Day; days > 0 {
		out += strconv.FormatInt(int64(days), 10) + "D"
	}
	rest := d % Day
	if rest == 0 {
		return out
	}
	out += "T"
	if h := rest / Hour; h > 0 {
		out += strconv.FormatInt(int64(h), 10) + "H"
	}
	if m := rest % Hour / Minute; m > 0 {
		out += strconv.FormatInt(int64(m), 10) + "M"
	}
	if s := rest % Minute; s > 0 {
		out += strconv.FormatInt(int64(s), 10) + "S"
	}
	return out
}

// MarshalJSON encodes the duration as an ISO 8601 string.
func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON decodes an ISO 8601 duration string.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseDuration(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}
