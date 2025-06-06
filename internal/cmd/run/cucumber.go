package run

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/ci"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cucumber"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewCucumberCmd creates the 'run' command for cucumber.
func NewCucumberCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              "cucumberjs",
		Short:            "Run Cucumber test",
		SilenceUsage:     true,
		Hidden:           true,
		TraverseChildren: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::options::paths", args)

			exitCode, err := runCucumber(cmd, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()

	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs.")
	sc.String("platformName", "suite::platformName", "", "Run against this platform.")

	// Cucumber
	sc.String("scenario-name", "suite::options::name", "", "Regular expressions of which scenario names should match one of to be run")
	sc.StringSlice("paths", "suite::options::paths", []string{}, "Paths to where the feature files are, using glob pattern")
	sc.StringSlice("excludedTestFiles", "suite::options::excludedTestFiles", []string{}, "Exclude test files to skip the tests, using glob pattern")
	sc.Bool("backtrace", "suite::options::backtrace", false, "Show the full backtrace for errors")
	sc.StringSlice("require", "suite::options::require", []string{}, "Paths to where your support code is, for CommonJS.")
	sc.StringSlice("import", "suite::options::import", []string{}, "Paths to where your support code is, for ESM")
	sc.StringSlice("scenario-tags", "suite::options::tags", []string{}, "Tag expression to filter which scenarios should be run")
	sc.StringSlice("format", "suite::options::format", []string{}, "Name/path and (optionally) output file path of each formatter to use")
	sc.Int("parallel", "suite::options::parallel", 0, "Run tests in parallel with the given number of worker processes, default: 0")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")

	return cmd
}

func runCucumber(cmd *cobra.Command, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := cucumber.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())

	if err := applyCucumberFlags(&p); err != nil {
		return 1, err
	}

	cucumber.SetDefaults(&p)

	if err := cucumber.Validate(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
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
			usage.Framework("playwright-cucumberjs", p.Playwright.Version),
			usage.Flags(cmd.Flags()),
			usage.SauceConfig(p.Sauce),
			usage.Artifacts(p.Artifacts),
			usage.NPM(p.Npm),
			usage.NumSuites(len(p.Suites)),
			usage.Sharding(cucumber.GetShardTypes(p.Suites), cucumber.GetShardOpts(p.Suites)),
			usage.SmartRetry(p.IsSmartRetried()),
			usage.Reporters(p.Reporters),
			usage.Node(p.NodeVersion),
		)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	creds := regio.Credentials()

	restoClient := http.NewResto(regio, creds.Username, creds.AccessKey, 0)
	testcompClient := http.NewTestComposer(regio.APIBaseURL(), creds, testComposerTimeout)
	webdriverClient := http.NewWebdriver(regio, creds, webdriverTimeout)
	appsClient := *http.NewAppStore(regio.APIBaseURL(), creds.Username, creds.AccessKey, gFlags.appStoreTimeout)
	rdcClient := http.NewRDCService(regio, creds.Username, creds.AccessKey, rdcTimeout)
	insightsClient := http.NewInsightsService(regio.APIBaseURL(), creds, insightsTimeout)
	iamClient := http.NewUserService(regio.APIBaseURL(), creds, iamTimeout)
	jobService := saucecloud.JobService{
		RDC:                    rdcClient,
		Resto:                  restoClient,
		Webdriver:              webdriverClient,
		TestComposer:           testcompClient,
		ArtifactDownloadConfig: p.Artifacts.Download,
	}
	buildService := http.NewBuildService(
		regio, creds.Username, creds.AccessKey, buildTimeout,
	)

	log.Info().
		Str("region", regio.String()).
		Str("tunnel", p.Sauce.Tunnel.Name).
		Msg("Running Playwright-Cucumberjs in Sauce Labs.")
	r := saucecloud.CucumberRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:        &appsClient,
			JobService:             jobService,
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
			MetadataSearchStrategy: framework.NewSearchStrategy(p.Playwright.Version, p.RootDir),
			NPMDependencies:        p.Npm.Dependencies,
			Retrier: &retry.SauceReportRetrier{
				JobService:      jobService,
				ProjectUploader: &appsClient,
				Project:         &p,
			},
		},
	}

	p.Npm.Packages = cleanPlaywrightPackages(p.Npm, p.Playwright.Version)
	return r.RunProject(cmd.Context())
}

func applyCucumberFlags(p *cucumber.Project) error {
	if gFlags.selectedSuite != "" {
		if err := cucumber.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	// Use the adhoc suite instead, if one is provided
	if p.Suite.Name != "" {
		p.Suites = []cucumber.Suite{p.Suite}
	}

	return nil
}
