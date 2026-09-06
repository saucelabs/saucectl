package authoring

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// variableNamePattern is the service's constraint on names.
var variableNamePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// bindValueSourceFlags registers the three ways to supply a value.
func bindValueSourceFlags(cmd *cobra.Command, vs *valueSource) {
	flags := cmd.Flags()
	flags.StringVar(&vs.value, "value", "", "The value. Visible in shell history and the process list; prefer --value-from-env or --value-from-file for secrets.")
	flags.StringVar(&vs.envName, "value-from-env", "", "Read the value from this environment variable.")
	flags.StringVar(&vs.file, "value-from-file", "", "Read the value from this file, or - for standard input. One trailing newline is stripped.")
}

// VariablesCreateCommand is `authoring variables create`.
func VariablesCreateCommand() *cobra.Command {
	var out, scope string
	var opts authoring.CreateVariableOptions
	var vs valueSource

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a variable",
		Long: `Create a variable at organisation, team, suite or test case scope.

The value comes from exactly one of --value, --value-from-env or --value-from-file. With
none of them, an interactive session prompts (masked for --secret) and a non-interactive
one fails. A secret's value is stored separately and never returned by any command.`,
		Example: `  saucectl authoring variables create --scope org --name password --secret --value-from-env SAUCE_DEMO_PASSWORD
  echo -n "standard_user" | saucectl authoring variables create --scope team --name username --value-from-file -
  saucectl authoring variables create --scope testCase --test-case-id 6a88… --name coupon --value SAVE10`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			parsed, ok := authoring.ParseVariableScope(scope)
			if scope == "" || !ok {
				return fmt.Errorf("--scope is required; options: org, team, testSuite, testCase")
			}
			opts.Scope = parsed
			if err := authoring.ValidateScopePairing(opts.Scope, opts.TestSuiteID, opts.TestCaseID); err != nil {
				return err
			}
			if opts.Name == "" {
				return errors.New("--name is required")
			}
			if !variableNamePattern.MatchString(opts.Name) {
				return fmt.Errorf("invalid --name %q: use lowercase letters, digits and underscores only", opts.Name)
			}
			if len(opts.Description) > 1000 {
				return errors.New("--description must be at most 1000 characters")
			}

			vs.secret = opts.IsSecret
			value, err := resolveValue(vs, os.Stdin)
			if err != nil {
				return err
			}
			opts.Value = value

			v, err := variableService.CreateVariable(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("failed to create variable: %w", err)
			}
			blankSecret(&v)
			if out == JSONOutput {
				return renderJSON(v)
			}
			fmt.Printf("Created %s variable %q (%s) at %s scope.\n", secretWord(v.IsSecret), v.Name, v.ID, v.Scope)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&scope, "scope", "", "Scope: org, team, testSuite or testCase. Required.")
	flags.StringVar(&opts.TestSuiteID, "test-suite-id", "", "Suite ID. Required with --scope testSuite.")
	flags.StringVar(&opts.TestCaseID, "test-case-id", "", "Test case ID. Required with --scope testCase.")
	flags.StringVar(&opts.Name, "name", "", "Name: lowercase letters, digits and underscores (1–255 characters). Required.")
	flags.StringVar(&opts.Description, "description", "", "Description (at most 1000 characters).")
	flags.BoolVar(&opts.IsSecret, "secret", false, "Mark the value confidential: it is never displayed again.")
	bindValueSourceFlags(cmd, &vs)
	registerScopeCompletion(cmd)

	return cmd
}

// secretWord renders "secret" or "plain" for messages.
func secretWord(secret bool) string {
	if secret {
		return "secret"
	}
	return "plain"
}
