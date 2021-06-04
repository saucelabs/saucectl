package run

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"path/filepath"
)

// NewEspressoCmd creates the 'run' command for espresso.
func NewEspressoCmd(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:              "espresso",
		Short:            "Run espresso tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := runEspressoCmd(cmd, cli, args)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				sentry.CaptureError(err, sentry.Scope{
					Username:   credentials.Get().Username,
					ConfigFile: cfgFilePath,
				})
			}
			os.Exit(exitCode)
		},
	}

	return cmd
}

// runEspressoCmd runs the espresso 'run' command.
func runEspressoCmd(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) (int, error) {
	creds := credentials.Get()
	if !creds.IsValid() {
		color.Red("\nSauceCTL requires a valid Sauce Labs account!\n\n")
		fmt.Println(`Set up your credentials by running:
> saucectl configure`)
		println()
		return 1, fmt.Errorf("no credentials set")
	}

	if cfgLogDir == defaultLogFir {
		pwd, _ := os.Getwd()
		cfgLogDir = filepath.Join(pwd, "logs")
	}
	cli.LogDir = cfgLogDir
	log.Info().Str("config", cfgFilePath).Msg("Reading config file")

	d, err := config.Describe(cfgFilePath)
	if err != nil {
		return 1, err
	}

	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         "", // updated later once region is determined
		Credentials: creds,
	}

	rs := resto.Client{
		HTTPClient: &http.Client{Timeout: restoTimeout},
		URL:        "", // updated later once region is determined
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	rc := rdc.Client{
		HTTPClient: &http.Client{
			Timeout: rdcTimeout,
		},
		Username:  creds.Username,
		AccessKey: creds.AccessKey,
	}

	as := appstore.New("", creds.Username, creds.AccessKey, appStoreTimeout)

	if d.Kind == config.KindEspresso && d.APIVersion == config.VersionV1Alpha {
		return runEspresso(cmd, tc, rs, rc, as)
	}

	return 1, errors.New("unknown framework configuration")
}

func runEspresso(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, rc rdc.Client, as *appstore.AppStore) (int, error) {
	p, err := espresso.FromFile(cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce)

	// TODO - add dry-run mode
	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	err = espresso.Validate(p)
	if err != nil {
		return 1, err
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterEspressoSuite(&p); err != nil {
			return 1, err
		}
	}

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()
	rc.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download
	rc.ArtifactConfig = p.Artifacts.Download

	return runEspressoInCloud(p, regio, tc, rs, rc, as)
}

func runEspressoInCloud(p espresso.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, rc rdc.Client, as *appstore.AppStore) (int, error) {
	log.Info().Msg("Running Espresso in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.EspressoRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:       as,
			JobStarter:            &tc,
			JobReader:             &rs,
			RDCJobReader:          &rc,
			JobStopper:            &rs,
			JobWriter:             &tc,
			CCYReader:             &rs,
			TunnelService:         &rs,
			Region:                regio,
			ShowConsoleLog:        false,
			ArtifactDownloader:    &rs,
			RDCArtifactDownloader: &rc,
		},
	}
	return r.RunProject()
}

func filterEspressoSuite(c *espresso.Project) error {
	for _, s := range c.Suites {
		if s.Name == suiteName {
			c.Suites = []espresso.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", suiteName)
}
