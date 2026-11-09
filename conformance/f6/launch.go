// Launcher of the F6 environment: writes server and client configurations on free ports and starts the three real binaries.
// Both processes read the same file clock, so the scenario replays two months in seconds (DP-9, DP-5, Part 4 M8).
package f6

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// freePort returns a TCP port that is free right now.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// writeJSON writes v as indented JSON.
func writeJSON(path string, v any) error {
	b, _ := json.MarshalIndent(v, "", "  ")
	return os.WriteFile(path, b, 0o644)
}

// Routes are the client routes of F6: contract requests and agreements in enforce, transfers with admission.
func Routes() []map[string]any {
	return []map[string]any{
		{"listener": "dsp-ingress", "match": map[string]string{"method": "POST", "path": "/negotiations/request"}, "message": "ContractRequestMessage",
			"gates": []string{"dsp_message", "policy_intake"}, "mode": "enforce", "body": map[string]string{"select": "$.offer"}},
		{"listener": "dsp-egress", "match": map[string]string{"method": "POST", "path": "/negotiations/:consumerPid/agreement"}, "message": "ContractAgreementMessage",
			"gates": []string{"dsp_message", "policy_intake"}, "mode": "enforce", "body": map[string]string{"select": "$.agreement"}},
		{"listener": "dsp-ingress", "match": map[string]string{"method": "POST", "path": "/transfers/request"}, "message": "TransferRequestMessage",
			"gates": []string{"dsp_message", "agreement_admission"}, "mode": "enforce"},
	}
}

// Launch starts fake-connector, alexandria-server and alexandria-client from binDir with state in work; stop kills them.
func Launch(binDir, work string, log io.Writer) (Env, func(), error) {
	var ports [5]int
	for i := range ports {
		p, err := freePort()
		if err != nil {
			return Env{}, nil, err
		}
		ports[i] = p
	}
	hub, conn, ingress, egress, admin := ports[0], ports[1], ports[2], ports[3], ports[4]
	e := Env{
		HubURL: fmt.Sprintf("http://127.0.0.1:%d", hub), Connector: fmt.Sprintf("http://127.0.0.1:%d", conn),
		ClientDSP: fmt.Sprintf("http://127.0.0.1:%d", ingress), ClientEgres: fmt.Sprintf("http://127.0.0.1:%d", egress),
		ClientAdmin: fmt.Sprintf("http://127.0.0.1:%d", admin), ClockFile: filepath.Join(work, "clock"),
		EventsFile: filepath.Join(work, "client", "events.jsonl"), Log: log,
	}
	_ = os.MkdirAll(filepath.Join(work, "client"), 0o755)
	if err := e.setClock("2026-09-01T10:00:00Z"); err != nil {
		return Env{}, nil, err
	}
	did := fmt.Sprintf("did:web:127.0.0.1%%3A%d", hub)
	tokens := map[string]any{
		"editor-token":   map[string]any{"principal": "steward", "roles": []string{"editor"}},
		"releaser-token": map[string]any{"principal": "releaser", "roles": []string{"releaser"}},
		"member-token":   map[string]any{"principal": "alexandria-client", "roles": []string{"member"}},
	}
	_ = writeJSON(filepath.Join(work, "tokens.json"), tokens)
	_ = writeJSON(filepath.Join(work, "server.json"), map[string]any{
		"listen": fmt.Sprintf("127.0.0.1:%d", hub), "public_base_url": e.HubURL, "did": did,
		"sqlite_path": "hub/hub.db", "blob_dir": "hub/blobs", "signing_key_path": "hub/hub.key", "tokens_path": "tokens.json", "compiler": "dsl",
	})
	_ = os.WriteFile(filepath.Join(work, "credentials.json"), []byte(`{"token":"member-token"}`), 0o600)
	_ = writeJSON(filepath.Join(work, "config.json"), map[string]any{
		"space":     "https://example.com/spaces/demo",
		"trust":     map[string]string{"hub_did": did, "hub": fmt.Sprintf("https://127.0.0.1:%d", hub), "credentials": "file://" + filepath.Join(work, "credentials.json")},
		"upstreams": map[string]string{"dsp": e.Connector},
		"listeners": map[string]any{"dsp-ingress": map[string]string{"bind": fmt.Sprintf("127.0.0.1:%d", ingress)},
			"dsp-egress": map[string]string{"bind": fmt.Sprintf("127.0.0.1:%d", egress)}, "admin": map[string]string{"bind": fmt.Sprintf("127.0.0.1:%d", admin)}},
		"routes": Routes(), "events": map[string]string{"sink": "file://" + e.EventsFile},
	})
	var procs []*exec.Cmd
	stop := func() {
		for _, p := range procs {
			_ = p.Process.Kill()
			_ = p.Wait()
		}
	}
	start := func(name string, args ...string) error {
		cmd := exec.Command(filepath.Join(binDir, name), args...)
		cmd.Dir = work
		out, _ := os.Create(filepath.Join(work, name+".log"))
		cmd.Stdout, cmd.Stderr = out, out
		if err := cmd.Start(); err != nil {
			return err
		}
		procs = append(procs, cmd)
		return nil
	}
	steps := []struct {
		name string
		args []string
		ping string
	}{
		{"fake-connector", []string{"-listen", fmt.Sprintf("127.0.0.1:%d", conn)}, e.Connector + "/"},
		{"alexandria-server", []string{"-config", "server.json", "-clock-file", e.ClockFile}, e.HubURL + "/.well-known/did.json"},
		{"alexandria-client", []string{"-config", "config.json", "-state-dir", "client/state", "-clock-file", e.ClockFile, "-did.allow_insecure_localhost"}, e.ClientAdmin + "/healthz"},
	}
	for _, s := range steps {
		if err := start(s.name, s.args...); err != nil {
			stop()
			return Env{}, nil, err
		}
		if err := waitUp(s.ping); err != nil {
			stop()
			b, _ := os.ReadFile(filepath.Join(work, s.name+".log"))
			return Env{}, nil, fmt.Errorf("%s: %v\n%s", s.name, err, strings.TrimSpace(string(b)))
		}
	}
	return e, stop, nil
}

// waitUp polls a URL until it answers.
func waitUp(u string) error {
	for i := 0; i < 100; i++ {
		if resp, err := http.Get(u); err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%s did not answer", u)
}
