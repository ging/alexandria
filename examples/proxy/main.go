// Example "proxy": alexandria-client is the L7 proxy in front of the connector and forwards the traffic itself (mode: proxy).
// Replays the F6 scenario through its dsp-ingress listener and checks the verdict table (see examples/proxy/README.md).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ging/alexandria/examples/internal/lab"
)

// main runs the F6 scenario with the client proxying the connector.
func main() {
	binDir := flag.String("bin-dir", "bin", "directory with the lab binaries; they are built when missing")
	keep := flag.Bool("keep", false, "keep the lab running after the scenario")
	verbose := flag.Bool("v", false, "print the logs of every process")
	flag.Parse()
	err := lab.Run(lab.Options{
		Title:   "counterparty -> alexandria-client (L7 proxy) -> connector",
		BinDir:  *binDir,
		Keep:    *keep,
		Verbose: *verbose,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "example proxy:", err)
		os.Exit(1)
	}
}
