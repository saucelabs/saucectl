package author

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/spf13/cobra"
)

// urlPattern matches the first http(s) URL in a spec's free text, so a
// spec like "Open a browser and go to https://www.saucedemo.com." can have
// its target URL extracted automatically without requiring separate
// frontmatter. Trailing punctuation that's clearly prose (a period, comma,
// closing paren/quote, etc.) rather than part of the URL is trimmed off.
var urlPattern = regexp.MustCompile(`https?://[^\s<>"')\]]+`)

// extractURL returns the first URL found in text, with trailing prose
// punctuation stripped, or "" if none is found.
func extractURL(text string) string {
	match := urlPattern.FindString(text)
	return strings.TrimRight(match, ".,;:!?")
}

// defaultTarget is a placeholder runSettings.target used when the caller
// doesn't supply --target-file. It almost certainly needs to be adjusted to
// match your account's actual capability schema -- it exists so the command
// is usable out of the box rather than failing on a required field.
var defaultTarget = map[string]interface{}{
	"capabilities": map[string]interface{}{
		"browserName": "chrome",
	},
}

// specResult describes the outcome of reconciling a single spec file.
type specResult struct {
	SpecPath   string `json:"specPath"`
	Action     string `json:"action"` // "created", "updated", "unchanged", "failed"
	TestCaseID string `json:"testCaseId,omitempty"`
	Error      string `json:"error,omitempty"`
}

func runSingleSpec(cmd *cobra.Command, specPath string, flags *sharedFlags) error {
	// Normalize so this spec's lockfile key matches what `author sync` would
	// use for the same file.
	specPath = filepath.Clean(specPath)

	lf, err := authoring.LoadLockfile(flags.lockfile)
	if err != nil {
		return fmt.Errorf("failed to read lockfile %s: %w", flags.lockfile, err)
	}

	// A test suite is optional here: with no --test-suite, the test case is
	// created standalone (no suite membership) rather than forced into one.
	var suiteID string
	if flags.testSuite != "" {
		suiteID, err = resolveOrCreateSuite(cmd.Context(), flags.testSuite)
		if err != nil {
			return fmt.Errorf("failed to resolve test suite %q: %w", flags.testSuite, err)
		}
	}

	printStart(flags.out, specPath)
	result, changed, err := syncSpec(cmd.Context(), &lf, specPath, suiteID, flags)
	printResult(flags.out, result)
	if err != nil {
		return err
	}

	if changed {
		if err := lf.Save(flags.lockfile); err != nil {
			return fmt.Errorf("failed to write lockfile %s: %w", flags.lockfile, err)
		}
	}

	if err := maybeWriteRunConfig(cmd, flags, lf); err != nil {
		return err
	}

	return renderResults(flags.out, []specResult{result})
}

// syncSpec reconciles a single spec file against the lockfile: unchanged
// specs are skipped, new/changed specs are (re-)authored and swapped into
// the suite. It mutates lf in place and reports whether lf actually changed
// (so callers only write the lockfile back to disk when needed).
func syncSpec(ctx context.Context, lf *authoring.Lockfile, specPath, suiteID string, flags *sharedFlags) (specResult, bool, error) {
	res := specResult{SpecPath: specPath}

	hash, err := authoring.HashSpec(specPath)
	if err != nil {
		res.Action = "failed"
		res.Error = err.Error()
		return res, false, fmt.Errorf("failed to hash %s: %w", specPath, err)
	}

	existing, hadEntry := lf.Entries[specPath]

	if !flags.force && !lf.Changed(specPath, hash) {
		res.Action = "unchanged"
		res.TestCaseID = existing.TestCaseID

		if suiteID != "" && suiteID != existing.TestSuiteID {
			// The spec itself hasn't changed, but this invocation targets a
			// different suite than last time -- most commonly, assigning a
			// previously-standalone test case (created via `add testcase`
			// with no --test-suite) to a suite after the fact. Reuse the
			// existing test case instead of regenerating one from scratch.
			if _, err := authoringService.UpdateTestSuite(ctx, suiteID, authoring.UpdateTestSuiteOptions{
				AddTestCases: []string{existing.TestCaseID},
			}); err != nil {
				res.Action = "failed"
				res.Error = err.Error()
				return res, false, fmt.Errorf("failed to add existing test case %s to suite %s: %w", existing.TestCaseID, suiteID, err)
			}
			if existing.TestSuiteID != "" {
				if _, err := authoringService.UpdateTestSuite(ctx, existing.TestSuiteID, authoring.UpdateTestSuiteOptions{
					RemoveTestCases: []string{existing.TestCaseID},
				}); err != nil {
					log.Warn().Err(err).Str("testCaseId", existing.TestCaseID).Msg("added test case to new suite but failed to remove it from its previous suite")
				}
			}

			lf.Entries[specPath] = authoring.Entry{
				Hash:          hash,
				TestCaseID:    existing.TestCaseID,
				TestSuiteID:   suiteID,
				TestSuiteName: flags.testSuite,
				SyncedAt:      time.Now().UTC(),
			}
			res.Action = "reassigned"
			return res, true, nil
		}

		return res, false, nil
	}

	intent, err := os.ReadFile(specPath)
	if err != nil {
		res.Action = "failed"
		res.Error = err.Error()
		return res, false, fmt.Errorf("failed to read spec %s: %w", specPath, err)
	}
	if len(intent) == 0 {
		res.Action = "failed"
		res.Error = "spec file is empty"
		return res, false, fmt.Errorf("spec file %s is empty", specPath)
	}
	if len(intent) > 20000 {
		res.Action = "failed"
		res.Error = "spec exceeds 20000 characters"
		return res, false, fmt.Errorf("spec %s exceeds the 20000 character limit for a test intent (%d chars)", specPath, len(intent))
	}

	target := defaultTarget
	if flags.targetJSON != "" {
		t, err := loadTargetFile(flags.targetJSON)
		if err != nil {
			return res, false, err
		}
		target = t
	}

	name := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))

	testURL := flags.url
	if testURL == "" {
		testURL = extractURL(string(intent))
	}
	if testURL == "" {
		res.Action = "failed"
		res.Error = "no URL found in spec and --url not set"
		return res, false, fmt.Errorf("spec %s doesn't mention a URL to test against; pass --url explicitly", specPath)
	}

	task, err := authoringService.GenerateTestCase(ctx, authoring.GenerateRequest{
		Name: name,
		Tags: flags.tags,
		PromptSettings: authoring.PromptSettings{
			Intent:   string(intent),
			MaxSteps: flags.maxSteps,
		},
		RunSettings: authoring.RunSettings{
			TestURL: testURL,
			Target:  target,
		},
	})
	if err != nil {
		res.Action = "failed"
		res.Error = err.Error()
		return res, false, fmt.Errorf("failed to submit %s for authoring: %w", specPath, err)
	}

	task, err = pollGenerateTask(ctx, task.TaskID, flags.pollEvery, flags.pollFor)
	if err != nil {
		res.Action = "failed"
		res.Error = err.Error()
		return res, false, fmt.Errorf("authoring %s failed: %w", specPath, err)
	}

	// Swap the new test case into the suite, and remove the old one (if
	// this was a replace, not a first-time create). Skipped entirely for a
	// standalone test case (no --test-suite given).
	if suiteID != "" {
		updateOpts := authoring.UpdateTestSuiteOptions{
			AddTestCases: []string{task.TestCaseID},
		}
		if hadEntry && existing.TestCaseID != "" && existing.TestSuiteID == suiteID {
			updateOpts.RemoveTestCases = []string{existing.TestCaseID}
		}
		if _, err := authoringService.UpdateTestSuite(ctx, suiteID, updateOpts); err != nil {
			res.Action = "failed"
			res.Error = err.Error()
			return res, false, fmt.Errorf("authored %s (test case %s) but failed to update suite %s: %w", specPath, task.TestCaseID, suiteID, err)
		}
	}

	// Best-effort cleanup of the superseded test case. Its removal from the
	// suite above already took effect, so a failure here just leaves an
	// orphaned-but-harmless test case behind rather than corrupting suite
	// membership.
	if hadEntry && existing.TestCaseID != "" && existing.TestCaseID != task.TestCaseID {
		if err := authoringService.DeleteTestCase(ctx, existing.TestCaseID); err != nil {
			log.Warn().Err(err).Str("testCaseId", existing.TestCaseID).Msg("failed to delete superseded test case; it has been removed from the suite but not deleted")
		}
	}

	lf.Entries[specPath] = authoring.Entry{
		Hash:          hash,
		TestCaseID:    task.TestCaseID,
		TestSuiteID:   suiteID,
		TestSuiteName: flags.testSuite,
		SyncedAt:      time.Now().UTC(),
	}

	res.TestCaseID = task.TestCaseID
	if hadEntry {
		res.Action = "updated"
	} else {
		res.Action = "created"
	}
	return res, true, nil
}

// pollGenerateTask polls GetGenerateTask until it reaches a terminal state
// or the timeout elapses.
func pollGenerateTask(ctx context.Context, taskID string, every, timeout time.Duration) (authoring.GenerateTask, error) {
	deadline := time.Now().Add(timeout)

	for {
		task, err := authoringService.GetGenerateTask(ctx, taskID)
		if err != nil {
			return authoring.GenerateTask{}, err
		}

		switch task.Status {
		case authoring.TaskCompleted:
			return task, nil
		case authoring.TaskFailed:
			return authoring.GenerateTask{}, fmt.Errorf("generation failed (%s): %s", task.ErrorCode, task.ErrorDetail)
		}

		if time.Now().After(deadline) {
			return authoring.GenerateTask{}, fmt.Errorf("timed out after %s waiting for task %s to complete", timeout, taskID)
		}

		select {
		case <-ctx.Done():
			return authoring.GenerateTask{}, ctx.Err()
		case <-time.After(every):
		}
	}
}

// resolveOrCreateSuite looks up a test suite by exact name match, creating
// it if it doesn't exist yet.
func resolveOrCreateSuite(ctx context.Context, name string) (string, error) {
	id, _, err := resolveOrCreateSuiteVerbose(ctx, name, nil)
	return id, err
}

// resolveOrCreateSuiteVerbose is like resolveOrCreateSuite, but also reports
// whether a new suite was actually created (as opposed to an existing one
// being resolved by name), and accepts tags to apply if creation happens.
// Used by `author add testsuite`, which needs to tell the two cases apart to
// report accurately.
func resolveOrCreateSuiteVerbose(ctx context.Context, name string, tags []string) (id string, created bool, err error) {
	suites, err := authoringService.ListTestSuites(ctx, authoring.ListTestSuiteOptions{Search: name})
	if err != nil {
		return "", false, err
	}

	for _, s := range suites {
		if strings.EqualFold(s.Name, name) {
			return s.ID, false, nil
		}
	}

	suite, err := authoringService.CreateTestSuite(ctx, name, tags)
	if err != nil {
		return "", false, err
	}
	return suite.ID, true, nil
}

func loadTargetFile(path string) (map[string]interface{}, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read target file %s: %w", path, err)
	}
	var target map[string]interface{}
	if err := json.Unmarshal(b, &target); err != nil {
		return nil, fmt.Errorf("failed to parse target file %s as JSON: %w", path, err)
	}
	return target, nil
}
