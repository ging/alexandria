// Fake DSP connector for the e2e test and the demo: answers 200 with a small JSON document on every route and logs requests.
// Not part of the product; it only stands in for an unmodified connector behind vhub-client (Part 5.3).
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:8282", "listen address")
	flag.Parse()
	log.Printf("fake-connector listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		log.Printf("connector received %s %s (%d bytes)", r.Method, r.URL.Path, len(b))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"@type":"Ack","received":"` + r.Method + " " + r.URL.Path + `"}`))
	})))
}
