// Package f6 launches the F6 environment: hub key, lab TLS certificate, configurations on free ports and the four real binaries.
// Every process reads the same file clock, so the scenario replays two and a half months in seconds (DP-4, DP-5, DP-8, DP-14, Part 4 M8).
package f6

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ging/alexandria/internal/core/did"
)

// syncWriter serializes concurrent writes from child process stdout/stderr goroutines.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// Write writes p under the mutex.
func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// Env describes a running F6 environment.
type Env struct {
	Work, BinDir, ClockFile string
	HubURL, IDPURL, ConnURL string
	Ingress, Egress, Admin  string
	HubDID                  did.DID
	CAFile, EventsFile      string
	Log                     io.Writer
	// Validator starts the client in validator mode on Check; Ingress is then the address of the external proxy.
	Validator bool
	Check     string
}

// freePort returns a TCP port that is free right now.
func freePort() (int, error) {
	l, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// writeJSON writes v as indented JSON.
func writeJSON(path string, v any) error {
	b, _ := json.MarshalIndent(v, "", "  ")
	return os.WriteFile(path, b, 0o600)
}

// hubKey writes a fresh Ed25519 key in PKCS#8 PEM and returns its did:key.
func hubKey(path string) (did.DID, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	d, _ := did.KeyDID(pub)
	return d, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600)
}

// labTLS writes a self-signed certificate for 127.0.0.1 and its key (DP-14).
func labTLS(certPath, keyPath string) error {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "alexandria-hub-lab"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		return err
	}
	kd, _ := x509.MarshalPKCS8PrivateKey(k)
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		return err
	}
	return os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kd}), 0o600)
}

// SetClock writes an instant to the shared file clock.
func (e Env) SetClock(ts string) error { return os.WriteFile(e.ClockFile, []byte(ts+"\n"), 0o600) }

// Launch starts fake-idp, fake-connector and alexandria-server from binDir; the client is started later by StartClient.
func Launch(binDir, work string, log io.Writer) (Env, func(), error) {
	var ports [7]int
	for i := range ports {
		p, err := freePort()
		if err != nil {
			return Env{}, nil, err
		}
		ports[i] = p
	}
	e := Env{
		Work: work, BinDir: binDir, ClockFile: filepath.Join(work, "clock"),
		HubURL: fmt.Sprintf("https://127.0.0.1:%d", ports[0]), IDPURL: fmt.Sprintf("http://127.0.0.1:%d", ports[1]),
		ConnURL: fmt.Sprintf("http://127.0.0.1:%d", ports[2]), Ingress: fmt.Sprintf("http://127.0.0.1:%d", ports[3]),
		Egress: fmt.Sprintf("127.0.0.1:%d", ports[4]), Admin: fmt.Sprintf("http://127.0.0.1:%d", ports[5]),
		Check:  fmt.Sprintf("127.0.0.1:%d", ports[6]),
		CAFile: filepath.Join(work, "hub.crt"), EventsFile: filepath.Join(work, "client", "events.jsonl"),
		Log: &syncWriter{w: log},
	}
	if err := e.SetClock("2026-09-01T09:00:00Z"); err != nil {
		return e, nil, err
	}
	var err error
	if e.HubDID, err = hubKey(filepath.Join(work, "hub.key")); err != nil {
		return e, nil, err
	}
	if err := labTLS(e.CAFile, filepath.Join(work, "hub-tls.key")); err != nil {
		return e, nil, err
	}
	server := map[string]any{
		"listen": strings.TrimPrefix(e.HubURL, "https://"), "base": e.HubURL, "signing": map[string]string{"key": "file:hub.key"},
		"rdf":   map[string]string{"context": "self-hosted"},
		"auth":  map[string]any{"issuers": []map[string]string{{"iss": e.IDPURL, "discovery_url": e.IDPURL + "/.well-known/openid-configuration"}}, "audience": "alexandria-vhub"},
		"vocab": map[string]any{"allow_insecure_localhost": true}, "storage": map[string]string{"database": "file:hub/hub.db", "blobs": "file:hub/blobs"},
		"tls": map[string]string{"cert": "file:hub.crt", "key": "file:hub-tls.key"},
	}
	if err := writeJSON(filepath.Join(work, "server.json"), server); err != nil {
		return e, nil, err
	}
	var procs []*exec.Cmd
	stop := func() {
		for _, p := range procs {
			_ = p.Process.Kill()
			_ = p.Wait()
		}
	}
	start := func(name string, args ...string) error {
		cmd := exec.CommandContext(context.Background(), filepath.Join(binDir, name), args...) // #nosec G204 -- test binaries built by the suite.
		cmd.Dir, cmd.Stdout, cmd.Stderr = work, e.Log, e.Log
		if err := cmd.Start(); err != nil {
			return err
		}
		procs = append(procs, cmd)
		return nil
	}
	steps := []func() error{
		func() error {
			return start("fake-idp", "serve", "--addr", strings.TrimPrefix(e.IDPURL, "http://"), "--issuer", e.IDPURL, "--key", "idp.key")
		},
		func() error { return start("fake-connector", "-listen", strings.TrimPrefix(e.ConnURL, "http://")) },
		func() error { return start("alexandria-server", "-config", "server.json", "-clock-file", e.ClockFile) },
	}
	for _, s := range steps {
		if err := s(); err != nil {
			stop()
			return e, nil, err
		}
	}
	for _, u := range []string{e.IDPURL + "/.well-known/openid-configuration", e.ConnURL + "/_received", e.HubURL + "/v1/namespaces"} {
		if err := e.waitUp(u); err != nil {
			stop()
			return e, nil, err
		}
	}
	return e, stop, nil
}

// StartClient writes the client configuration and starts alexandria-client; it returns a stop function.
func (e Env) StartClient(log io.Writer) (func(), error) {
	cfg := map[string]any{
		"trust":     map[string]string{"hub": e.HubURL, "namespace": "demo", "hub_did": string(e.HubDID), "context": e.HubURL + "/pub/_vhub/v1/context.jsonld"},
		"upstreams": map[string]string{"dsp": e.ConnURL + "/dsp"},
		"listeners": map[string]any{
			"dsp-ingress": map[string]string{"bind": strings.TrimPrefix(e.Ingress, "http://")}, "dsp-egress": map[string]string{"bind": e.Egress},
			"admin": map[string]string{"bind": strings.TrimPrefix(e.Admin, "http://")},
		},
		"rules":  map[string]string{"seed": "file:" + filepath.Join(repoRoot(), "deploy", "rules.seed.json")},
		"events": map[string]string{"sink": "file:" + e.EventsFile},
	}
	if e.Validator {
		cfg["mode"] = "validator"
		delete(cfg, "upstreams")
		cfg["listeners"].(map[string]any)["dsp-ingress"] = map[string]string{"bind": e.Check}
	}
	if err := os.MkdirAll(filepath.Join(e.Work, "client"), 0o750); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(e.Work, "client.json"), cfg); err != nil {
		return nil, err
	}
	w := e.Log
	if w == nil {
		w = &syncWriter{w: log}
	}
	cmd := exec.CommandContext(context.Background(), filepath.Join(e.BinDir, "alexandria-client"), "-config", "client.json", "-state-dir", "client", "-clock-file", e.ClockFile, "-hub-ca", e.CAFile) // #nosec G204 -- test binary.
	cmd.Dir, cmd.Stdout, cmd.Stderr = e.Work, w, w
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	stop := func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }
	if err := e.waitUp(e.Admin + "/healthz"); err != nil {
		stop()
		return nil, err
	}
	return stop, nil
}

// EnsureBinaries returns binDir when it holds the four binaries, and builds them into fallbackDir otherwise.
func EnsureBinaries(binDir, fallbackDir string) (string, error) {
	targets := map[string]string{
		"alexandria-server": "./cmd/alexandria-server",
		"alexandria-client": "./cmd/alexandria-client",
		"fake-idp":          "./deploy/fake-idp",
		"fake-connector":    "./deploy/fake-connector",
	}
	allExist := true
	for name := range targets {
		if _, err := os.Stat(filepath.Join(binDir, name)); err != nil {
			allExist = false
			break
		}
	}
	if allExist {
		return filepath.Abs(binDir)
	}
	if err := os.MkdirAll(fallbackDir, 0o750); err != nil {
		return "", err
	}
	for name, pkg := range targets {
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", filepath.Join(fallbackDir, name), pkg) // #nosec G204 -- fixed package list of this repository.
		cmd.Dir = repoRoot()
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("build %s: %w\n%s", name, err, out)
		}
	}
	return fallbackDir, nil
}

// Token mints an access token for sub at the current file-clock instant, valid for 100 days.
func (e Env) Token(sub string, admin bool) (string, error) {
	args := []string{"token", "--issuer", e.IDPURL, "--key", "idp.key", "--sub", sub, "--ttl", "2400h", "--clock-file", e.ClockFile}
	if admin {
		args = append(args, "--admin")
	}
	cmd := exec.CommandContext(context.Background(), filepath.Join(e.BinDir, "fake-idp"), args...) // #nosec G204 -- test binary.
	cmd.Dir = e.Work
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// waitUp polls a URL until it answers.
func (e Env) waitUp(u string) error {
	c := e.HTTP()
	for i := 0; i < 100; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
		if resp, err := c.Do(req); err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%s did not come up", u)
}

// repoRoot returns the repository root, found from the working directory.
func repoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// HTTP returns a client that trusts the lab hub certificate.
func (e Env) HTTP() *http.Client { return httpClient(e.CAFile) }
