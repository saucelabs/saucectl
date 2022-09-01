package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

// FullName returns the full command name by concatenating the command names of any parents,
// except the name of the CLI itself.
func FullName(cmd *cobra.Command) string {
	name := ""

	for cmd.Name() != "saucectl" {
		// Prepending, because we are looking up names from the bottom up: cypress < run < saucectl
		// which ends up correctly as 'run cypress' (sans saucectl).
		name = fmt.Sprintf("%s %s", cmd.Name(), name)
		cmd = cmd.Parent()
	}

	return strings.TrimSpace(name)
}
