// Example "envoy": Envoy is the L7 proxy and asks alexandria-client (mode: validator) through its ext_authz HTTP filter before forwarding.
// Renders envoy.yaml, runs Envoy natively or in Docker, replays F6 through it and checks the verdict table (see examples/envoy/README.md).
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ging/alexandria/conformance/f6"
	"github.com/ging/alexandria/examples/internal/lab"
)

// envoyTemplate is the Envoy configuration with ${NAME} placeholders.
//
//go:embed envoy.yaml
var envoyTemplate string

// main runs the F6 scenario with Envoy proxying and the client validating.
func main() {
	binDir := flag.String("bin-dir", "bin", "directory with the lab binaries; they are built when missing")
	image := flag.String("image", "envoyproxy/envoy:v1.33-latest", "Envoy image used when no envoy binary is on PATH")
	keep := flag.Bool("keep", false, "keep the lab running after the scenario")
	verbose := flag.Bool("v", false, "print the logs of every process")
	flag.Parse()
	err := lab.Run(lab.Options{
		Title:     "counterparty -> Envoy (L7 proxy) -> connector, with ext_authz -> alexandria-client (validator)",
		BinDir:    *binDir,
		Validator: true,
		Keep:      *keep,
		Verbose:   *verbose,
		Front:     func(env f6.Env) (func(), error) { return startEnvoy(env, *image, *verbose) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "example envoy:", err)
		os.Exit(1)
	}
}

// render fills the placeholders of envoy.yaml for a listen address and the host where connector and validator live.
func render(env f6.Env, listen, host string) (string, error) {
	conn, err := url.Parse(env.ConnURL)
	if err != nil {
		return "", err
	}
	_, port, err := net.SplitHostPort(strings.TrimPrefix(env.Ingress, "http://"))
	if err != nil {
		return "", err
	}
	_, check, err := net.SplitHostPort(env.Check)
	if err != nil {
		return "", err
	}
	vars := map[string]string{
		"LISTEN_ADDRESS": listen, "LISTEN_PORT": port,
		"CONNECTOR_HOST": host, "CONNECTOR_PORT": conn.Port(),
		"VALIDATOR_HOST": host, "VALIDATOR_PORT": check,
	}
	return os.Expand(envoyTemplate, func(k string) string { return vars[k] }), nil
}

// startEnvoy runs Envoy on env.Ingress, from PATH when installed and in Docker otherwise, and waits until it routes.
func startEnvoy(env f6.Env, image string, verbose bool) (func(), error) {
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(env.Ingress, "http://"))
	var cmd *exec.Cmd
	stop := func() {}
	if bin, err := exec.LookPath("envoy"); err == nil {
		cfg, err := render(env, "127.0.0.1", "127.0.0.1")
		if err != nil {
			return nil, err
		}
		path := filepath.Join(env.Work, "envoy.yaml")
		if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
			return nil, err
		}
		cmd = exec.CommandContext(context.Background(), bin, "-c", path, "--log-level", "warn") // #nosec G204 -- envoy binary found on PATH.
	} else {
		name := "alexandria-example-envoy-" + port
		args := []string{"run", "--rm", "--name", name}
		listen, host := "0.0.0.0", "host.docker.internal"
		if runtime.GOOS == "linux" {
			// On Linux the container shares the host network, so loopback services are reachable as they are.
			args, listen, host = append(args, "--network", "host"), "127.0.0.1", "127.0.0.1"
		} else {
			args = append(args, "-p", "127.0.0.1:"+port+":"+port)
		}
		cfg, err := render(env, listen, host)
		if err != nil {
			return nil, err
		}
		args = append(args, image, "envoy", "--config-yaml", cfg, "--log-level", "warn")
		cmd = exec.CommandContext(context.Background(), "docker", args...) // #nosec G204 -- fixed docker invocation with a rendered configuration.
		stop = func() {
			_ = exec.CommandContext(context.Background(), "docker", "rm", "-f", name).Run() // #nosec G204 -- container started above.
		}
	}
	if verbose {
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start envoy (install envoy or start Docker): %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	halt := func() {
		stop()
		_ = cmd.Process.Kill()
	}
	if err := waitRouting(env.Ingress, exited); err != nil {
		halt()
		return nil, err
	}
	return halt, nil
}

// waitRouting polls a harmless path until Envoy has asked the validator and reached the connector.
func waitRouting(ingress string, exited <-chan error) error {
	c := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 600; i++ {
		select {
		case err := <-exited:
			return fmt.Errorf("envoy exited before it was ready (run with -v to see why): %w", err)
		default:
		}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ingress+"/_ready", nil)
		if resp, err := c.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("envoy did not start routing on %s", ingress)
}
