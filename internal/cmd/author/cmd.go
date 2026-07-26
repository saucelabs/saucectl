// Package author implements `saucectl author`, a thin CLI wrapper around the
// Sauce Labs AI Test Authoring API.
//
// It exists to close a gap in that API: there is no server-side way to
// detect "have I already authored this spec, and has it changed since," and
// no way to re-author an existing test case in place (every generate call
// mints a brand new test case ID). `saucectl author` owns that bookkeeping
// itself via a lockfile (see internal/authoring/lockfile.go) so that:
//
//   - unchanged specs are not needlessly re-authored on every invocation
//   - changed specs are replaced (new test case generated, added to the
//     suite, old one removed from the suite and deleted) rather than
//     accumulating duplicates
//   - the lockfile itself is the only thing that needs to be committed to
//     version control -- it's machine-generated provenance, not test code
package author

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

var (
	authoringService authoring.Service
	authoringTimeout = 5 * time.Minute
)

// sharedFlags are flags common to both `author` (single spec) and
// `author sync` (directory reconciliation).
type sharedFlags struct {
	testSuite   string
	tags        []string
	lockfile    string
	force       bool
	out         string
	maxSteps    int
	pollEvery   time.Duration
	pollFor     time.Duration
	targetJSON  string
	configOut   string
	concurrency int
	url         string
}

func addSharedFlags(cmd *cobra.Command, f *sharedFlags) {
	flags := cmd.Flags()
	flags.StringVar(&f.testSuite, "test-suite", "", "The name of the test suite to add the authored test case(s) to. Created automatically if it doesn't already exist.")
	flags.StringSliceVar(&f.tags, "tags", nil, "A comma separated list of tags to assign to authored test cases.")
	flags.StringVar(&f.lockfile, "lockfile", authoring.DefaultLockfileName, "Path to the authoring lockfile that tracks which spec produced which test case.")
	flags.BoolVar(&f.force, "force", false, "Re-author every spec, even if its content hash matches the lockfile.")
	flags.StringVarP(&f.out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.IntVar(&f.maxSteps, "max-steps", 0, "Caps the number of steps the AI may generate for a test case (0 = server default).")
	flags.DurationVar(&f.pollEvery, "poll-interval", 3*time.Second, "How often to poll the authoring API while a test case is being generated.")
	flags.DurationVar(&f.pollFor, "poll-timeout", 4*time.Minute, "How long to wait for test case generation to complete before giving up.")
	flags.StringVar(&f.targetJSON, "target-file", "", "Path to a JSON file describing the runSettings.target object (e.g. browser capabilities) to run authored test cases against. If omitted, a minimal Chrome desktop target is used as a placeholder -- adjust to match your account's actual capability schema.")
	flags.StringVar(&f.configOut, "config-out", "", "If set, write/update a saucectl run config file (kind: authoring) at this path listing every test suite currently referenced by the lockfile, so `saucectl run -c <path>` can execute them. Disabled by default so an existing config.yml for another framework is never clobbered.")
	flags.IntVar(&f.concurrency, "concurrency", 2, "The sauce::concurrency value to write into the generated --config-out file.")
	flags.StringVar(&f.url, "url", "", "The target URL to test against (sent as runSettings.testUrl), for specs that don't otherwise state one in their text. If a spec's text contains an http(s) URL, it's extracted automatically and this flag is only needed to override that.")
}

// Command returns the `author` command, which doubles as both a runnable
// leaf (single spec authoring) and a parent for `author sync`.
func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string
	var specPath string
	flags := &sharedFlags{}

	cmd := &cobra.Command{
		Use:          "author",
		Short:        "Author test cases from natural language specs via the Sauce AI Test Authoring API",
		Long: `author generates a Sauce Labs test case from a natural language spec file and
adds it to a named test suite, creating the suite if it doesn't already exist.

It tracks what it has already authored in a lockfile (.sauce/authoring-lock.yml
by default) so that re-running it against an unchanged spec is a no-op, and
re-running it against a changed spec replaces the old test case rather than
creating a duplicate.

Use 'saucectl author sync <dir>' to reconcile an entire directory of specs at
once, including retiring test cases whose source spec has been deleted.`,
		SilenceUsage:     true,
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				preRun(cmd, args)
			}

			reg := region.FromString(regio)
			if reg == region.None {
				return errors.New("invalid region")
			}
			if reg == region.Staging {
				usage.DefaultClient.Enabled = false
			}

			creds := credentials.Get()
			if !creds.IsSet() {
				return errors.New("no credentials configured; run `saucectl configure` or set SAUCE_USERNAME/SAUCE_ACCESS_KEY")
			}

			authoringService = http.NewAuthoringService(reg.APIBaseURL(), creds.Username, creds.AccessKey, authoringTimeout)

			return nil
		},
		Args: func(_ *cobra.Command, args []string) error {
			if specPath == "" {
				return errors.New("no spec file specified; use --spec")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.testSuite == "" {
				return errors.New("no test suite specified; use --test-suite")
			}
			if flags.out != "text" && flags.out != "json" {
				return errors.New("unknown output format")
			}

			return runSingleSpec(cmd, specPath, flags)
		},
	}

	pflags := cmd.PersistentFlags()
	pflags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.Flags().StringVar(&specPath, "spec", "", "Path to the spec file to author a test case from.")
	addSharedFlags(cmd, flags)

	cmd.AddCommand(
		SyncCommand(),
	)

	return cmd
}
