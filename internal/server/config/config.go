// Loader of the PoC server.json, validated against its own JSON Schema; relative paths resolve from the file's folder.
// The server has no normative configuration (Q16), so this format is a PoC decision (DP-12, REQ-HUB-008, REQ-HUB-010).
package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/schemas"
)

//go:embed server.schema.json
var schemaJSON []byte

// Schema returns the embedded server configuration schema.
func Schema() []byte { return schemaJSON }

// Server is server.json.
type Server struct {
	Listen         string           `json:"listen"`
	PublicBaseURL  string           `json:"public_base_url"`
	DID            string           `json:"did"`
	SQLitePath     string           `json:"sqlite_path"`
	BlobDir        string           `json:"blob_dir"`
	SigningKeyPath string           `json:"signing_key_path"`
	TokensPath     string           `json:"tokens_path"`
	ClockFile      string           `json:"clock_file,omitempty"`
	Compiler       string           `json:"compiler,omitempty"`
	VendorDir      string           `json:"vendor_dir,omitempty"`
	Lifecycle      json.RawMessage  `json:"lifecycle,omitempty"`
	Defaults       lifecycle.Params `json:"-"`
}

// Load reads and validates a server.json.
func Load(path string) (Server, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Server{}, err
	}
	s, err := schemas.Compile(schemaJSON)
	if err != nil {
		return Server{}, err
	}
	if err := s.Validate(b); err != nil {
		return Server{}, fmt.Errorf("server.json: %w", err)
	}
	var c Server
	if err := json.Unmarshal(b, &c); err != nil {
		return Server{}, err
	}
	if c.Defaults, err = lifecycle.ParseOver(lifecycle.Defaults(), c.Lifecycle); err != nil {
		return Server{}, err
	}
	if errs := c.Defaults.Validate(); len(errs) > 0 {
		return Server{}, fmt.Errorf("server.json lifecycle: %v", errs[0])
	}
	dir := filepath.Dir(path)
	for _, p := range []*string{&c.SQLitePath, &c.BlobDir, &c.SigningKeyPath, &c.TokensPath, &c.ClockFile, &c.VendorDir} {
		if *p != "" && !filepath.IsAbs(*p) {
			*p = filepath.Join(dir, *p)
		}
	}
	return c, nil
}
