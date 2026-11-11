// Fake DSP connector for the e2e test and the demo: answers 200 with JSON on every DSP route, records what it receives and serves agreements.
// Not part of the product; it only stands in for an unmodified connector behind alexandria-client (Part 5.2).
package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// received is one recorded request.
type received struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   string `json:"body"`
}

// connector records requests.
type connector struct {
	mu  sync.Mutex
	log []received
}

// main serves the fake connector on the -listen address.
func main() {
	addr := flag.String("listen", "127.0.0.1:8282", "listen address")
	flag.Parse()
	c := &connector{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_received", c.list)
	mux.HandleFunc("GET /_agreement", agreement)
	mux.HandleFunc("GET /dcat-ap.jsonld", context)
	mux.HandleFunc("/", c.ack)
	log.Printf("fake-connector listening on %s", *addr)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

// ack records the request and answers with a JSON acknowledgement naming it.
func (c *connector) ack(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.log = append(c.log, received{r.Method, r.URL.Path, string(b)})
	c.mu.Unlock()
	log.Printf("connector received %q %q (%d bytes)", r.Method, r.URL.Path, len(b)) // #nosec G706 -- values are quoted, so newlines cannot forge entries.
	out, _ := json.Marshal(map[string]string{"@type": "Ack", "received": r.Method + " " + r.URL.Path})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out) // #nosec G705 -- JSON-encoded body served as application/json.
}

// list returns the recorded requests.
func (c *connector) list(w http.ResponseWriter, _ *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c.log)
}

// agreement returns a ContractAgreementMessage for ?id=&action= as the connector would emit it.
func agreement(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	msg := map[string]any{
		"@context": []string{"https://w3id.org/dspace/2025/1/context.jsonld"}, "@type": "ContractAgreementMessage",
		"providerPid": "urn:uuid:p-1", "consumerPid": "urn:uuid:c-1", "action": q.Get("action"),
		"agreement": map[string]any{"@id": q.Get("id"), "@type": "Agreement", "target": "urn:uuid:ds-1"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msg)
}

// context serves a small JSON-LD context standing in for the DCAT-AP upstream of a proxied vocabulary (F6.1).
func context(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/ld+json")
	_, _ = w.Write([]byte(`{"@context":{"dcat":"http://www.w3.org/ns/dcat#","dct":"http://purl.org/dc/terms/"}}`))
}
