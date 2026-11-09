// Package dsl implements the N5 check of context.jsonld with json-gold: the context processes offline and every alias expands to its term IRI.
// This is what a policy_intake gate relies on when it expands a message with the profile context (§11.5, §13.1).
package dsl

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ging/alexandria/internal/core/jsonld"
)

// checkContext expands a probe typed with every alias of the profile and compares the result with the index.
func checkContext(ix *Index, ctx []byte) error {
	l := jsonld.NewLoader()
	if err := l.Add(ContextURL(ix), ctx); err != nil {
		return err
	}
	want := map[string]string{}
	var aliases []string
	for _, e := range ix.Entries {
		if dspDefined[e.Alias] && strings.HasPrefix(e.IRI, Reserved["odrl"]) {
			continue
		}
		want[e.Alias] = e.IRI
		aliases = append(aliases, e.Alias)
	}
	sort.Strings(aliases)
	node := map[string]any{"@context": ContextURL(ix), "@id": "urn:probe"}
	if len(aliases) > 0 {
		node["@type"] = aliases
	}
	probe, _ := json.Marshal(node)
	out, err := jsonld.Expand(probe, l)
	if err != nil {
		return fmt.Errorf("context does not process: %w", err)
	}
	if len(aliases) == 0 {
		return nil
	}
	if len(out) != 1 {
		return fmt.Errorf("probe expanded to %d nodes", len(out))
	}
	got, _ := out[0].(map[string]any)["@type"].([]any)
	if len(got) != len(aliases) {
		return fmt.Errorf("%d of %d aliases expanded", len(got), len(aliases))
	}
	for i, a := range aliases {
		if got[i] != want[a] {
			return fmt.Errorf("alias %q expands to %v, not %s", a, got[i], want[a])
		}
	}
	return nil
}
