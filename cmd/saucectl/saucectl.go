package main

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/cmd/artifacts"
	"github.com/saucelabs/saucectl/internal/cmd/completion"
	"github.com/saucelabs/saucectl/internal/cmd/configure"
	"github.com/saucelabs/saucectl/internal/cmd/imagerunner"
	"github.com/saucelabs/saucectl/internal/cmd/ini"
	"github.com/saucelabs/saucectl/internal/cmd/jobs"
	"github.com/saucelabs/saucectl/internal/cmd/new"
	"github.com/saucelabs/saucectl/internal/cmd/run"
	"github.com/saucelabs/saucectl/internal/cmd/signup"
	"github.com/saucelabs/saucectl/internal/cmd/storage"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/version"
	"github.com/spf13/cobra"
)

var (
	cmdUse   = "saucectl [OPTIONS] COMMAND [ARG...]"
	cmdShort = "saucectl"
	cmdLong  = `Please refer to our examples for how to setup saucectl for your project:

- https://github.com/saucelabs/saucectl-cypress-example
- https://github.com/saucelabs/saucectl-espresso-example
- https://github.com/saucelabs/saucectl-playwright-example
- https://github.com/saucelabs/saucectl-testcafe-example
- https://github.com/saucelabs/saucectl-xcuitest-example`
)

func main() {
	cmd := &cobra.Command{
		Use:              cmdUse,
		Short:            cmdShort,
		Long:             cmdLong,
		SilenceUsage:     true,
		TraverseChildren: true,
		Version:          fmt.Sprintf("%s\n(build %s)", version.Version, version.GitCommit),
	}

	cmd.SetVersionTemplate("saucectl version {{.Version}}\n")
	cmd.Flags().BoolP("version", "v", false, "print version")

	verbosity := cmd.PersistentFlags().Bool("verbose", false, "turn on verbose logging")
	noColor := cmd.PersistentFlags().Bool("no-color", false, "disable colorized output")
	noTracking := cmd.PersistentFlags().Bool("disable-usage-metrics", false, "Disable usage metrics collection.")

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		setupLogging(*verbosity, *noColor)
		segment.DefaultTracker.Enabled = !*noTracking
	}

	cmd.AddCommand(
		new.Command(),
		run.Command(),
		configure.Command(),
		ini.Command(),
		signup.Command(),
		completion.Command(),
		storage.Command(cmd.PersistentPreRun),
		artifacts.Command(cmd.PersistentPreRun),
		jobs.Command(cmd.PersistentPreRun),
		imagerunner.Command(cmd.PersistentPreRun),
	)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupLogging(verbose bool, noColor bool) {
	color.NoColor = noColor
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

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: timeFormat, NoColor: noColor})
}
