package configure

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/spf13/cobra"
)

var (
	configureUse           = "configure"
	configureShort         = "Configure your Sauce Labs credentials"
	configureLong          = `Persist locally your Sauce Labs credentials`
	configureExample       = "saucectl configure"
	cliUsername            = ""
	cliAccessKey           = ""
	cliDisableUsageMetrics = false
)

// Command creates the `configure` command
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     configureUse,
		Short:   configureShort,
		Long:    configureLong,
		Example: configureExample,
		Run: func(cmd *cobra.Command, args []string) {
			tracker := segment.New(!cliDisableUsageMetrics)

			defer func() {
				tracker.Collect("Configure", nil)
				_ = tracker.Close()
			}()

			if err := Run(); err != nil {
				log.Err(err).Msg("failed to execute configure command")
				sentry.CaptureError(err, sentry.Scope{
					Username: cliUsername,
				})
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&cliUsername, "username", "u", "", "username, available on your sauce labs account")
	cmd.Flags().StringVarP(&cliAccessKey, "accessKey", "a", "", "accessKey, available on your sauce labs account")
	cmd.Flags().BoolVar(&cliDisableUsageMetrics, "disable-usage-metrics", false, "Disable usage metrics collection.")
	return cmd
}

// interactiveConfiguration expect user to manually type-in its credentials
func interactiveConfiguration() (credentials.Credentials, error) {
	fmt.Println(msg.SignupMessage)

	creds := credentials.Get()

	println("") // visual paragraph break
	qs := []*survey.Question{
		{
			Name: "username",
			Prompt: &survey.Input{
				Message: "SauceLabs username",
				Default: creds.Username,
			},
			Validate: func(val interface{}) error {
				str, ok := val.(string)
				if !ok {
					return errors.New("invalid username")
				}
				str = strings.TrimSpace(str)
				if str == "" {
					return errors.New("you need to type a username")

				}
				return nil
			},
		},
		{
			Name: "accessKey",
			Prompt: &survey.Input{
				Message: "SauceLabs access key",
				Default: creds.AccessKey,
			},
			Validate: func(val interface{}) error {
				str, ok := val.(string)
				if !ok {
					return errors.New("invalid access key")
				}
				str = strings.TrimSpace(str)
				if str == "" {
					return errors.New("you need to type an access key")

				}
				return nil
			},
		},
	}

	if err := survey.Ask(qs, &creds); err != nil {
		return creds, err
	}
	println() // visual paragraph break
	return creds, nil
}

// Run starts the configure command
func Run() error {
	var creds credentials.Credentials
	var err error

	if cliUsername == "" && cliAccessKey == "" {
		creds, err = interactiveConfiguration()
	} else {
		creds = credentials.Credentials{
			Username:  cliUsername,
			AccessKey: cliAccessKey,
		}
	}
	if err != nil {
		return err
	}

	if !creds.IsValid() {
		log.Error().Msg("The provided credentials appear to be invalid and will NOT be saved.")
		return fmt.Errorf("invalid credentials provided")
	}
	if err := credentials.ToFile(creds); err != nil {
		return fmt.Errorf("unable to save credentials: %s", err)
	}
	println("You're all set!")
	return nil
}
