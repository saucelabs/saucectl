package configure

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/segment"
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
				backtrace.Report(err, map[string]interface{}{
					"username": cliUsername,
				}, "")
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&cliUsername, "username", "u", "", "username, available on your sauce labs account")
	cmd.Flags().StringVarP(&cliAccessKey, "accessKey", "a", "", "accessKey, available on your sauce labs account")
	cmd.Flags().BoolVar(&cliDisableUsageMetrics, "disable-usage-metrics", false, "Disable usage metrics collection.")
	return cmd
}

func printCreds(creds credentials.Credentials) {
	println()

	labelStyle := color.New(color.Bold)
	valueStyle := color.New(color.FgBlue)

	fmt.Println("Currently configured credentials:")
	fmt.Println(labelStyle.Sprint("      Username:"), valueStyle.Sprint(creds.Username))
	fmt.Println(labelStyle.Sprint("    Access key:"), valueStyle.Sprint(mask(creds.AccessKey)))

	println()
	println()
}

// interactiveConfiguration expect user to manually type-in its credentials
func interactiveConfiguration() (credentials.Credentials, error) {
	overwrite := true
	var err error

	creds := credentials.Get()

	if creds.IsValid() {
		printCreds(creds)

		qs := &survey.Confirm{
			Message: "Overwrite existing credentials?",
		}
		err = survey.AskOne(qs, &overwrite)
		if err != nil {
			return creds, err
		}
	}

	if overwrite {
		if !creds.IsValid() {
			fmt.Println(msg.SignupMessage)
		}

		qs := []*survey.Question{
			{
				Name: "username",
				Prompt: &survey.Input{
					Message: "SauceLabs username",
				},
				Validate: func(val interface{}) error {
					str, ok := val.(string)
					if !ok {
						return errors.New(msg.InvalidUsername)
					}
					str = strings.TrimSpace(str)
					if str == "" {
						return errors.New(msg.EmptyUsername)

					}
					return nil
				},
			},
			{
				Name: "accessKey",
				Prompt: &survey.Password{
					Message: "SauceLabs access key",
				},
				Validate: func(val interface{}) error {
					str, ok := val.(string)
					if !ok {
						return errors.New(msg.InvalidAccessKey)
					}
					str = strings.TrimSpace(str)
					if str == "" {
						return errors.New(msg.EmptyAccessKey)

					}
					return nil
				},
			},
		}

		println() // visual paragraph break
		if err = survey.Ask(qs, &creds); err != nil {
			return creds, err
		}
		println() // visual paragraph break
	}

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
		return fmt.Errorf(msg.InvalidCredentials)
	}
	if err := credentials.ToFile(creds); err != nil {
		return fmt.Errorf("unable to save credentials: %s", err)
	}
	println("You're all set!")
	return nil
}

func mask(str string) string {
	n := len(str)
	if n == 0 {
		return ""
	}
	res := []byte{}
	for i := 0; i < n; i++ {
		if str[i] == '-' {
			res = append(res, str[i])
		} else if i >= n-4 {
			res = append(res, str[i])
		} else {
			res = append(res, '*')
		}
	}
	return string(res)
}
