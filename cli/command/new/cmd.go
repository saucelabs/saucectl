package new

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	newUse     = "new"
	newShort   = "Start a new project"
	newLong    = `Some long description`
	newExample = "saucectl new"

	qs = []*survey.Question{
		{
			Name: "framework",
			Prompt: &survey.Select{
				Message: "Choose a framework:",
				Options: []string{"Cypress", "Playwright", "Puppeteer", "TestCafe"},
				Default: "Cypress",
			},
		},
		{
			Name: "region",
			Prompt: &survey.Select{
				Message: "Choose the sauce labs region:",
				Options: []string{"us-west-1", "eu-central-1"},
				Default: "us-west-1",
			},
		},
	}

	answers = struct {
		Framework string
		Platform  string
		Region    string
	}{}

	availablePlatformMap = map[string][]string{
		"testcafe": {"Windows 10", "macOS 11.00"},
	}
)

// Command creates the `new` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     newUse,
		Short:   newShort,
		Long:    newLong,
		Example: newExample,
		Run: func(cmd *cobra.Command, args []string) {
			if err := Run(cmd, cli, args); err != nil {
				log.Err(err).Msg("failed to execute new command")
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVarP(&answers.Framework, "framework", "f", "Cypress",
		"Selects the framework. Specifying this will skip the prompt.")
	cmd.Flags().StringVarP(&answers.Region, "region", "r", "us-west-1",
		"Selects the region. Specifying this will skip the prompt.")
	return cmd
}

// Run starts the new command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	creds := credentials.Get()
	if creds == nil {
		fmt.Println("\nIt looks you have not configured your SauceLab account !" +
			"\nTo enjoy SauceLabs capabilities, configure your account by running:\n$ saucectl configure")
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if showPrompt(cmd.Flags()) {
		err = survey.Ask(qs, &answers)
		if err != nil {
			return err
		}
	}

	answers.Framework = strings.ToLower(answers.Framework)
	if err := os.MkdirAll(filepath.Join(cwd, ".sauce"), 0777); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	if val, ok := availablePlatformMap[answers.Framework]; ok {
		answers.Platform, err = getPlatform(val)
		if err != nil {
			return err
		}
	}

	cfgFilePath := ".sauce/config.yml"

	r := region.FromString(answers.Region)

	tc := testcomposer.Client{
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		URL:         r.APIBaseURL(),
		Credentials: *credentials.Get(),
	}

	m, err := tc.Search(cmd.Context(), framework.SearchOptions{
		Name:             answers.Framework,
		FrameworkVersion: "",
	})
	if err != nil {
		return err
	}

	org, repo, tag, err := framework.GitReleaseSegments(&m)
	if err != nil {
		return err
	}
	rinfo := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", org, repo, tag)
	log.Info().Str("release", rinfo).Msg("Downloading template.")

	err = FetchAndExtractTemplate(org, repo, tag)
	if err != nil {
		return fmt.Errorf("no template available for %s (%s)", answers.Framework, err)
	}

	if _, ok := availablePlatformMap[answers.Framework]; ok {
		err = updatePlatform(cfgFilePath, answers.Platform)
		if err != nil {
			return err
		}
	}
	err = updateRegion(cfgFilePath, answers.Region)
	if err != nil {
		return err
	}

	fmt.Println("\nNew project bootstrapped successfully! You can now run:\n$ saucectl run")
	return nil
}

// Overwrite the region from the template archive
func updateRegion(cfgFile string, region string) error {
	cwd, _ := os.Getwd()
	cfgPath := filepath.Join(cwd, cfgFile)

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	oldString := "\n  region: us-west-1\n"
	replacement := "\n  region: " + region + "\n"

	replaced := strings.Replace(string(data), oldString, replacement, 1)
	return os.WriteFile(cfgPath, []byte(replaced), 0644)
}

func getPlatform(platforms []string) (string, error) {
	var answer string
	question := &survey.Select{
		Message: "Choose a platform:",
		Options: platforms,
		Default: platforms[0],
	}
	err := survey.AskOne(question, &answer, nil)
	return answer, err
}

// Overwrite the platform from users' request
func updatePlatform(cfgFile string, platform string) error {
	cwd, _ := os.Getwd()
	cfgPath := filepath.Join(cwd, cfgFile)

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	oldStr := `platformName: "windows 10"`
	replacement := fmt.Sprintf(`platform: "%s"`, platform)

	replaced := strings.Replace(string(data), oldStr, replacement, 1)
	return os.WriteFile(cfgPath, []byte(replaced), 0644)
}

func showPrompt(flags *pflag.FlagSet) bool {
	// Skip prompt if at least one flag is set.
	return !(flags.Lookup("framework").Changed || flags.Lookup("region").Changed)
}
