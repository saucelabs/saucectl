package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
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
			viper.Set("suite::testMatch", args)

			exitCode, err := runPuppeteer(cmd, tcClient, restoClient)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				sentry.CaptureError(err, sentry.Scope{
					Username:   credentials.Get().Username,
					ConfigFile: gFlags.cfgFilePath,
				})
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

	// Puppeteer
	sc.String("puppeteer.version", "puppeteer::version", "", "The Puppeteer version to use")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.Bool("npm.strictSSL", "npm::strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	sc.StringSlice("browserArgs", "suite::browserArgs", []string{}, "Pass browser args to puppeteer")
	sc.StringSlice("groups", "suite::groups", []string{}, "Pass groups to puppeteer")

	return cmd
}

func runPuppeteer(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client) (int, error) {
	p, err := puppeteer.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())
	p.Sauce.Metadata.ExpandEnv()

	// Normalize path to package.json file
	if p.Puppeteer.Version == "package.json" {
		p.Puppeteer.Version = filepath.Join(p.RootDir, p.Puppeteer.Version)
	}

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

	tracker := segment.New(!gFlags.disableUsageMetrics)

	defer func() {
		props := usage.Properties{}
		props.SetFramework("puppeteer").SetFVersion(p.Puppeteer.Version).SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetDocker(p.Docker).SetNPM(p.Npm).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).
			SetSlack(p.Notifications.Slack)
		tracker.Collect(strings.Title(fullCommandName(cmd)), props)
		_ = tracker.Close()
	}()

	if p.Artifacts.Cleanup {
		download.Cleanup(p.Artifacts.Download.Directory)
	}

	return runPuppeteerInDocker(p, tc, rs)
}

func runPuppeteerInDocker(p puppeteer.Project, testco testcomposer.Client, rs resto.Client) (int, error) {
	log.Info().Msg("Running puppeteer in Docker")
	printTestEnv("docker")

	cd, err := docker.NewPuppeteer(p, &testco, &testco, &rs, &rs, createReporters(p.Reporters, p.Notifications,
		p.Sauce.Metadata, &testco, &rs, "puppeteer", "docker"))
	if err != nil {
		return 1, err
	}

	cleanPuppeteerPackages(&p)
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

func cleanPuppeteerPackages(p *puppeteer.Project) {
	// Don't allow framework installation, it is provided by the runner
	ignoredPackages := []string{}
	puppeteerVersion, hasPuppeteer := p.Npm.Packages["puppeteer"]
	puppeteerCoreVersion, hasPuppeteerCore := p.Npm.Packages["puppeteer-core"]
	if hasPuppeteer {
		ignoredPackages = append(ignoredPackages, fmt.Sprintf("puppeteer@%s", puppeteerVersion))
	}
	if hasPuppeteerCore {
		ignoredPackages = append(ignoredPackages, fmt.Sprintf("puppeteer-core@%s", puppeteerCoreVersion))
	}
	if hasPuppeteer || hasPuppeteerCore {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("puppeteer", p.Puppeteer.Version, ignoredPackages))
		p.Npm.Packages = config.CleanNpmPackages(p.Npm.Packages, []string{"puppeteer", "puppeteer-core"})
	}
}
