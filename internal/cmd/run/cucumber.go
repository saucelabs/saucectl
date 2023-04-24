package run

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/ci"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cucumber"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::options::paths", args)

			exitCode, err := runCucumber(cmd, true)
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
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'. (sauce mode only)")

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

	go func() {
		props := usage.Properties{}
		props.SetFramework("playwright-cucumberjs").SetFVersion(p.Playwright.Version).SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetDocker(p.Docker).SetNPM(p.Npm).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).
			SetSlack(p.Notifications.Slack).SetSharding(cucumber.IsSharded(p.Suites)).SetLaunchOrder(p.Sauce.LaunchOrder)
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	dockerProject, sauceProject := cucumber.SplitSuites(p)
	if len(dockerProject.Suites) != 0 {
		exitCode, err := runCucumberInDocker(dockerProject)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}
	if len(sauceProject.Suites) != 0 {
		return runCucumberInCloud(sauceProject, regio)
	}

	return 0, nil
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

func runCucumberInDocker(p cucumber.Project) (int, error) {
	log.Info().Msg("Running Playwright-Cucumberjs in Docker")
	printTestEnv("docker")

	cd, err := docker.NewCucumber(p, &testcompClient, &testcompClient, &restoClient, &restoClient, createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &testcompClient, &restoClient,
		"cucumber", "docker"))
	if err != nil {
		return 1, err
	}

	p.Npm.Packages = cleanPlaywrightPackages(p.Npm, p.Playwright.Version)
	return cd.RunProject()
}

func runCucumberInCloud(p cucumber.Project, regio region.Region) (int, error) {
	log.Info().Msg("Running Playwright-Cucumberjs in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.CucumberRunner{
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
				"cucumber", "sauce"),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.NewSearchStrategy(p.Playwright.Version, p.RootDir),
			NPMDependencies:        p.Npm.Dependencies,
			Retrier:                &retry.BasicRetrier{},
		},
	}

	p.Npm.Packages = cleanPlaywrightPackages(p.Npm, p.Playwright.Version)
	return r.RunProject()
}
