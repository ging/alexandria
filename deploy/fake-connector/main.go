// Fake DSP connector for the e2e test and the demo: answers 200 with a small JSON document on every route and logs requests.
// Not part of the product; it only stands in for an unmodified connector behind vhub-client (Part 5.3).
package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"time"
)

// main serves the fake connector on the -listen address.
func main() {
	addr := flag.String("listen", "127.0.0.1:8282", "listen address")
	flag.Parse()
	log.Printf("fake-connector listening on %s", *addr)
	srv := &http.Server{Addr: *addr, Handler: http.HandlerFunc(ack), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

// ack logs the request and answers with a JSON acknowledgement naming it.
func ack(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	log.Printf("connector received %q %q (%d bytes)", r.Method, r.URL.Path, len(b)) // #nosec G706 -- values are quoted, so newlines cannot forge entries.
	out, _ := json.Marshal(map[string]string{"@type": "Ack", "received": r.Method + " " + r.URL.Path})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out) // #nosec G705 -- JSON-encoded body served as application/json.
}
