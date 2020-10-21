package configure

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/spf13/cobra"
	"github.com/tj/survey"
	"os"
)

var (
	configureUse     = "configure"
	configureShort   = "Configure your Sauce Labs credentials"
	configureLong    = `Persist locally your Sauce Labs credentials`
	configureExample = "saucectl configure"

	qs = []*survey.Question{
		{
			Name: "username",
			Prompt: &survey.Input{
				Message: "SauceLabs username:",
				Default: "",
			},
		},
		{
			Name: "accessKey",
			Prompt: &survey.Input{
				Message: "SauceLabs access key:",
				Default: "",
			},
		},
	}
)

// Command creates the `new` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     configureUse,
		Short:   configureShort,
		Long:    configureLong,
		Example: configureExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start Configure Command")
			if err := Run(cmd, cli, args); err != nil {
				log.Err(err).Msg("failed to execute configure command")
				os.Exit(1)
			}
		},
	}
	return cmd
}

func askNotEmpty(prompt survey.Prompt, dest *string) error {
	prev := *dest
	for {
		// Add validator for format
		err := survey.AskOne(prompt, dest, nil)
		if err != nil {
			return err
		}
		// Keep old input
		if *dest == "" {
			*dest = prev
		}
		if *dest != "" {
			break
		}
	}
	return nil
}

// Explain why do this
func explainHowToObtainCredentials() {
	fmt.Printf(`
Don't have an account ? Signup here https://saucelabs.com/sign-up !
Already have an account ? Get your username and access key here: https://app.saucelabs.com/user-settings


`)
}

// Run starts the new command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	explainHowToObtainCredentials()

	fileCreds := credentials.GetCredentialsFromFile()
	envCreds := credentials.GetCredentialsFromEnv()

	defaultUsername := fileCreds.Username
	if defaultUsername == "" {
		defaultUsername = envCreds.Username
	}
	defaultAccessKey := fileCreds.AccessKey
	if defaultAccessKey == "" {
		defaultAccessKey = envCreds.AccessKey
	}
	creds := credentials.Credentials{
		AccessKey: defaultAccessKey,
		Username: defaultUsername,
	}
	usernameQuestion := &survey.Input{
		Message: "SauceLabs username",
		Default: defaultUsername,
	}

	accessKeyQuestion := &survey.Input{
		Message: "SauceLabs access key",
		Default: defaultAccessKey,
	}
	askNotEmpty(usernameQuestion, &creds.Username)
	askNotEmpty(accessKeyQuestion, &creds.AccessKey)

	if !creds.IsValid() {
		fmt.Println("\nCredentials provided looks invalid. They won't be saved.")
		return fmt.Errorf("invalid credentials")
	}

	if err := fileCreds.Store(); err != nil {
		return fmt.Errorf("unable to save credentials")
	}
	fmt.Println("\n\nYou're all set ! ")
	return nil
}
