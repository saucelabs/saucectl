package signup

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/spf13/cobra"
)

var (
	runUse     = "signup"
	runShort   = "Signup for Sauce Labs"
	runLong    = `TODO: Some long description about signup`
	runExample = "saucectl signup"

	defaultLogFir = "<cwd>/logs"
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start Signup Command")
			exitCode, err := Run(cmd, cli, args)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}
	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) (int, error) {
	saucebotSignup := `
                   (‾)
                   ||                          Puppeteer,
           ##################             /(    Playwright,
         ##                  ##         ,..%(    TestCafe,
        (#                   ##     .,,.....%(    Cypress!
       (##                   ##   ((((.......%(
        (##                  ##   ####
          ,##################    ## %##
                  ###             /###
           /################\    /##
         (#####/ sSSSs \##########)
       /######( sSSSSSs )#####
     ##/ ######\ sSSSs /######
  ####   #####################
##   ##     (####     #####
    ##      #####     #####
            #####     #####

Achieve digital confidence with Sauce Labs

View and analyze test results online with a Sauce Labs account:
https://bit.ly/saucectl-signup`

	fmt.Println(saucebotSignup)
	return 0, nil
}
