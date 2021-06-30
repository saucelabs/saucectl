package run

import (
	"errors"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
	"os"
)

type cypressFlags struct {
	RootDir string
	Suite   cypress.Suite
	Cypress cypress.Cypress
	NPM     config.Npm
}

// NewCypressCmd creates the 'run' command for Cypress.
func NewCypressCmd() *cobra.Command {
	lflags := cypressFlags{}

	cmd := &cobra.Command{
		Use:              "cypress",
		Short:            "Run cypress tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			lflags.Suite.Config.TestFiles = args

			exitCode, err := runCypress(cmd, lflags, tcClient, restoClient, appsClient)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				sentry.CaptureError(err, sentry.Scope{
					Username:   credentials.Get().Username,
					ConfigFile: gFlags.cfgFilePath,
				})
			}
			os.Exit(exitCode)
		},
	}

	f := cmd.Flags()
	f.StringVar(&lflags.Suite.Name, "name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	f.StringVar(&lflags.Suite.Browser, "browser", "", "Run tests against this browser")
	f.StringVar(&lflags.Suite.BrowserVersion, "browserVersion", "", "The browser version (default: latest)")
	f.StringVar(&lflags.Suite.PlatformName, "platformName", "", "Run tests against this platform")

	// Cypress
	f.StringVar(&lflags.Cypress.Version, "cypress.version", "", "The Cypress version to use")
	f.StringVar(&lflags.Cypress.ConfigFile, "cypress.configFile", "", "The path to the cypress.json config file")
	f.StringVar(&lflags.Cypress.Key, "cypress.key", "", "The Cypress record key")
	f.BoolVar(&lflags.Cypress.Record, "cypress.record", false, "Whether or not to record tests to the cypress dashboard")

	// Video & Screen(shots)
	f.StringVar(&lflags.Suite.ScreenResolution, "screenResolution", "", "The screen resolution")

	// Misc
	f.StringVar(&lflags.RootDir, "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")

	// NPM
	f.StringVar(&lflags.NPM.Registry, "npm.registry", "", "Specify the npm registry URL")
	f.StringToStringVar(&lflags.NPM.Packages, "npm.packages", map[string]string{}, "Specify npm packages that are required to run tests")
	f.BoolVar(&lflags.NPM.StrictSSL, "npm.strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	return cmd
}

func runCypress(cmd *cobra.Command, flags cypressFlags, tc testcomposer.Client, rs resto.Client, as appstore.AppStore) (int, error) {
	p, err := cypress.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.Sauce.Metadata.ExpandEnv()
	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	if err := applyCypressFlags(cmd, &p, flags); err != nil {
		return 1, err
	}
	cypress.SetDefaults(&p)

	if err := cypress.Validate(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", gFlags.regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download

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

	cd, err := docker.NewCypress(p, &testco, &testco, &rs)
	if err != nil {
		return 1, err
	}
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
			TunnelService:      &rs,
			Region:             regio,
			ShowConsoleLog:     p.ShowConsoleLog,
			ArtifactDownloader: &rs,
			DryRun:             gFlags.dryRun,
		},
	}
	return r.RunProject()
}

func applyCypressFlags(cmd *cobra.Command, p *cypress.Project, flags cypressFlags) error {
	if flags.Cypress.Version != "" {
		p.Cypress.Version = flags.Cypress.Version
	}
	if flags.Cypress.ConfigFile != "" {
		p.Cypress.ConfigFile = flags.Cypress.ConfigFile
	}
	if flags.Cypress.Key != "" {
		p.Cypress.Key = flags.Cypress.Key
	}
	if cmd.Flags().Changed("cyoress.record") {
		p.Cypress.Record = flags.Cypress.Record
	}

	if cmd.Flags().Changed("rootDir") || p.RootDir == "" {
		p.RootDir = flags.RootDir
	}

	if flags.NPM.Registry != "" {
		p.Npm.Registry = flags.NPM.Registry
	}

	if len(flags.NPM.Packages) != 0 {
		p.Npm.Packages = flags.NPM.Packages
	}

	if cmd.Flags().Changed("npm.strictSSL") {
		p.Npm.StrictSSL = flags.NPM.StrictSSL
	}

	if gFlags.showConsoleLog {
		p.ShowConsoleLog = true
	}
	if gFlags.runnerVersion != "" {
		p.RunnerVersion = gFlags.runnerVersion
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := cypress.FilterSuites(p, gFlags.suiteName); err != nil {
			return err
		}
	}

	// Create an adhoc suite if "--name" is provided
	if flags.Suite.Name != "" {
		p.Suites = []cypress.Suite{flags.Suite}
	}

	for k, v := range gFlags.env {
		for ks := range p.Suites {
			s := &p.Suites[ks]
			if s.Config.Env == nil {
				s.Config.Env = map[string]string{}
			}
			s.Config.Env[k] = v
		}
	}

	return nil
}
