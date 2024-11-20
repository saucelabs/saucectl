package run

import (
	"errors"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/puppeteer/replay"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		PreRunE: func(_ *cobra.Command, _ []string) error {
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

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	tracker := usage.DefaultClient
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		tracker.Collect(
			cmds.FullName(cmd),
			usage.Framework("puppeteer-replay", ""),
			usage.Flags(cmd.Flags()),
			usage.SauceConfig(p.Sauce),
			usage.Artifacts(p.Artifacts),
			usage.NumSuites(len(p.Suites)),
		)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	return runPuppeteerReplayInSauce(p, regio)
}

func runPuppeteerReplayInSauce(p replay.Project, regio region.Region) (int, error) {
	log.Info().
		Str("region", regio.String()).
		Str("tunnel", p.Sauce.Tunnel.Name).
		Msg("Replaying chrome devtools recordings.")

	creds := regio.Credentials()
	restoClient := http.NewResto(regio, creds.Username, creds.AccessKey, 0)
	testcompClient := http.NewTestComposer(regio.APIBaseURL(), creds, testComposerTimeout)
	webdriverClient := http.NewWebdriver(regio, creds, webdriverTimeout)
	appsClient := *http.NewAppStore(regio.APIBaseURL(), creds.Username, creds.AccessKey, gFlags.appStoreTimeout)
	rdcClient := http.NewRDCService(regio, creds.Username, creds.AccessKey, rdcTimeout)
	insightsClient := http.NewInsightsService(regio.APIBaseURL(), creds, insightsTimeout)
	iamClient := http.NewUserService(regio.APIBaseURL(), creds, iamTimeout)
	buildService := http.NewBuildService(
		regio, creds.Username, creds.AccessKey, buildTimeout,
	)

	r := saucecloud.ReplayRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: &appsClient,
			JobService: saucecloud.JobService{
				RDC:                    rdcClient,
				Resto:                  restoClient,
				Webdriver:              webdriverClient,
				TestComposer:           testcompClient,
				ArtifactDownloadConfig: p.Artifacts.Download,
			},
			TunnelService:          &restoClient,
			MetadataService:        &testcompClient,
			InsightsService:        &insightsClient,
			UserService:            &iamClient,
			BuildService:           &buildService,
			Region:                 regio,
			ShowConsoleLog:         p.ShowConsoleLog,
			Reporters:              createReporters(p.Reporters, gFlags.async),
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
