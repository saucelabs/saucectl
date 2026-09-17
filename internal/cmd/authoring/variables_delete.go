package authoring

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// VariablesDeleteCommand is `authoring variables delete`.
func VariablesDeleteCommand() *cobra.Command {
	var yes bool
	var expectedLastUpdate string

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Delete a variable",
		Example: `  saucectl authoring variables delete 5e7a…
  saucectl authoring variables delete 5e7a… --yes --expected-last-update 2026-09-05T14:59:59.000Z`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteVariable(cmd.Context(), args[0], yes, expectedLastUpdate)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt. Required when not running interactively.")
	flags.StringVar(&expectedLastUpdate, "expected-last-update", "", "The lastUpdate value from an earlier read; the delete fails if the variable changed since. Default: the current value.")

	return cmd
}

// deleteVariable confirms, naming scope and secrecy, then deletes with
// concurrency control. The token goes in the query string on this endpoint.
func deleteVariable(ctx context.Context, id string, yes bool, expectedLastUpdate string) error {
	v, err := variableService.GetVariable(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get variable: %w", err)
	}

	affects := []string{fmt.Sprintf("it is a %s variable at %s scope", secretWord(v.IsSecret), v.Scope)}
	if v.TestSuiteID != "" {
		affects = append(affects, fmt.Sprintf("tests in suite %s that reference {{%s:%s}} will lose it", v.TestSuiteID, v.Scope, v.Name))
	} else if v.TestCaseID != "" {
		affects = append(affects, fmt.Sprintf("test case %s references it as {{%s:%s}}", v.TestCaseID, v.Scope, v.Name))
	} else {
		affects = append(affects, fmt.Sprintf("every test referencing {{%s:%s}} will lose it", v.Scope, v.Name))
	}

	if err := confirmDestructive(yes, fmt.Sprintf("variable %q (%s)", v.Name, v.ID), affects); err != nil {
		return err
	}

	token := expectedLastUpdate
	if token == "" {
		token = v.LastUpdate
	}
	if err := variableService.DeleteVariable(ctx, v.ID, token); err != nil {
		if errors.Is(err, authoring.ErrVariableVersionConflict) {
			return fmt.Errorf("variable %s was changed by someone else since it was read; re-read it with 'saucectl authoring variables get %s' and try again", id, id)
		}
		return fmt.Errorf("failed to delete variable: %w", err)
	}
	fmt.Printf("Deleted variable %q (%s).\n", v.Name, v.ID)
	return nil
}
