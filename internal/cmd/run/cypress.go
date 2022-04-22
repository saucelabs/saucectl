package run

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/testcomposer"
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
			viper.Set("suite::config::testFiles", args)

			exitCode, err := runCypress(cmd, tcClient, restoClient, appsClient)
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
	sc.StringSlice("excludedTestFiles", "suite::config::excludedTestFiles", []string{}, "Exclude test files to skip the tests, using glob pattern")

	// Video & Screen(shots)
	sc.String("screenResolution", "suite::screenResolution", "", "The screen resolution")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	sc.String("shard", "suite::shard", "", "Controls whether or not (and how) tests are sharded across multiple machines, supported value: spec|concurrency")
	sc.String("headless", "suite::headless", "", "Controls whether or not tests are run in headless mode (default: false)")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.Bool("npm.strictSSL", "npm::strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	return cmd
}

func runCypress(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	p, err := cypress.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())
	p.Sauce.Metadata.SetDefaultBuild()

	if err := applyCypressFlags(&p); err != nil {
		return 1, err
	}

	cypress.SetDefaults(&p)

	if err := cypress.Validate(&p); err != nil {
		return 1, err
	}

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	regio := region.FromString(p.Sauce.Region)

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

	tracker := segment.New(!gFlags.disableUsageMetrics)

	defer func() {
		props := usage.Properties{}
		props.SetFramework("cypress").SetFVersion(p.Cypress.Version).SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetDocker(p.Docker).SetNPM(p.Npm).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).
			SetSlack(p.Notifications.Slack).SetSharding(cypress.IsSharded(p.Suites))
		tracker.Collect(strings.Title(fullCommandName(cmd)), props)
		_ = tracker.Close()
	}()

	if p.Artifacts.Cleanup {
		download.Cleanup(p.Artifacts.Download.Directory)
	}

	dockerProject, sauceProject := cypress.SplitSuites(p)
	if len(dockerProject.Suites) != 0 {
		exitCode, err := runCypressInDocker(dockerProject, tc, rs)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}
	if len(sauceProject.Suites) != 0 {
		return runCypressInSauce(sauceProject, regio, tc, rs, as)
	}

	return 0, nil
}

func runCypressInDocker(p cypress.Project, testco testcomposer.Client, rs resto.Client) (int, error) {
	log.Info().Msg("Running Cypress in Docker")
	printTestEnv("docker")

	cd, err := docker.NewCypress(p, &testco, &testco, &rs, &rs, createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &testco, &rs,
		"cypress", "docker"))
	if err != nil {
		return 1, err
	}

	cleanCypressPackages(&p)
	return cd.RunProject()
}

func runCypressInSauce(p cypress.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	log.Info().Msg("Running Cypress in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.CypressRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:    &as,
			JobStarter:         &tc,
			JobReader:          &rs,
			JobStopper:         &rs,
			JobWriter:          &tc,
			CCYReader:          &rs,
			MetadataService:    &tc,
			TunnelService:      &rs,
			Region:             regio,
			ShowConsoleLog:     p.ShowConsoleLog,
			ArtifactDownloader: &rs,
			Reporters: createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &tc, &rs,
				"cypress", "sauce"),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.NewSearchStrategy(p.Cypress.Version, p.RootDir),
		},
	}

	cleanCypressPackages(&p)
	return r.RunProject()
}

func applyCypressFlags(p *cypress.Project) error {
	if gFlags.selectedSuite != "" {
		if err := cypress.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	// Create an adhoc suite if "--name" is provided
	if p.Suite.Name != "" {
		p.Suites = []cypress.Suite{p.Suite}
	}

	return nil
}

func cleanCypressPackages(p *cypress.Project) {
	// Don't allow framework installation, it is provided by the runner
	version, hasFramework := p.Npm.Packages["cypress"]
	if hasFramework {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("cypress", p.Cypress.Version, []string{fmt.Sprintf("cypress@%s", version)}))
		p.Npm.Packages = config.CleanNpmPackages(p.Npm.Packages, []string{"cypress"})
	}
}
