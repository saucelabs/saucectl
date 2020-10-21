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
	cliUsername      = ""
	cliAccessKey     = ""
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
	cmd.Flags().StringVarP(&cliUsername, "username", "u", "", "username,, available on your saucelabs account")
	cmd.Flags().StringVarP(&cliAccessKey, "accessKey", "a", "", "accessKey, available on your saucelabs account")
	return cmd
}

// askNotEmpty asks the user to type in a value.
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

// explainHowToObtainCredentials explains how to get credentials
func explainHowToObtainCredentials() {
	fmt.Println("\nDon't have an account ? Signup here https://saucelabs.com/sign-up !")
	fmt.Println("Already have an account ? Get your username and access key here: https://app.saucelabs.com/user-settings\n\n")
}

// interactiveConfiguration expect user to manually type-in its credentials
func interactiveConfiguration() credentials.Credentials {
	explainHowToObtainCredentials()
	creds := getDefaultCredentials()

	usernameQuestion := &survey.Input{
		Message: "SauceLabs username",
		Default: creds.Username,
	}
	accessKeyQuestion := &survey.Input{
		Message: "SauceLabs access key",
		Default: creds.AccessKey,
	}
	askNotEmpty(usernameQuestion, &creds.Username)
	askNotEmpty(accessKeyQuestion, &creds.AccessKey)

	fmt.Println("\n")
	return creds
}

// Run starts the new command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	var creds credentials.Credentials

	if cliUsername == "" && cliAccessKey == "" {
		creds = interactiveConfiguration()
	} else {
		creds = credentials.Credentials{
			Username: cliUsername,
			AccessKey: cliAccessKey,
		}
	}

	if !creds.IsValid() {
		fmt.Println("Credentials provided looks invalid and won't be saved.")
		return nil
	}
	if err := creds.Store(); err != nil {
		return fmt.Errorf("unable to save credentials")
	}
	fmt.Println("You're all set ! ")
	return nil
}

// getDefaultCredentials returns first the file credentials, then the one founded in the env.
func getDefaultCredentials() credentials.Credentials {
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
	return credentials.Credentials{
		AccessKey: defaultAccessKey,
		Username:  defaultUsername,
	}
}
