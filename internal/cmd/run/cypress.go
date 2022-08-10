package run

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
)

// NewCypressCmd creates the 'run' command for Cypress.
func NewCypressCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              "cypress",
		Short:            "Run cypress tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::config::specPattern", args)

			exitCode, err := runCypress(cmd)
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
	sc.String("headless", "suite::headless", "", "Controls whether or not tests are run in headless mode (default: false)")
	sc.String("timeZone", "suite::timeZone", "", "Specifies timeZone for this test")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.StringSlice("npm.dependencies", "npm::dependencies", []string{}, "Specify local npm dependencies for saucectl to upload. These dependencies must already be installed in the local node_modules directory.")
	sc.Bool("npm.strictSSL", "npm::strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	return cmd
}

func runCypress(cmd *cobra.Command) (int, error) {
	p, err := cypress.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.SetCLIFlags(flags.CaptureCommandLineFlags(cmd.Flags()))
	if err := p.ApplyFlags(gFlags.selectedSuite); err != nil {
		return 1, err
	}
	p.SetDefaults()
	if !gFlags.noAutoTagging {
		p.AppendTags(ci.GetTags())
	}

	if err := p.Validate(); err != nil {
		return 1, err
	}

	regio := region.FromString(p.GetSauceCfg().Region)
	testcompClient.URL = regio.APIBaseURL()
	webdriverClient.URL = regio.WebDriverBaseURL()
	restoClient.URL = regio.APIBaseURL()
	appsClient.URL = regio.APIBaseURL()
	insightsClient.URL = regio.APIBaseURL()
	iamClient.URL = regio.APIBaseURL()
	restoClient.ArtifactConfig = p.GetArtifactsCfg().Download
	tracker := segment.New(!gFlags.disableUsageMetrics)

	defer func() {
		props := usage.Properties{}
		props.SetFramework("cypress").SetFVersion(p.GetVersion()).SetFlags(cmd.Flags()).SetSauceConfig(p.GetSauceCfg()).
			SetArtifacts(p.GetArtifactsCfg()).SetDocker(p.GetDocker()).SetNPM(p.GetNpm()).SetNumSuites(len(p.GetSuites())).SetJobs(captor.Default.TestResults).
			SetSlack(p.GetNotifications().Slack).SetSharding(p.IsSharded())

		tracker.Collect(cases.Title(language.English).String(fullCommandName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.GetArtifactsCfg())
	dockerProject, sauceProject := cypress.SplitSuites(p)
	if dockerProject.GetSuiteCount() != 0 {
		exitCode, err := runCypressInDocker(dockerProject)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}
	if sauceProject.GetSuiteCount() != 0 {
		return runCypressInSauce(sauceProject, regio)
	}

	return 0, nil
}

func runCypressInDocker(p cypress.Project) (int, error) {
	log.Info().Msg("Running Cypress in Docker")
	printTestEnv("docker")

	cd, err := docker.NewCypress(p, &testcompClient, &testcompClient, &restoClient, &restoClient, createReporters(p.GetReporter(), p.GetNotifications(), p.GetSauceCfg().Metadata, &testcompClient, &restoClient,
		"cypress", "docker"))
	if err != nil {
		return 1, err
	}

	p.CleanPackages()
	return cd.RunProject()
}

func runCypressInSauce(p cypress.Project, regio region.Region) (int, error) {
	log.Info().Msg("Running Cypress in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.CypressRunner{
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
			CCYReader:       &restoClient,
			MetadataService: &testcompClient,
			TunnelService:   &restoClient,
			InsightsService: &insightsClient,
			UserService:     &iamClient,
			Region:          regio,
			ShowConsoleLog:  p.IsShowConsoleLog(),
			Reporters: createReporters(p.GetReporter(), p.GetNotifications(), p.GetSauceCfg().Metadata, &testcompClient, &restoClient,
				"cypress", "sauce"),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.NewSearchStrategy(p.GetVersion(), p.GetRootDir()),
			NPMDependencies:        p.GetNpm().Dependencies,
		},
	}

	p.CleanPackages()
	return r.RunProject()
}
