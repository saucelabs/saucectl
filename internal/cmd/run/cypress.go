package run

import (
	"errors"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type cypressFlags struct {
	npmStrictSSL bool
}

// NewCypressCmd creates the 'run' command for Cypress.
func NewCypressCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}
	var cflags cypressFlags

	cmd := &cobra.Command{
		Use:              "cypress",
		Short:            "Run cypress tests",
		SilenceUsage:     true,
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::config::specPattern", args)

			exitCode, err := runCypress(cmd, cflags, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()
	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite::browser", "", "Run tests against this browser")
	sc.String("browserVersion", "suite::browserVersion", "", "The browser version (default: latest)")
	sc.String("platform", "suite::platformName", "", "Run tests against this platform")

	// Cypress
	sc.String("cypress.version", "cypress::version", "", "The Cypress version to use")
	sc.String("cypress.configFile", "cypress::configFile", "", "The path to the cypress.json config file")
	sc.String("cypress.key", "cypress::key", "", "The Cypress record key")
	sc.Bool("cypress.record", "cypress::record", false, "Whether or not to record tests to the cypress dashboard")
	sc.StringSlice("excludeSpecPattern", "suite::config::excludeSpecPattern", []string{}, "Exclude test files to skip the tests, using glob pattern")
	sc.String("testingType", "suite::config::testingType", "e2e", "Specify the type of tests to execute; either e2e or component. Defaults to e2e")

	// Video & Screen(shots)
	sc.String("screenResolution", "suite::screenResolution", "", "The screen resolution")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	sc.String("shard", "suite::shard", "", "Controls whether or not (and how) tests are sharded across multiple machines, supported value: spec|concurrency")
	sc.Bool("shardGrepEnabled", "suite::shardGrepEnabled", false, "When sharding is configured and the suite is configured to filter using cypress-grep, let saucectl filter tests before executing")
	sc.String("headless", "suite::headless", "", "Controls whether or not tests are run in headless mode (default: false)")
	sc.String("timeZone", "suite::timeZone", "", "Specifies timeZone for this test")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.StringSlice("npm.dependencies", "npm::dependencies", []string{}, "Specify local npm dependencies for saucectl to upload. These dependencies must already be installed in the local node_modules directory.")
	cmd.Flags().BoolVar(&cflags.npmStrictSSL, "npm.strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https.")

	// Deprecated flags
	_ = sc.Fset.MarkDeprecated("npm.registry", "please set the npm registries field in the Sauce configuration file")

	return cmd
}

func runCypress(cmd *cobra.Command, cflags cypressFlags, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := cypress.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.SetCLIFlags(flags.CaptureCommandLineFlags(cmd.Flags()))
	if err := p.ApplyFlags(gFlags.selectedSuite); err != nil {
		return 1, err
	}

	if cmd.Flags().Changed("npm.strictSSL") {
		p.SetNpmStrictSSL(&cflags.npmStrictSSL)
	}

	p.SetDefaults()
	if !gFlags.noAutoTagging {
		p.AppendTags(ci.GetTags())
	}

	if err := p.Validate(); err != nil {
		return 1, err
	}

	regio := region.FromString(p.GetSauceCfg().Region)
	if regio == region.USEast4 {
		return 1, errors.New(msg.NoFrameworkSupport)
	}

	tracker := usage.DefaultClient
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		tracker.Collect(
			cmds.FullName(cmd),
			usage.Framework("cypress", p.GetVersion()),
			usage.Flags(cmd.Flags()),
			usage.SauceConfig(p.GetSauceCfg()),
			usage.Artifacts(p.GetArtifactsCfg()),
			usage.NPM(p.GetNpm()),
			usage.NumSuites(len(p.GetSuites())),
			usage.Sharding(p.GetShardTypes(), p.GetShardOpts()),
			usage.SmartRetry(p.IsSmartRetried()),
			usage.Reporters(p.GetReporters()),
			usage.Node(p.GetNodeVersion()),
		)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.GetArtifactsCfg())

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
		ArtifactDownloadConfig: p.GetArtifactsCfg().Download,
	}
	buildService := http.NewBuildService(
		regio, creds.Username, creds.AccessKey, buildTimeout,
	)

	log.Info().
		Str("region", regio.String()).
		Str("tunnel", p.GetSauceCfg().Tunnel.Name).
		Msg("Running Cypress in Sauce Labs.")
	r := saucecloud.CypressRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:        &appsClient,
			JobService:             jobService,
			MetadataService:        &testcompClient,
			TunnelService:          &restoClient,
			InsightsService:        &insightsClient,
			UserService:            &iamClient,
			BuildService:           &buildService,
			Region:                 regio,
			ShowConsoleLog:         p.IsShowConsoleLog(),
			Reporters:              createReporters(p.GetReporters(), gFlags.async),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.NewSearchStrategy(p.GetVersion(), p.GetRootDir()),
			NPMDependencies:        p.GetNpm().Dependencies,
			Retrier: &retry.SauceReportRetrier{
				JobService:      jobService,
				ProjectUploader: &appsClient,
				Project:         p,
			},
		},
	}

	p.CleanPackages()
	return r.RunProject()
}
