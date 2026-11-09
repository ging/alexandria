// System and file clock adapters of the timex.Clock port; the only place allowed to call time.Now (DP-9).
// file_clock reads an RFC 3339 instant from a file so tests and the demo can replay F6 without waiting (REQ-SYS-001).
package clock

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ging/alexandria/internal/core/timex"
)

// System reads the operating system clock.
type System struct{}

// Now returns the current instant.
func (System) Now() timex.Instant { return timex.FromTime(time.Now()) }

// File reads the instant from a file on each call and keeps the last valid value (PoC and tests only).
type File struct {
	path string
	mu   sync.Mutex
	last timex.Instant
}

// NewFile opens a file clock; the file must contain a valid instant at start.
func NewFile(path string) (*File, error) {
	f := &File{path: path}
	v, err := f.read()
	if err != nil {
		return nil, err
	}
	f.last = v
	return f, nil
}

// read parses the instant stored in the file.
func (f *File) read() (timex.Instant, error) {
	b, err := os.ReadFile(f.path)
	if err != nil {
		return 0, err
	}
	return timex.ParseInstant(strings.TrimSpace(string(b)))
}

// Now returns the instant in the file, or the last valid one if the file is momentarily unreadable.
func (f *File) Now() timex.Instant {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, err := f.read(); err == nil {
		f.last = v
	}
	return f.last
}

// Set writes an instant to a clock file (used by tests and the demo driver).
func Set(path string, t timex.Instant) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(t.String()+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Stopwatch measures wall-clock latency for metrics only; it never feeds a decision.
type Stopwatch struct{ start time.Time }

// Start begins a measurement.
func Start() Stopwatch { return Stopwatch{time.Now()} }

// Seconds returns the elapsed wall-clock time in seconds.
func (s Stopwatch) Seconds() float64 { return time.Since(s.start).Seconds() }
