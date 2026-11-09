// Tests for offline package verification S8, S11, S12 and S13.
// Each case corrupts one aspect of a valid package and expects exactly its code (REQ-CLI-006, REQ-CLI-007).
package pkgv

import (
	"encoding/json"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
)

// valid builds a coherent package for version 1.0.0 with one schema file.
func valid() (Files, ni.Digest) {
	content := []byte(`{"type":"object"}`)
	m := Manifest{Space: "https://example.com/spaces/demo", Version: "1.0.0",
		Files: []File{{Path: "schemas/policy.schema.json", MediaType: "application/schema+json", Digest: ni.Sum(content), Role: "json-schema", Slot: "odrl", AppliesTo: []string{"policy_intake"}}},
		Slots: []Slot{{Slot: "odrl", Obligation: "mandatory"}}}
	mb, _ := json.Marshal(m)
	d := ni.Sum(mb)
	sl, _ := json.Marshal(SpaceLock{Space: m.Space, Version: "1.0.0", Manifest: d, Lifecycle: lifecycle.Defaults()})
	return Files{Manifest: mb, SpaceLock: sl, Content: map[string][]byte{"schemas/policy.schema.json": content}}, d
}

// codeOf returns the code of a coded error or an empty code.
func codeOf(e *codes.Error) codes.Code {
	if e == nil {
		return ""
	}
	return e.Code
}

func TestVerifyValidPackage(t *testing.T) {
	f, d := valid()
	if _, err := Verify(f, d, "1.0.0"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyS8DigestMismatch(t *testing.T) {
	f, _ := valid()
	if _, err := Verify(f, ni.Sum([]byte("other")), "1.0.0"); codeOf(err) != codes.SyncManifestDigestMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyS11FileHashMismatch(t *testing.T) {
	f, d := valid()
	f.Content["schemas/policy.schema.json"] = []byte(`{"type":"objecT"}`)
	if _, err := Verify(f, d, "1.0.0"); codeOf(err) != codes.SyncFileHashMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyS12ExtraFile(t *testing.T) {
	f, d := valid()
	f.Content["extra.json"] = []byte(`{}`)
	if _, err := Verify(f, d, "1.0.0"); codeOf(err) != codes.SyncFileSetMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyS12MissingFile(t *testing.T) {
	f, d := valid()
	delete(f.Content, "schemas/policy.schema.json")
	if _, err := Verify(f, d, "1.0.0"); codeOf(err) != codes.SyncFileSetMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyS13VersionMismatch(t *testing.T) {
	f, d := valid()
	if _, err := Verify(f, d, "1.0.1"); codeOf(err) != codes.SyncIncoherent {
		t.Fatalf("got %v", err)
	}
}
