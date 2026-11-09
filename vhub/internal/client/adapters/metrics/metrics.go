// Hand-written OpenMetrics exposition: classifications per gate, version and result, listing age and request latency.
// Avoids a metrics framework; only the three families of §4.11 are exposed (REQ-CLI-020, REQ-CLI-021).
package metrics

import (
	"fmt"
	"io"
	"sort"
	"sync"
)

// Registry accumulates metric values.
type Registry struct {
	mu           sync.Mutex
	counts       map[[3]string]int64
	latencySum   float64
	latencyCount int64
}

// New returns an empty registry.
func New() *Registry { return &Registry{counts: map[[3]string]int64{}} }

// Classified counts one gate classification.
func (r *Registry) Classified(gate, version, result string) {
	r.mu.Lock()
	r.counts[[3]string{gate, version, result}]++
	r.mu.Unlock()
}

// ObserveLatency records one request latency.
func (r *Registry) ObserveLatency(seconds float64) {
	r.mu.Lock()
	r.latencySum += seconds
	r.latencyCount++
	r.mu.Unlock()
}

// ContentType is the OpenMetrics text media type.
const ContentType = "application/openmetrics-text; version=1.0.0; charset=utf-8"

// Write renders the exposition with the given listing age in seconds (negative if none).
func (r *Registry) Write(w io.Writer, listingAge int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintln(w, "# TYPE vhub_classifications counter")
	fmt.Fprintln(w, "# HELP vhub_classifications Gate classifications by gate, version and result.")
	keys := make([][3]string, 0, len(r.counts))
	for k := range r.counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	for _, k := range keys {
		fmt.Fprintf(w, "vhub_classifications_total{gate=%q,version=%q,result=%q} %d\n", k[0], k[1], k[2], r.counts[k])
	}
	fmt.Fprintln(w, "# TYPE vhub_listing_age_seconds gauge")
	if listingAge >= 0 {
		fmt.Fprintf(w, "vhub_listing_age_seconds %d\n", listingAge)
	}
	fmt.Fprintln(w, "# TYPE vhub_request_latency_seconds summary")
	fmt.Fprintf(w, "vhub_request_latency_seconds_sum %g\n", r.latencySum)
	fmt.Fprintf(w, "vhub_request_latency_seconds_count %d\n", r.latencyCount)
	fmt.Fprintln(w, "# EOF")
}
