package authoring

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// secretPlaceholder is what a confidential value renders as, whatever the
// service returned (FR-022).
const secretPlaceholder = "<secret>"

// VariablesListCommand is `authoring variables list`.
func VariablesListCommand() *cobra.Command {
	var out, scope string
	var page pageFlags
	var opts authoring.ListVariablesOptions

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List variables",
		Example: `  saucectl authoring variables list --scope org
  saucectl authoring variables list --scope testSuite --test-suite-id 3f2a…
  saucectl authoring variables list --search pass -o json`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			if err := page.validate(); err != nil {
				return err
			}
			page.capture(cmd.Flags())
			// Unlike the other listings, the variables endpoint requires
			// 1 <= limit <= 200 and has no count-only mode (observed: 400
			// INVALID_QUERY "limit: Too small: expected number to be >=1").
			if !page.all && (page.limit < 1 || page.limit > 200) {
				return fmt.Errorf("--limit must be between 1 and 200 for variables; the service has no count-only mode")
			}
			if scope != "" {
				parsed, ok := authoring.ParseVariableScope(scope)
				if !ok {
					return fmt.Errorf("invalid --scope %q; options: org, team, testSuite, testCase", scope)
				}
				opts.Scope = parsed
			}
			// The service enforces the pairing on listing too (400 INVALID_QUERY);
			// checking here gives a precise message before any request.
			if err := authoring.ValidateScopePairing(opts.Scope, opts.TestSuiteID, opts.TestCaseID); err != nil {
				return err
			}
			items, total, err := fetchPage(cmd.Context(), page, "variables", func(ctx context.Context, lo authoring.ListOptions) (authoring.List[authoring.Variable], error) {
				opts.ListOptions = lo
				return variableService.ListVariables(ctx, opts)
			})
			if err != nil {
				return fmt.Errorf("failed to list variables: %w", err)
			}
			for i := range items {
				blankSecret(&items[i])
			}
			if out == JSONOutput {
				return renderJSON(authoring.List[authoring.Variable]{Items: items, Total: total})
			}
			renderVariableTable(items, total)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&scope, "scope", "", "Filter by scope: org, team, testSuite or testCase.")
	flags.StringVar(&opts.TestSuiteID, "test-suite-id", "", "Suite ID. Required with --scope testSuite.")
	flags.StringVar(&opts.TestCaseID, "test-case-id", "", "Test case ID. Required with --scope testCase.")
	flags.StringVar(&opts.Search, "search", "", "Case-insensitive substring match on the name.")
	page.bind(flags)
	registerScopeCompletion(cmd)

	return cmd
}

// registerScopeCompletion completes --scope with the four scopes.
func registerScopeCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("scope", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		names := make([]string, len(authoring.AllVariableScopes))
		for i, s := range authoring.AllVariableScopes {
			names[i] = string(s)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	})
}

// blankSecret removes a confidential variable's value before rendering,
// regardless of what the service returned, so a service change can never
// leak it (FR-022).
func blankSecret(v *authoring.Variable) {
	if v.IsSecret {
		v.Value = ""
	}
}

// displayValue renders a variable's value, or the placeholder for secrets.
func displayValue(v authoring.Variable) string {
	if v.IsSecret {
		return secretPlaceholder
	}
	return v.Value
}

// renderVariableTable prints the listing table, or a single line when empty.
// lastUpdate is shown raw because users paste it back as a token.
func renderVariableTable(items []authoring.Variable, total int) {
	if len(items) == 0 {
		fmt.Printf("No variables found (total: %d).\n", total)
		return
	}
	t := newTable()
	t.AppendHeader(table.Row{"ID", "Scope", "Name", "Secret", "Value", "Last Update"})
	for _, v := range items {
		t.AppendRow(table.Row{v.ID, v.Scope, v.Name, v.IsSecret, truncate(displayValue(v), 40), v.LastUpdate})
	}
	t.AppendFooter(listFooter(len(items), total, "variables"))
	fmt.Println(t.Render())
}

// VariablesGetCommand is `authoring variables get`.
func VariablesGetCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "get <id>",
		Short:        "Show a variable",
		Example:      `  saucectl authoring variables get 5e7a…`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			v, err := variableService.GetVariable(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get variable: %w", err)
			}
			blankSecret(&v)
			if out == JSONOutput {
				return renderJSON(v)
			}
			renderVariableDetail(v)
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")

	return cmd
}

// renderVariableDetail prints the two-column property view.
func renderVariableDetail(v authoring.Variable) {
	t := newTable()
	t.AppendHeader(table.Row{"Property", "Value"})
	t.AppendRow(table.Row{"ID", v.ID})
	t.AppendRow(table.Row{"Name", v.Name})
	t.AppendRow(table.Row{"Scope", v.Scope})
	t.AppendRow(table.Row{"Suite", orDash(v.TestSuiteID)})
	t.AppendRow(table.Row{"Test Case", orDash(v.TestCaseID)})
	t.AppendRow(table.Row{"Description", orDash(v.Description)})
	t.AppendRow(table.Row{"Secret", v.IsSecret})
	t.AppendRow(table.Row{"Value", displayValue(v)})
	t.AppendRow(table.Row{"Created", fmt.Sprintf("%s by %s", humanizeDate(v.CreationDate), orDash(v.CreatorUserName))})
	t.AppendRow(table.Row{"Last Update", v.LastUpdate})
	t.AppendRow(table.Row{"Modified By", orDash(v.LastModifierUserName)})
	fmt.Println(t.Render())
	fmt.Printf("Pass --expected-last-update %q to update or delete against exactly this version.\n", v.LastUpdate)
}
