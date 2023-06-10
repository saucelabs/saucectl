package run

import (
	"errors"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/puppeteer/replay"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
)

// NewReplayCmd creates the 'run' command for replay.
func NewReplayCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              "replay",
		Short:            "Replay chrome devtools recordings",
		Long:             "Unlike 'saucectl run', this command allows you to bypass the config file partially or entirely by configuring an adhoc suite (--name) via flags.",
		Example:          `saucectl run replay recording.json -c "" --name "My Suite"`,
		SilenceUsage:     true,
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::recordings", args)

			exitCode, err := runReplay(cmd, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()

	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs.")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")

	// Browser & Platform
	sc.String("browser", "suite::browserName", "chrome", "Set the browser to use. Only chrome is supported at this time.")
	sc.String("browserVersion", "suite::browserVersion", "", "Set the browser version to use. If not specified, the latest version will be used.")
	sc.String("platform", "suite::platform", "", "Run against this platform.")

	return cmd
}

func runReplay(cmd *cobra.Command, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := replay.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	if err := applyPuppeteerReplayFlags(&p); err != nil {
		return 1, err
	}
	replay.SetDefaults(&p)

	if err := replay.Validate(&p); err != nil {
		return 1, err
	}

	ss, err := replay.ShardSuites(p.Suites)
	if err != nil {
		return 1, err
	}
	p.Suites = ss

	regio := region.FromString(p.Sauce.Region)
	if regio == region.USEast4 {
		return 1, errors.New(msg.NoFrameworkSupport)
	}

	webdriverClient.URL = regio.WebDriverBaseURL()
	testcompClient.URL = regio.APIBaseURL()
	restoClient.URL = regio.APIBaseURL()
	appsClient.URL = regio.APIBaseURL()
	insightsClient.URL = regio.APIBaseURL()
	iamClient.URL = regio.APIBaseURL()

	restoClient.ArtifactConfig = p.Artifacts.Download

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	tracker := segment.DefaultTracker
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		props := usage.Properties{}
		props.SetFramework("puppeteer-replay").SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).
			SetSlack(p.Notifications.Slack).SetLaunchOrder(p.Sauce.LaunchOrder)
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	return runPuppeteerReplayInSauce(p, regio)
}

func runPuppeteerReplayInSauce(p replay.Project, regio region.Region) (int, error) {
	log.Info().Msg("Replaying chrome devtools recordings")

	r := saucecloud.ReplayRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: &appsClient,
			JobService: saucecloud.JobService{
				VDCStarter:    &webdriverClient,
				RDCStarter:    &rdcClient,
				VDCReader:     &restoClient,
				RDCReader:     &rdcClient,
				VDCWriter:     &testcompClient,
				VDCStopper:    &restoClient,
				VDCDownloader: &restoClient,
			},
			TunnelService:   &restoClient,
			MetadataService: &testcompClient,
			InsightsService: &insightsClient,
			UserService:     &iamClient,
			BuildService:    &restoClient,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
			Reporters: createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &testcompClient, &restoClient,
				"puppeteer-replay", "sauce"),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.ExactStrategy{},
			Retrier:                &retry.BasicRetrier{},
		},
	}

	return r.RunProject()
}

func applyPuppeteerReplayFlags(p *replay.Project) error {
	if gFlags.selectedSuite != "" {
		if err := replay.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	// Use the adhoc suite instead, if one is provided
	if p.Suite.Name != "" {
		p.Suites = []replay.Suite{p.Suite}
	}

	return nil
}
