package author

import (
	"fmt"

	"github.com/spf13/cobra"
)

// AddCommand returns the `author add` command, a parent for two
// single-resource creation subcommands:
//
//	saucectl author add testsuite <name>
//	saucectl author add testcase <spec.md>
//	saucectl author add testcase <spec.md> --test-suite <name>
//
// Unlike `author sync`, which reconciles a whole directory against a suite,
// `add` operates on exactly one resource per invocation. It still goes
// through the same lockfile as `author`/`author sync` (so a spec added this
// way is tracked, skipped if unchanged on a later run, and replaced if it
// changes) rather than being a fire-and-forget one-shot.
func AddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "add",
		Short:        "Create a single test suite or test case",
		SilenceUsage: true,
	}

	cmd.AddCommand(addTestSuiteCommand(), addTestCaseCommand())

	return cmd
}

func addTestSuiteCommand() *cobra.Command {
	var tags []string

	cmd := &cobra.Command{
		Use:   "testsuite <name>",
		Short: "Create a test suite, or resolve an existing one with the same name",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] == "" {
				return fmt.Errorf("expected exactly one argument: the test suite name")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			id, created, err := resolveOrCreateSuiteVerbose(cmd.Context(), name, tags)
			if err != nil {
				return fmt.Errorf("failed to resolve test suite %q: %w", name, err)
			}

			if created {
				fmt.Printf("created    test suite %q (%s)\n", name, id)
			} else {
				fmt.Printf("exists     test suite %q (%s)\n", name, id)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&tags, "tags", nil, "A comma separated list of tags to assign to the test suite, if it needs to be created.")

	return cmd
}

func addTestCaseCommand() *cobra.Command {
	flags := &sharedFlags{}

	cmd := &cobra.Command{
		Use:   "testcase <spec.md>",
		Short: "Author a single test case from a spec file",
		Long: `add testcase generates a test case from a natural language spec file -- the
same underlying operation as the top-level 'author' command, but taking the
spec as a positional argument instead of --spec.

--test-suite is optional here. If omitted, the test case is created
standalone with no suite membership -- it won't be included in a suite run or
a --config-out-generated run config until it's assigned to one. Running the
same spec again later with --test-suite set reuses the existing test case
(it's added to that suite) rather than authoring a new one, as long as the
spec's content hasn't changed in the meantime.`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] == "" {
				return fmt.Errorf("expected exactly one argument: the spec file to author")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.out != "text" && flags.out != "json" {
				return fmt.Errorf("unknown output format")
			}
			return runSingleSpec(cmd, args[0], flags)
		},
	}

	addSharedFlags(cmd, flags)

	return cmd
}
