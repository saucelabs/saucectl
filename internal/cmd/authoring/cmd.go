// Package authoring implements the `saucectl authoring` command group: the
// command-line surface of the Sauce Labs AI Test Authoring service — test
// cases, test suites, schedules, variables and artifacts. Running authored
// suites as part of `saucectl run` lives in internal/cmd/run and
// internal/authoring instead.
package authoring

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/usage"
)

// Service handles shared by every subcommand. They are set by the root
// command's pre-run and kept unexported so the four subgroups can share them
// without a global registry — the same choice internal/cmd/apit makes.
var (
	testCaseService   authoring.TestCaseService
	testSuiteService  authoring.TestSuiteService
	scheduleService   authoring.ScheduleService
	variableService   authoring.VariableService
	artifactService   authoring.ArtifactService
	entitlementReader authoring.EntitlementReader
	userService       iam.UserService

	// regio is the resolved region, needed to derive dashboard links.
	regio region.Region
	// currentUser is resolved by the entitlement gate and reused by commands
	// that default to the caller's own identity.
	currentUser iam.User
)

// Request timeouts. authoringTimeout bounds one request to the authoring
// service; listings can be heavy, so it is generous. iamTimeout bounds the
// user lookup behind the entitlement gate.
var (
	authoringTimeout = 2 * time.Minute
	iamTimeout       = 30 * time.Second
)

// Command creates the `authoring` command group. preRun is the root command's
// persistent pre-run (logging and usage setup), invoked first so behaviour
// matches every other group.
func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regionFlag string

	cmd := &cobra.Command{
		Use:   "authoring",
		Short: "Manage and run AI-authored tests",
		Long: `Manage Sauce Labs AI Test Authoring assets — test cases, test suites, schedules and
variables — and author new tests from a plain-language description.

To run authored suites as part of a pipeline, with reporters, artifact download and CI
exit codes, use 'saucectl run' with a 'kind: authoring' configuration instead.`,
		Example: `  saucectl authoring testcases list --tag smoke
  saucectl authoring testcases get 6a882c1dc8b4482c166e96c9 --show-steps
  saucectl authoring testsuites create --name "Checkout" --test-case 6a882c1dc8b4482c166e96c9
  saucectl authoring variables create --scope org --name password --secret --value-from-env PASSWORD`,
		SilenceUsage:     true,
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				preRun(cmd, args)
			}

			reg := region.FromString(regionFlag)
			if reg == region.None {
				return fmt.Errorf("invalid region %q; options: us-west-1, us-east-4, eu-central-1", regionFlag)
			}
			if reg == region.Staging {
				usage.DefaultClient.Enabled = false
			}
			regio = reg

			creds := credentials.Get()
			if !creds.IsSet() {
				return errors.New("no credentials set; run 'saucectl configure' or set SAUCE_USERNAME and SAUCE_ACCESS_KEY")
			}

			svc := http.NewAuthoringService(reg, creds, authoringTimeout)
			testCaseService = &svc
			testSuiteService = &svc
			scheduleService = &svc
			variableService = &svc
			artifactService = &svc
			entitlementReader = &svc

			iamClient := http.NewUserService(reg.APIBaseURL(), creds, iamTimeout)
			userService = &iamClient

			user, err := authoring.VerifyEntitlement(cmd.Context(), userService, entitlementReader)
			if err != nil {
				return err
			}
			currentUser = user
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&regionFlag, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, us-east-4, eu-central-1.")

	cmd.AddCommand(
		TestCasesCommand(),
		TestSuitesCommand(),
		SchedulesCommand(),
		VariablesCommand(),
		DownloadArtifactCommand(),
	)

	return cmd
}

// trackUsage reports the command invocation the way every other command does.
func trackUsage(cmd *cobra.Command) {
	tracker := usage.DefaultClient
	go func() {
		tracker.Collect(
			cmds.FullName(cmd),
			usage.Flags(cmd.Flags()),
		)
		_ = tracker.Close()
	}()
}

// requireArgs returns a cobra argument validator demanding exactly n
// non-empty positional arguments, named for the error message.
func requireArgs(names ...string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != len(names) {
			return fmt.Errorf("expected %d argument(s): %v", len(names), names)
		}
		for i, a := range args {
			if a == "" {
				return fmt.Errorf("argument %s must not be empty", names[i])
			}
		}
		return nil
	}
}
