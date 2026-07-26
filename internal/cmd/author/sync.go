package author

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/spf13/cobra"
)

// SyncCommand returns the `author sync` command, which reconciles an entire
// directory of spec files against a test suite: new specs are authored,
// changed specs are replaced, and specs that have been deleted from disk
// have their corresponding test case retired (removed from the suite and
// deleted).
func SyncCommand() *cobra.Command {
	var ext string
	flags := &sharedFlags{}

	cmd := &cobra.Command{
		Use:          "sync <dir>",
		Short:        "Reconcile a directory of specs against a test suite",
		Long: `sync walks a directory of spec files and brings a test suite's membership in
line with what's on disk:

  - new specs are authored and added to the suite
  - specs whose content has changed since the last sync are re-authored and
    swap in for the old test case
  - specs that no longer exist on disk have their test case removed from the
    suite and deleted

Run this as its own step in CI (e.g. on merge to main), separate from
whatever step actually runs the suite -- authoring should never happen as
part of a regression run.`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] == "" {
				return fmt.Errorf("expected exactly one argument: the directory of specs to sync")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.testSuite == "" {
				return fmt.Errorf("no test suite specified; use --test-suite")
			}
			if flags.out != "text" && flags.out != "json" {
				return fmt.Errorf("unknown output format")
			}

			return runSync(cmd, args[0], ext, flags)
		},
	}

	cmd.Flags().StringVar(&ext, "ext", ".md", "File extension (including the dot) that identifies a spec file. Only files with this extension are considered.")
	addSharedFlags(cmd, flags)

	return cmd
}

func runSync(cmd *cobra.Command, dir string, ext string, flags *sharedFlags) error {
	lf, err := authoring.LoadLockfile(flags.lockfile)
	if err != nil {
		return fmt.Errorf("failed to read lockfile %s: %w", flags.lockfile, err)
	}

	suiteID, err := resolveOrCreateSuite(cmd.Context(), flags.testSuite)
	if err != nil {
		return fmt.Errorf("failed to resolve test suite %q: %w", flags.testSuite, err)
	}

	specs, err := findSpecs(dir, ext)
	if err != nil {
		return fmt.Errorf("failed to walk %s: %w", dir, err)
	}

	found := make(map[string]bool, len(specs))
	var results []specResult
	lockfileChanged := false

	for _, specPath := range specs {
		found[specPath] = true

		printStart(flags.out, specPath)
		res, changed, err := syncSpec(cmd.Context(), &lf, specPath, suiteID, flags)
		printResult(flags.out, res)
		if err != nil {
			// Report the failure but keep going -- one bad spec shouldn't
			// block reconciliation of the rest of the directory.
			results = append(results, res)
			lockfileChanged = lockfileChanged || changed
			continue
		}
		results = append(results, res)
		lockfileChanged = lockfileChanged || changed
	}

	// Retire any lockfile entries whose spec is no longer on disk. Only
	// entries pointing at this same suite are touched, so this sync doesn't
	// reach into suites managed by some other spec directory that happens to
	// share the same lockfile.
	for specPath, entry := range lf.Entries {
		if found[specPath] {
			continue
		}
		if entry.TestSuiteID != suiteID {
			continue
		}
		if !strings.HasPrefix(specPath, filepath.Clean(dir)+string(filepath.Separator)) && specPath != filepath.Clean(dir) {
			continue
		}

		printStart(flags.out, specPath)
		res := specResult{SpecPath: specPath, Action: "retired"}

		if _, err := authoringService.UpdateTestSuite(cmd.Context(), suiteID, authoring.UpdateTestSuiteOptions{
			RemoveTestCases: []string{entry.TestCaseID},
		}); err != nil {
			res.Action = "failed"
			res.Error = fmt.Sprintf("failed to remove retired test case %s from suite: %v", entry.TestCaseID, err)
			printResult(flags.out, res)
			results = append(results, res)
			continue
		}
		if err := authoringService.DeleteTestCase(cmd.Context(), entry.TestCaseID); err != nil {
			res.Error = fmt.Sprintf("removed from suite but failed to delete test case %s: %v", entry.TestCaseID, err)
		}

		printResult(flags.out, res)
		delete(lf.Entries, specPath)
		lockfileChanged = true
		results = append(results, res)
	}

	if lockfileChanged {
		if err := lf.Save(flags.lockfile); err != nil {
			return fmt.Errorf("failed to write lockfile %s: %w", flags.lockfile, err)
		}
	}

	if err := maybeWriteRunConfig(cmd, flags, lf); err != nil {
		return err
	}

	return renderResults(flags.out, results)
}

// findSpecs returns every file under dir with the given extension, as
// cleaned, slash-separated relative paths suitable for use as lockfile keys.
func findSpecs(dir string, ext string) ([]string, error) {
	var specs []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ext {
			return nil
		}
		specs = append(specs, filepath.Clean(path))
		return nil
	})
	if err != nil {
		return nil, err
	}

	return specs, nil
}
