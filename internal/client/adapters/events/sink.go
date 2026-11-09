// Package events provides the event sinks selected by events.sink: append-only JSON Lines file (file:) or one HTTP POST per event (http:, https:).
// Implements the EventSink port for every event type of §4.11 (REQ-CLI-021).
package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ging/alexandria/internal/client/app"
)

// FileSink appends one JSON document per line.
type FileSink struct {
	mu sync.Mutex
	f  *os.File
}

// HTTPSink posts each event as JSON.
type HTTPSink struct {
	URL  string
	HTTP *http.Client
}

// Tee forwards events to several sinks.
type Tee []app.EventSink

// Emit forwards to every sink.
func (t Tee) Emit(e any) {
	for _, s := range t {
		s.Emit(e)
	}
}

// Open builds the sink of a URI.
func Open(uri string) (app.EventSink, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "file":
		p := u.Path
		if p == "" {
			p = u.Opaque
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304 -- sink path comes from the operator configuration.
		if err != nil {
			return nil, err
		}
		return &FileSink{f: f}, nil
	case "http", "https":
		return HTTPSink{URL: uri, HTTP: &http.Client{Timeout: 5 * time.Second}}, nil
	}
	return nil, fmt.Errorf("events: unsupported sink %s", uri)
}

// Emit appends the event to the file.
func (s *FileSink) Emit(e any) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.f.Write(append(b, '\n'))
}

// Emit posts the event, logging failures.
func (s HTTPSink) Emit(e any) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.URL, bytes.NewReader(b))
	if err != nil {
		log.Printf("events: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		log.Printf("events: %v", err)
		return
	}
	_ = resp.Body.Close()
}
