package author

import (
	"encoding/json"
	"fmt"
	"os"
)

// printStart announces that a spec is about to be processed, in text mode
// only. Authoring a spec involves a generate call followed by up to
// --poll-timeout of polling, so without this the CLI can sit silent for
// minutes at a time on a single spec (or the first of many) with no visible
// progress. JSON mode stays silent here so stdout remains a single valid
// JSON document.
func printStart(format, specPath string) {
	if format != "json" {
		fmt.Printf("%-10s %s...\n", "authoring", specPath)
	}
}

// printResult prints a single spec's outcome immediately, in text mode only.
// JSON mode defers to renderResults so the full result set stays one valid
// JSON array.
func printResult(format string, r specResult) {
	if format == "json" {
		return
	}
	printResultLine(r)
}

func printResultLine(r specResult) {
	if r.Error != "" {
		fmt.Printf("%-10s %s: %s\n", r.Action, r.SpecPath, r.Error)
		return
	}
	if r.TestCaseID != "" {
		fmt.Printf("%-10s %s -> %s\n", r.Action, r.SpecPath, r.TestCaseID)
	} else {
		fmt.Printf("%-10s %s\n", r.Action, r.SpecPath)
	}
}

// renderResults prints the outcome of an author/sync run in the requested
// format. In text mode, results have already been streamed as they
// completed (see printStart/printResult), so this only has final output
// left to produce for json mode.
func renderResults(format string, results []specResult) error {
	if format != "json" {
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}
