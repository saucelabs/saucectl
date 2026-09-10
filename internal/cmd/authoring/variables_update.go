package authoring

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// variableUpdateFlags are the flags of `variables update`.
type variableUpdateFlags struct {
	out                string
	name               string
	description        string
	secret             bool
	expectedLastUpdate string
	vs                 valueSource
}

// VariablesUpdateCommand is `authoring variables update`.
func VariablesUpdateCommand() *cobra.Command {
	var f variableUpdateFlags

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Change a variable's name, description, value or secrecy",
		Long: `Change a variable. Only the given flags change.

Changes are guarded against concurrent edits: by default the variable is read first and
its current version is sent along, so a colleague's change made in between is detected
and refused rather than overwritten. Pass --expected-last-update to pin a version you
read earlier instead. There is deliberately no --force: re-reading and re-sending is
already the default.`,
		Example: `  saucectl authoring variables update 5e7a… --value-from-env NEW_PASSWORD
  saucectl authoring variables update 5e7a… --secret=true
  saucectl authoring variables update 5e7a… --description "Rotated 2026-09" --expected-last-update 2026-09-05T14:59:59.000Z`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(f.out); err != nil {
				return err
			}
			return updateVariable(cmd.Context(), cmd, args[0], f)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&f.out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&f.name, "name", "", "New name: lowercase letters, digits and underscores.")
	flags.StringVar(&f.description, "description", "", "New description.")
	flags.BoolVar(&f.secret, "secret", false, "Set whether the value is confidential (--secret=true or --secret=false).")
	flags.StringVar(&f.expectedLastUpdate, "expected-last-update", "", "The lastUpdate value from an earlier read; the update fails if the variable changed since. Default: read the current value first.")
	bindValueSourceFlags(cmd, &f.vs)

	return cmd
}

// updateVariable assembles the change and applies it with concurrency
// control, surfacing a version conflict as a message that names the command
// to re-read with (FR-024).
func updateVariable(ctx context.Context, cmd *cobra.Command, id string, f variableUpdateFlags) error {
	var opts authoring.UpdateVariableOptions
	changed := false

	if cmd.Flags().Changed("name") {
		if !variableNamePattern.MatchString(f.name) {
			return fmt.Errorf("invalid --name %q: use lowercase letters, digits and underscores only", f.name)
		}
		v := f.name
		opts.Name = &v
		changed = true
	}
	if cmd.Flags().Changed("description") {
		if len(f.description) > 1000 {
			return errors.New("--description must be at most 1000 characters")
		}
		v := f.description
		opts.Description = &v
		changed = true
	}
	if cmd.Flags().Changed("secret") {
		v := f.secret
		opts.IsSecret = &v
		changed = true
	}
	// The variable is read when we need its concurrency token, and also
	// whenever a new value is being supplied: the shell-history warning has
	// to key off the *stored* secrecy, because rotating an existing secret
	// with --value is precisely the case the warning exists for and
	// --secret is not repeated on such a command.
	needCurrent := f.expectedLastUpdate == "" || f.vs.set()
	var current authoring.Variable
	if needCurrent {
		var err error
		current, err = variableService.GetVariable(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to read variable before updating: %w", err)
		}
	}

	if f.vs.set() {
		f.vs.secret = current.IsSecret
		if cmd.Flags().Changed("secret") {
			f.vs.secret = f.vs.secret || f.secret
		}
		value, err := resolveValue(f.vs, os.Stdin)
		if err != nil {
			return err
		}
		opts.Value = &value
		changed = true
	}
	if !changed {
		return errors.New("nothing to update: specify --name, --description, --secret or a value source")
	}

	token := f.expectedLastUpdate
	if token == "" {
		token = current.LastUpdate
	}
	opts.ExpectedLastUpdate = token

	v, err := variableService.UpdateVariable(ctx, id, opts)
	if err != nil {
		if errors.Is(err, authoring.ErrVariableVersionConflict) {
			return fmt.Errorf("variable %s was changed by someone else since it was read; re-read it with 'saucectl authoring variables get %s' and try again", id, id)
		}
		return fmt.Errorf("failed to update variable: %w", err)
	}
	blankSecret(&v)
	if f.out == JSONOutput {
		return renderJSON(v)
	}
	fmt.Printf("Updated %s variable %q (%s); new version %s.\n", secretWord(v.IsSecret), v.Name, v.ID, v.LastUpdate)
	return nil
}
