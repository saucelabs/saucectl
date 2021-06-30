package run

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
	"os"
)

type puppeteerFlags struct {
	RootDir   string
	Suite     puppeteer.Suite
	Puppeteer puppeteer.Puppeteer
	NPM       config.Npm
}

// NewPuppeteerCmd creates the 'run' command for Puppeteer.
func NewPuppeteerCmd() *cobra.Command {
	lflags := puppeteerFlags{}

	cmd := &cobra.Command{
		Use:              "puppeteer",
		Short:            "Run puppeteer tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			lflags.Suite.TestMatch = args

			exitCode, err := runPuppeteer(cmd, lflags, tcClient, restoClient)
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

	// Puppeteer
	f.StringVar(&lflags.Puppeteer.Version, "puppeteer.version", "", "The Puppeteer version to use")

	// Misc
	f.StringVar(&lflags.RootDir, "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")

	// NPM
	f.StringVar(&lflags.NPM.Registry, "npm.registry", "", "Specify the npm registry URL")
	f.StringToStringVar(&lflags.NPM.Packages, "npm.packages", map[string]string{}, "Specify npm packages that are required to run tests")
	f.BoolVar(&lflags.NPM.StrictSSL, "npm.strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	return cmd
}

func runPuppeteer(cmd *cobra.Command, flags puppeteerFlags, tc testcomposer.Client, rs resto.Client) (int, error) {
	p, err := puppeteer.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	if err := applyPuppeteerFlags(cmd, &p, flags); err != nil {
		return 1, err
	}
	puppeteer.SetDefaults(&p)
	if err := puppeteer.Validate(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	rs.URL = regio.APIBaseURL()
	tc.URL = regio.APIBaseURL()

	return runPuppeteerInDocker(p, tc, rs)
}

func runPuppeteerInDocker(p puppeteer.Project, testco testcomposer.Client, rs resto.Client) (int, error) {
	log.Info().Msg("Running puppeteer in Docker")
	printTestEnv("docker")

	cd, err := docker.NewPuppeteer(p, &testco, &testco, &rs)
	if err != nil {
		return 1, err
	}
	return cd.RunProject()
}

func filterPuppeteerSuite(c *puppeteer.Project) error {
	for _, s := range c.Suites {
		if s.Name == gFlags.suiteName {
			c.Suites = []puppeteer.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", gFlags.suiteName)
}

func applyPuppeteerFlags(cmd *cobra.Command, p *puppeteer.Project, flags puppeteerFlags) error {
	if flags.Puppeteer.Version != "" {
		p.Puppeteer.Version = flags.Puppeteer.Version
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

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterPuppeteerSuite(p); err != nil {
			return err
		}
	}

	// Create an adhoc suite if "--name" is provided
	if flags.Suite.Name != "" {
		p.Suites = []puppeteer.Suite{flags.Suite}
	}

	for k, v := range gFlags.env {
		for ks := range p.Suites {
			s := &p.Suites[ks]
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = v
		}
	}

	return nil
}
