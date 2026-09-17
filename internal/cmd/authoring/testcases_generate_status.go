package authoring

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestCasesGenerateStatusCommand is `authoring testcases generate-status`: it
// shows, or reattaches to, an authoring task started earlier.
func TestCasesGenerateStatusCommand() *cobra.Command {
	var out string
	var wait bool
	var pollInterval, waitTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "generate-status <task-id>",
		Short: "Show the status of an authoring task, or reattach and wait for it",
		Example: `  saucectl authoring testcases generate-status 8b1f… 
  saucectl authoring testcases generate-status 8b1f… --wait`,
		SilenceUsage: true,
		Args:         requireArgs("task-id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			if wait {
				if waitTimeout <= 0 {
					waitTimeout = defaultWaitTimeout
				}
				return waitForGeneration(cmd.Context(), args[0], pollInterval, waitTimeout, out, os.Stdout)
			}

			state, err := testCaseService.GenerationStatus(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get generation status: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(state)
			}
			renderGenerationState(args[0], state)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.BoolVar(&wait, "wait", false, "Wait for the task to finish, streaming each action as it happens.")
	flags.DurationVar(&pollInterval, "poll-interval", defaultGenerationPoll, "How often to poll while waiting.")
	flags.DurationVar(&waitTimeout, "wait-timeout", 0, "How long to wait before giving up locally. 0 means 1h2m.")

	return cmd
}

// renderGenerationState prints a one-shot snapshot of a task.
func renderGenerationState(taskID string, state authoring.GenerationState) {
	fmt.Printf("Task %s: %s\n", taskID, state.Status)
	for _, r := range state.Reasoning {
		fmt.Printf("  * %s\n", r.Title)
	}
	for _, s := range state.Steps {
		fmt.Printf("  %s\n", formatGenerationStep(s))
	}
	switch state.Status {
	case authoring.GenerationCompleted:
		fmt.Printf("\nNew test case: %s\nInspect it with: saucectl authoring testcases get %s --show-steps\n", state.TestCaseID, state.TestCaseID)
	case authoring.GenerationFailed:
		if state.Error != nil {
			fmt.Printf("\n%s\n", state.Error.Error())
		}
	default:
		fmt.Printf("\nStill running. Reattach with: saucectl authoring testcases generate-status %s --wait\n", taskID)
	}
}
