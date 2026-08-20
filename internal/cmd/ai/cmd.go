package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/saucelabs/saucectl/internal/aiauthoring"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

var (
	testCaseService   aiauthoring.TestCaseService
	testCaseRunner    aiauthoring.TestCaseRunner
	testSuiteReader   aiauthoring.TestSuiteReader
	entitlementReader aiauthoring.EntitlementReader
	userService       iam.UserService
	appURL            string
	aiTimeout         = 1 * time.Minute
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:   "ai",
		Short: "Interact with Sauce Labs AI Test Authoring",
		// TODO: Unhide once AI Test Authoring support is ready for release.
		Hidden:           true,
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
			appURL = reg.AppBaseURL()

			creds := credentials.Get()
			if !creds.IsSet() {
				return errors.New("no credentials set; run `saucectl configure` to set them up")
			}

			aiService := http.NewAIAuthoringService(reg, creds, aiTimeout)
			testCaseService = &aiService
			testCaseRunner = &aiService
			testSuiteReader = &aiService
			entitlementReader = &aiService

			us := http.NewUserService(reg.APIBaseURL(), creds, aiTimeout)
			userService = &us

			return checkEntitlement(cmd.Context())
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, us-east-4, eu-central-1.")

	cmd.AddCommand(
		TestCasesCommand(),
		RunCommand(),
		RunSuiteCommand(),
	)

	return cmd
}

// checkEntitlement verifies that AI Test Authoring is part of the
// organization's plan. The gate fails closed, but distinguishes "not in your
// plan" from a failure to verify.
func checkEntitlement(ctx context.Context) error {
	user, err := userService.User(ctx)
	if err != nil {
		return fmt.Errorf("unable to verify the AI Test Authoring entitlement: %w", err)
	}

	enabled, err := entitlementReader.IsAIAuthoringEnabled(ctx, user.Organization.ID)
	if err != nil {
		return fmt.Errorf("unable to verify the AI Test Authoring entitlement: %w", err)
	}
	if !enabled {
		return errors.New("AI Test Authoring is not enabled for your organization; contact your Sauce Labs account executive to add it to your plan")
	}

	return nil
}
