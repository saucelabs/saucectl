package signup

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/spf13/cobra"
)

var (
	runUse     = "signup"
	runShort   = "Signup for Sauce Labs"
	runLong    = "Provide a web link for free trial signup"
	runExample = "saucectl signup"
)

// Command creates the `run` command
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start Signup Command")
			err := Run()
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				sentry.CaptureError(err, sentry.Scope{})
				backtrace.Report(err, nil, "")
				os.Exit(1)
			}
		},
	}
	return cmd
}

// Run runs the command
func Run() error {
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

Achieve digital confidence with the Sauce Labs Testrunner Toolkit

View and analyze test results online with a free Sauce Labs account:
https://bit.ly/saucectl-signup`

	fmt.Println(saucebotSignup)
	return nil
}
