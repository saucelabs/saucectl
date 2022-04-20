package run

import (
	"github.com/saucelabs/saucectl/internal/puppeteer/replay"
	"github.com/saucelabs/saucectl/internal/viper"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/usage"
)

// NewReplayCmd creates the 'run' command for replay.
func NewReplayCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              "replay",
		Short:            "Replay chrome devtools recordings",
		Long:             "Unlike 'saucectl run', this command allows you to bypass the config file partially or entirely by configuring an adhoc suite (--name) via flags.",
		Example:          `saucectl run replay recording.json -c "" --name "My Suite"`,
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::recordings", args)

			exitCode, err := runReplay(cmd, tcClient, restoClient, appsClient)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				backtrace.Report(err, map[string]interface{}{
					"username": credentials.Get().Username,
				}, gFlags.cfgFilePath)
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()

	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs.")

	// Browser & Platform
	sc.String("browser", "suite::browserName", "chrome", "Set the browser to use. Only chrome is supported at this time.")
	sc.String("browserVersion", "suite::browserVersion", "", "Set the browser version to use. If not specified, the latest version will be used.")
	sc.String("platform", "suite::platform", "", "Run against this platform.")

	return cmd
}

func runReplay(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	p, err := replay.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.Sauce.Metadata.ExpandEnv()

	if err := applyPuppeteerReplayFlags(&p); err != nil {
		return 1, err
	}
	replay.SetDefaults(&p)

	if err := replay.Validate(&p); err != nil {
		return 1, err
	}

	ss, err  := replay.ShardSuites(p.Suites)
	if err != nil {
		return 1, err
	}
	p.Suites = ss

	regio := region.FromString(p.Sauce.Region)

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	tracker := segment.New(!gFlags.disableUsageMetrics)

	defer func() {
		props := usage.Properties{}
		props.SetFramework("puppeteer-replay").SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).
			SetSlack(p.Notifications.Slack)
		tracker.Collect(strings.Title(fullCommandName(cmd)), props)
		_ = tracker.Close()
	}()

	if p.Artifacts.Cleanup {
		download.Cleanup(p.Artifacts.Download.Directory)
	}

	return runPuppeteerReplayInSauce(p, regio, tc, rs, as)
}

func runPuppeteerReplayInSauce(p replay.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	log.Info().Msg("Replaying chrome devtools recordings")
	printTestEnv("sauce")

	r := saucecloud.ReplayRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:    &as,
			JobStarter:         &tc,
			JobReader:          &rs,
			JobStopper:         &rs,
			JobWriter:          &tc,
			CCYReader:          &rs,
			TunnelService:      &rs,
			MetadataService:    &tc,
			Region:             regio,
			ShowConsoleLog:     p.ShowConsoleLog,
			ArtifactDownloader: &rs,
			Reporters: createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &tc, &rs,
				"puppeteer-replay", "sauce"),
			Async:    gFlags.async,
			FailFast: gFlags.failFast,
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
