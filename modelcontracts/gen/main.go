// Command gen rewrites the shipped contract payload from the governance specs.
//
//	go run ./modelcontracts/gen
//
// Run it after editing anything under docs/model-governance/specs. The drift
// test in the parent package fails until it is run, so a forgotten regeneration
// cannot reach a release.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/modelcontracts"
)

func main() {
	specDir := flag.String("specs", "docs/model-governance/specs", "directory holding the governance specs")
	out := flag.String("out", "modelcontracts/contracts.json", "payload file to write")
	flag.Parse()

	doc, err := modelcontracts.Build(*specDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}

	// encoding/json, not the common wrapper: this is a build tool writing a
	// committed, reviewable artifact, not a request path. Indentation is the
	// point — the file is meant to be read in diffs.
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, append(body, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	fmt.Printf("gen: wrote %d contracts to %s\n", len(doc.Models), *out)
}
