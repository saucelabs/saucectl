package authoring

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rs/zerolog/log"
)

// valueSource is the set of flags a value can come from. Exactly one may be
// used; when none is, an interactive session prompts and a non-interactive one
// errors out naming every option (FR-023, SC-003).
type valueSource struct {
	// value is --value: works, but is visible in shell history and the
	// process list.
	value string
	// envName is --value-from-env: the name of an environment variable.
	envName string
	// file is --value-from-file: a path, or "-" for stdin.
	file string
	// secret selects a masked prompt and triggers the --value warning.
	secret bool
}

// set reports whether any source was given.
func (v valueSource) set() bool {
	return v.value != "" || v.envName != "" || v.file != ""
}

// promptValue asks for a value on the terminal, masked when secret. A
// variable so tests can replace it.
var promptValue = func(secret bool) (string, error) {
	var out string
	var err error
	if secret {
		err = survey.AskOne(&survey.Password{Message: "Value:"}, &out)
	} else {
		err = survey.AskOne(&survey.Input{Message: "Value:"}, &out)
	}
	return out, err
}

// stdinIsTerminal reports whether stdin is a terminal, in which case reading
// a value from it would hang waiting for input nobody knows to give. A
// variable for tests.
var stdinIsTerminal = func() bool { return isTerm(os.Stdin.Fd()) }

// resolveValue produces the value from exactly one source. stdin is what
// --value-from-file - reads.
func resolveValue(v valueSource, stdin io.Reader) (string, error) {
	sources := 0
	for _, s := range []string{v.value, v.envName, v.file} {
		if s != "" {
			sources++
		}
	}
	if sources > 1 {
		return "", errors.New("specify only one of --value, --value-from-env or --value-from-file")
	}

	switch {
	case v.envName != "":
		val, ok := os.LookupEnv(v.envName)
		if !ok || val == "" {
			return "", fmt.Errorf("environment variable %s is not set or empty", v.envName)
		}
		return val, nil

	case v.file == "-":
		if stdinIsTerminal() {
			return "", errors.New("--value-from-file - reads standard input, but standard input is a terminal; pipe the value in or use another source")
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("reading value from standard input: %w", err)
		}
		return stripTrailingNewline(string(b)), nil

	case v.file != "":
		b, err := os.ReadFile(v.file)
		if err != nil {
			return "", fmt.Errorf("reading value file: %w", err)
		}
		return stripTrailingNewline(string(b)), nil

	case v.value != "":
		if v.secret {
			log.Warn().Msg("A secret passed with --value is visible in shell history and the process list; prefer --value-from-env or --value-from-file.")
		}
		return v.value, nil
	}

	if !isInteractive() {
		return "", errors.New("no value given; supply one with --value-from-env NAME, --value-from-file PATH (or - for stdin), or --value")
	}
	return promptValue(v.secret)
}

// stripTrailingNewline removes exactly one trailing line ending, so
// `echo secret | ...` behaves as expected while a deliberately multi-line
// value keeps its inner newlines.
func stripTrailingNewline(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return strings.TrimSuffix(s, "\r")
}
