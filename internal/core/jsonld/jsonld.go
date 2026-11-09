// Offline JSON-LD 1.1 expansion with json-gold: the loader only serves the documents it was given, never the network.
// Shared by the compiler (N5 context check) and the client evaluator, so both expand alike (D-PVP-1, REQ-CLI-010, REQ-SYS-001).
package jsonld

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/piprate/json-gold/ld"
)

// NotInPackage reports a context URL the loader was not given.
type NotInPackage struct{ URL string }

// Error implements error.
func (e *NotInPackage) Error() string { return "context " + e.URL + " is not in the package" }

// Loader is a json-gold DocumentLoader restricted to a fixed set of documents.
type Loader struct{ docs map[string]any }

var _ ld.DocumentLoader = (*Loader)(nil)

// NewLoader returns an empty loader.
func NewLoader() *Loader { return &Loader{docs: map[string]any{}} }

// Add makes the JSON document b available at url.
func (l *Loader) Add(url string, b []byte) error {
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("jsonld: %s: %w", url, err)
	}
	l.docs[url] = doc
	return nil
}

// LoadDocument serves a known document or fails with NotInPackage.
func (l *Loader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	doc, ok := l.docs[u]
	if !ok {
		return nil, &NotInPackage{URL: u}
	}
	return &ld.RemoteDocument{DocumentURL: u, Document: doc}, nil
}

// Expand expands a JSON-LD document with the loader in JSON-LD 1.1 mode.
func Expand(doc []byte, l *Loader) ([]any, error) {
	var in any
	if err := json.Unmarshal(doc, &in); err != nil {
		return nil, fmt.Errorf("jsonld: %w", err)
	}
	opts := ld.NewJsonLdOptions("")
	opts.DocumentLoader = l
	opts.ProcessingMode = ld.JsonLd_1_1
	return ld.NewJsonLdProcessor().Expand(in, opts)
}

// MissingContext returns the URL of a context the loader refused, if that is why expansion failed.
func MissingContext(err error) (string, bool) {
	var n *NotInPackage
	if errors.As(err, &n) {
		return n.URL, true
	}
	return "", false
}
