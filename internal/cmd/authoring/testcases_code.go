package authoring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// codeFlags are the flags of `testcases code`.
type codeFlags struct {
	out       string
	target    string
	filename  string
	targetDir string
	force     bool
}

// TestCasesCodeCommand is `authoring testcases code`: export a test case as
// source code in a chosen language and framework.
func TestCasesCodeCommand() *cobra.Command {
	var f codeFlags

	cmd := &cobra.Command{
		Use:   "code <id>",
		Short: "Export a test case as source code",
		Long: `Export the latest revision of a test case as source code for a language and framework.

Use 'list-code-targets' to see the targets available for a test case. Without --target, an
interactive session prompts for one. By default the source is printed to standard output
so it can be redirected; -f writes to a file and -d writes into a directory with a
filename derived from the target's language. Existing files are never overwritten
without --force.`,
		Example: `  saucectl authoring testcases code 6a882c1dc8b4482c166e96c9 --target typescript_playwright > login.spec.ts
  saucectl authoring testcases code 6a882c1dc8b4482c166e96c9 --target python_selenium -d tests/
  saucectl authoring testcases code 6a882c1dc8b4482c166e96c9 --target java_selenium -f LoginTest.java --force`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(f.out); err != nil {
				return err
			}
			if f.filename != "" && f.targetDir != "" {
				return errors.New("-f/--filename and -d/--target-dir are mutually exclusive")
			}
			return exportCode(cmd.Context(), args[0], f)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&f.out, "out", "o", TextOutput, "Output format when printing to standard output. Options: text (raw source), json.")
	flags.StringVar(&f.target, "target", "", "Code generation target, e.g. typescript_playwright. See list-code-targets.")
	flags.StringVarP(&f.filename, "filename", "f", "", "Write the source to this file.")
	flags.StringVarP(&f.targetDir, "target-dir", "d", "", "Write the source into this directory, deriving the filename from the target.")
	flags.BoolVar(&f.force, "force", false, "Overwrite an existing file.")

	_ = cmd.RegisterFlagCompletionFunc("target", func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 || testCaseService == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		targets, err := testCaseService.CodeTargets(cmd.Context(), args[0])
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return targets, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

// exportCode resolves the target, fetches the source and writes it where
// asked. Available targets are always resolved first: one cheap request buys
// a precise error instead of an opaque CODE_GENERATION_TARGET_NOT_FOUND.
func exportCode(ctx context.Context, id string, f codeFlags) error {
	targets, err := testCaseService.CodeTargets(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to list code targets: %w", err)
	}
	if len(targets) == 0 {
		return fmt.Errorf("test case %s has no code export targets", id)
	}

	target := f.target
	if target == "" {
		if !interactive() {
			return fmt.Errorf("--target is required; available: %s", strings.Join(targets, ", "))
		}
		if err := survey.AskOne(&survey.Select{Message: "Export target:", Options: targets}, &target); err != nil {
			return err
		}
	}
	if !contains(targets, target) {
		return fmt.Errorf("target %q is not available for this test case; available: %s", target, strings.Join(targets, ", "))
	}

	code, err := testCaseService.Code(ctx, id, target)
	if err != nil {
		if errors.Is(err, authoring.ErrTestCaseEmpty) {
			return fmt.Errorf("test case %s has no steps to export", id)
		}
		return fmt.Errorf("failed to export code: %w", err)
	}
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("test case %s produced no source: it is empty", id)
	}

	if f.filename == "" && f.targetDir == "" {
		if f.out == JSONOutput {
			return renderJSON(struct {
				Target string `json:"target"`
				Code   string `json:"code"`
			}{Target: target, Code: code})
		}
		fmt.Print(code)
		if !strings.HasSuffix(code, "\n") {
			fmt.Println()
		}
		return nil
	}

	dest := f.filename
	if dest == "" {
		tc, err := testCaseService.GetTestCase(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get test case for naming the file: %w", err)
		}
		name, known := deriveFilename(target, tc.Name, code)
		if !known {
			log.Warn().Msgf("Unknown language prefix in target %q; using %s. Use -f to choose a filename.", target, name)
		}
		dest = filepath.Join(f.targetDir, name)
	}

	if !f.force {
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("%s already exists; use --force to overwrite", dest)
		}
	}
	if dir := filepath.Dir(dest); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(dest, []byte(code), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}
	fmt.Printf("Wrote %d bytes to %s (%s).\n", len(code), dest, target)
	return nil
}

// contains reports whether list holds s.
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// javaClassPattern finds the public class a Java file must be named after.
var javaClassPattern = regexp.MustCompile(`public\s+(?:final\s+)?class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)

// deriveFilename picks a filename for the export from the target's language
// prefix (the part before the first underscore), so targets added later
// still get a sensible name. For Java the public class name is taken from the
// source, because javac requires the filename to match it. The boolean is
// false when the prefix is unknown and a generic name was used.
func deriveFilename(target, testName, code string) (string, bool) {
	prefix, _, _ := strings.Cut(target, "_")
	snake := snakeCase(testName)
	pascal := pascalCase(testName)

	switch strings.ToLower(prefix) {
	case "typescript":
		return snake + ".spec.ts", true
	case "javascript":
		return snake + ".spec.js", true
	case "python":
		return "test_" + snake + ".py", true
	case "csharp":
		return pascal + ".cs", true
	case "java":
		if m := javaClassPattern.FindStringSubmatch(code); len(m) == 2 {
			return m[1] + ".java", true
		}
		return pascal + ".java", true
	case "ruby":
		return snake + "_spec.rb", true
	case "kotlin":
		return pascal + ".kt", true
	default:
		return snake + ".txt", false
	}
}

// snakeCase lowercases and replaces runs of non-alphanumerics with one
// underscore; an empty result becomes "test".
func snakeCase(s string) string {
	var b strings.Builder
	lastUnderscore := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "test"
	}
	if unicode.IsDigit([]rune(out)[0]) {
		out = "test_" + out
	}
	return out
}

// pascalCase capitalises each alphanumeric run and drops separators; an
// empty result becomes "Test".
func pascalCase(s string) string {
	var b strings.Builder
	upperNext := true
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			upperNext = true
			continue
		}
		if upperNext {
			b.WriteRune(unicode.ToUpper(r))
			upperNext = false
		} else {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "Test"
	}
	if unicode.IsDigit([]rune(out)[0]) {
		out = "Test" + out
	}
	return out
}
