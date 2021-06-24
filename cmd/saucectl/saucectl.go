package main

import (
	"fmt"
	"github.com/saucelabs/saucectl/cli/command/configure"
	initCmd "github.com/saucelabs/saucectl/cli/command/init"
	"github.com/saucelabs/saucectl/cli/command/run"
	"github.com/saucelabs/saucectl/cli/command/signup"
	"github.com/saucelabs/saucectl/cli/setup"
	"github.com/saucelabs/saucectl/internal/version"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"

	"github.com/getsentry/sentry-go"
	"github.com/saucelabs/saucectl/cli/command/new"
)

var (
	cmdUse   = "saucectl [OPTIONS] COMMAND [ARG...]"
	cmdShort = "saucectl"
	cmdLong  = `Please refer to our examples for how to setup saucectl for your project:

- https://github.com/saucelabs/saucectl-cypress-example
- https://github.com/saucelabs/saucectl-espresso-example
- https://github.com/saucelabs/saucectl-playwright-example
- https://github.com/saucelabs/saucectl-puppeteer-example
- https://github.com/saucelabs/saucectl-testcafe-example
- https://github.com/saucelabs/saucectl-xcuitest-example`
)

func main() {
	cmd := &cobra.Command{
		Use:              cmdUse,
		Short:            cmdShort,
		Long:             cmdLong,
		TraverseChildren: true,
		Version:          fmt.Sprintf("%s\n(build %s)", version.Version, version.GitCommit),
	}

	cmd.SetVersionTemplate("saucectl version {{.Version}}\n")
	cmd.Flags().BoolP("version", "v", false, "print version")

	verbosity := cmd.PersistentFlags().Bool("verbose", false, "turn on verbose logging")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		setupLogging(*verbosity)
		setupSentry()
		return nil
	}

	cmd.AddCommand(
		new.Command(),
		run.Command(),
		configure.Command(),
		initCmd.Command(),
		signup.Command(),
	)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupLogging(verbose bool) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.DurationFieldInteger = true
	timeFormat := "15:04:05"
	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		zerolog.TimeFieldFormat = time.RFC3339Nano
		timeFormat = "15:04:05.000"
	}

	zerolog.TimestampFunc = func() time.Time {
		return time.Now().In(time.Local)
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: timeFormat})
}

func setupSentry() {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         setup.SentryDSN,
		Environment: "production",
		Release:     fmt.Sprintf("saucectl@%s", version.Version),
		Debug:       false,
	})
	if err != nil {
		log.Debug().Err(err).Msg("Failed to setup sentry")
		return
	}
}
