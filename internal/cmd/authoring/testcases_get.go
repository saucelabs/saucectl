package authoring

import (
	"context"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestCasesGetCommand is `authoring testcases get`.
func TestCasesGetCommand() *cobra.Command {
	var out string
	var revisionID string
	var showSteps bool

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Show a test case, optionally with its recorded steps",
		Example: `  saucectl authoring testcases get 6a882c1dc8b4482c166e96c9
  saucectl authoring testcases get 6a882c1dc8b4482c166e96c9 --show-steps
  saucectl authoring testcases get 6a882c1dc8b4482c166e96c9 -o json | jq '.revisions[-1].steps'`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			return getTestCase(cmd.Context(), out, args[0], revisionID, showSteps)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&revisionID, "revision", "", "Show this revision instead of the latest.")
	flags.BoolVar(&showSteps, "show-steps", false, "Also print the revision's recorded steps.")

	return cmd
}

// getTestCase fetches and renders one test case.
func getTestCase(ctx context.Context, out, id, revisionID string, showSteps bool) error {
	tc, err := testCaseService.GetTestCase(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get test case: %w", err)
	}

	rev, ok := tc.LatestRevision()
	if revisionID != "" {
		ok = false
		for _, r := range tc.Revisions {
			if r.ID == revisionID {
				rev, ok = r, true
				break
			}
		}
		if !ok {
			return fmt.Errorf("test case %s has no revision %s", id, revisionID)
		}
	}

	if out == JSONOutput {
		return renderJSON(tc)
	}

	renderTestCaseDetail(tc, rev, ok)
	if showSteps && ok {
		renderSteps(rev.Steps)
	}
	return nil
}

// renderTestCaseDetail prints the two-column property view, following
// `devices get`.
func renderTestCaseDetail(tc authoring.TestCase, rev authoring.Revision, hasRevision bool) {
	t := newTable()
	t.AppendHeader(table.Row{"Property", "Value"})
	t.AppendRow(table.Row{"ID", tc.ID})
	t.AppendRow(table.Row{"Name", tc.Name})
	t.AppendRow(table.Row{"Tags", joinOrDash(tc.Tags)})
	t.AppendRow(table.Row{"Suite", orDash(tc.TestSuiteID)})
	t.AppendRow(table.Row{"Created", fmt.Sprintf("%s by %s", humanizeDate(tc.CreationDate), orDash(tc.CreatorUserName))})
	t.AppendRow(table.Row{"Updated", fmt.Sprintf("%s by %s", humanizeDate(tc.LastUpdateDate), orDash(tc.LastModifierUserName))})
	t.AppendRow(table.Row{"Test URL", orDash(tc.RunSettings.TestURL)})
	t.AppendRow(table.Row{"Tunnel", describeStoredTunnel(tc.RunSettings.TunnelName)})
	t.AppendRow(table.Row{"Primary Target", describeTarget(tc.RunSettings.PrimaryTarget)})
	targets := make([]string, 0, len(tc.RunSettings.RunTargets))
	for _, rt := range tc.RunSettings.RunTargets {
		targets = append(targets, describeTarget(rt))
	}
	t.AppendRow(table.Row{"Run Targets", joinOrDash(targets)})
	t.AppendRow(table.Row{"Revisions", len(tc.Revisions)})
	if hasRevision {
		t.AppendRow(table.Row{"Revision", rev.ID})
		t.AppendRow(table.Row{"Intent", rev.Intent})
		t.AppendRow(table.Row{"Discovered Intent", orDash(rev.DiscoveredIntent)})
		t.AppendRow(table.Row{"Description", orDash(rev.Description)})
		t.AppendRow(table.Row{"Steps", len(rev.Steps)})
	}
	fmt.Println(t.Render())

	// The revision-level reasoning: the agent's titled paragraphs about the
	// journey as a whole, distinct from each step's own reason (FR-015).
	if hasRevision && len(rev.Reasoning) > 0 {
		fmt.Println("Reasoning:")
		for _, r := range rev.Reasoning {
			fmt.Printf("  * %s\n", r.Title)
			if r.Description != "" {
				fmt.Printf("    %s\n", r.Description)
			}
		}
		fmt.Println()
	}
}

// describeStoredTunnel explains the stored tunnel value, calling out the
// empty string, which is not the same as none (research R-008).
func describeStoredTunnel(name *string) string {
	switch {
	case name == nil:
		return "-"
	case *name == "":
		return `"" (empty; cleared automatically on run)`
	default:
		return *name
	}
}

// renderSteps prints a revision's steps, one readable line each, with the
// extracted artifact identifier rather than the kilobyte-long signed URL.
func renderSteps(steps []authoring.Step) {
	if len(steps) == 0 {
		fmt.Println("This revision has no steps.")
		return
	}

	t := newTable()
	t.AppendHeader(table.Row{"#", "Action", "Result", "Screenshot", "Reasoning"})
	for i, s := range steps {
		t.AppendRow(table.Row{
			i + 1,
			s.Tool.Summary(),
			stepResult(s.Result),
			orDash(s.ArtifactID()),
			truncate(strings.TrimSpace(s.Tool.Reasoning()), 80),
		})
	}
	fmt.Println(t.Render())
}

// stepResult renders a step outcome as a mark plus any message.
func stepResult(r *authoring.StepResult) string {
	if r == nil {
		return "-"
	}
	mark := "✔"
	if !r.Success {
		mark = "✖"
	}
	if r.Message != "" {
		return mark + " " + truncate(r.Message, 60)
	}
	return mark
}
