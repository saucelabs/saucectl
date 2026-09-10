package authoring

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mattn/go-isatty"

	"github.com/saucelabs/saucectl/internal/tables"
)

// Output formats selected with -o/--out, matching the tool's other commands.
const (
	JSONOutput = "json"
	TextOutput = "text"
)

// validateOutput rejects an unknown format before any request is made.
func validateOutput(out string) error {
	if out != JSONOutput && out != TextOutput {
		return fmt.Errorf("unknown output format %q; options: %s, %s", out, TextOutput, JSONOutput)
	}
	return nil
}

// renderJSON writes val as one JSON document to stdout, the same way
// `builds list -o json` does.
func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}

// newTable returns a table writer in the shared saucectl style.
func newTable() table.Writer {
	t := table.NewWriter()
	t.SetStyle(tables.DefaultTableStyle)
	t.SuppressEmptyColumns()
	return t
}

// listFooter renders the "showing N of M <resource>" footer every listing
// carries, so users always know how many results exist beyond those shown.
func listFooter(shown, total int, resource string) table.Row {
	return table.Row{fmt.Sprintf("showing %d of %d %s", shown, total, resource)}
}

// humanizeDate renders a service timestamp for people: local time without
// sub-second noise. It falls back to the raw value on anything it cannot
// parse, so an unexpected format is a display concern rather than a failure.
func humanizeDate(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// isTerm reports whether fd is an interactive terminal, Cygwin included.
func isTerm(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// interactive reports whether both stdin and stdout are terminals, which is
// the precondition for prompting.
func interactive() bool {
	return isTerm(os.Stdin.Fd()) && isTerm(os.Stdout.Fd())
}

// truncate shortens s to at most n runes, marking the cut with an ellipsis.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// joinOrDash joins strings for a table cell, showing "-" for none.
func joinOrDash(parts []string) string {
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// orDash returns s, or "-" when empty, for table cells.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
