package init

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

var (
	initUse     = "init"
	initShort   = "bootstrap project"
	initLong    = "bootstrap an existing project for Sauce Labs"
	initExample = "saucectl init"

	frameworkName = ""
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     initUse,
		Short:   initShort,
		Long:    initLong,
		Example: initExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start Init Command")
			err := Run(cmd, cli, args)
			if err != nil {
				log.Err(err).Msg("failed to execute init command")
				sentry.CaptureError(err, sentry.Scope{})
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&frameworkName, "framework", "f", "", "framework to init")
	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	// FIXME: Provision using API
	framework, _ := ask(frameworkSelector)

	switch strings.ToLower(framework) {
	case "espresso":
		return configureEspresso()
	case "xcuitest":
		return configureXCUITest()
	case "":
		return fmt.Errorf("interrupting configuration")
	default:
		return fmt.Errorf("%s: not implemented", strings.ToLower(framework))
	}
}