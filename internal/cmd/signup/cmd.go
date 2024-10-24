package signup

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
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
		Use:          runUse,
		Short:        runShort,
		Long:         runLong,
		Example:      runExample,
		SilenceUsage: true,
		Run: func(_ *cobra.Command, _ []string) {
			log.Info().Msg("Start Signup Command")
			err := Run()
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				os.Exit(1)
			}
		},
	}
	return cmd
}

// Run runs the command
func Run() error {
	signupMessage := `
Achieve digital confidence with the Sauce Labs Testrunner Toolkit

View and analyze test results online with a free Sauce Labs account:
https://bit.ly/saucectl-signup`

	fmt.Println(signupMessage)
	return nil
}
