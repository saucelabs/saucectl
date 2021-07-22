package run

import (
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
)

// NewPuppeteerCmd creates the 'run' command for Puppeteer.
func NewPuppeteerCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              "puppeteer",
		Short:            "Run puppeteer tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite.testMatch", args)

			exitCode, err := runPuppeteer(cmd, tcClient, restoClient)
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

	sc.Fset = cmd.Flags()

	sc.String("name", "suite.name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite.browser", "", "Run tests against this browser")

	// Puppeteer
	sc.String("puppeteer.version", "puppeteer.version", "", "The Puppeteer version to use")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")

	// NPM
	sc.String("npm.registry", "npm.registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm.packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.Bool("npm.strictSSL", "npm.strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	return cmd
}

func runPuppeteer(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client) (int, error) {
	p, err := puppeteer.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CommandLine = flags.CaptureCommandLineFlags(cmd)
	p.Sauce.Metadata.ExpandEnv()

	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	if err := applyPuppeteerFlags(&p); err != nil {
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

func applyPuppeteerFlags(p *puppeteer.Project) error {
	if gFlags.selectedSuite != "" {
		if err := puppeteer.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	// Use the adhoc suite instead, if one is provided
	if p.Suite.Name != "" {
		p.Suites = []puppeteer.Suite{p.Suite}
	}

	return nil
}
