package authoring

import (
	"errors"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
)

// ErrAborted is returned when the user declines a confirmation prompt.
var ErrAborted = errors.New("aborted")

// ErrConfirmationRequired is returned when a destructive command runs without
// a terminal to ask on and without an explicit bypass.
var ErrConfirmationRequired = errors.New("refusing to proceed without confirmation: not running interactively; re-run with --yes to confirm")

// promptConfirm asks the yes/no question on the terminal. It is a variable so
// tests can replace it; the default uses survey like the rest of the tool.
var promptConfirm = func(question string) (bool, error) {
	var ok bool
	err := survey.AskOne(&survey.Confirm{Message: question, Default: false}, &ok)
	return ok, err
}

// isInteractive reports whether a prompt can be shown. A variable for tests.
var isInteractive = interactive

// confirmDestructive gates every removal of a shared asset (FR-037–041). The
// behaviour is identical across all four asset types so it is learned once:
//
//	interactive, no --yes      → print what is affected, then prompt
//	interactive, --yes         → proceed silently
//	non-interactive, no --yes  → refuse with ErrConfirmationRequired
//	non-interactive, --yes     → proceed
//
// This deliberately departs from `storage delete`, which acts immediately:
// these are organisation-level assets that colleagues depend on, with no
// undo. The refusal, rather than a hang or an unconfirmed proceed, is what
// makes the command safe in a pipeline (FR-040).
func confirmDestructive(yes bool, what string, affects []string) error {
	if yes {
		return nil
	}
	if !isInteractive() {
		return fmt.Errorf("%w (%s)", ErrConfirmationRequired, what)
	}

	fmt.Fprintf(os.Stdout, "About to delete %s.\n", what)
	for _, a := range affects {
		fmt.Fprintf(os.Stdout, "  - %s\n", a)
	}

	ok, err := promptConfirm("Proceed?")
	if err != nil {
		return err
	}
	if !ok {
		return ErrAborted
	}
	return nil
}
