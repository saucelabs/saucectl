package ini

import (
	// imports embed to load .sauceignore
	_ "embed"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func PlaywrightCmd() *cobra.Command {
	cfg := &initConfig{
		frameworkName: playwright.Kind,
	}

	cmd := &cobra.Command{
		Use:          "playwright",
		Short:        "Bootstrap a Playwright project.",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, _ []string) {
			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Properties{}.SetFlags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()

			err := Run(cmd, cfg)
			if err != nil {
				log.Err(err).Msg("failed to execute init command")
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&cfg.frameworkVersion, "version", "", "Playwright version.")
	cmd.Flags().StringVar(&cfg.platformName, "platform", "", "OS name and version, such as 'Windows 11' or 'macOS 13'.")
	cmd.Flags().StringVar(&cfg.browserName, "browser", "", "Browser name.")
	cmd.Flags().StringVar(&cfg.playwrightProject, "project", "", "Playwright project name.")
	cmd.Flags().StringSliceVar(&cfg.testMatch, "test-match", []string{".*.spec.js"}, "Test file pattern to match against.")
	cmd.Flags().StringVar(&cfg.artifactWhenStr, "artifacts-when", "fail", "When to download artifacts.")

	return cmd
}

func configurePlaywright(cfg *initConfig) interface{} {
	return playwright.Project{
		TypeDef: config.TypeDef{
			APIVersion: playwright.APIVersion,
			Kind:       playwright.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: cfg.concurrency,
		},
		RootDir: ".",
		Playwright: playwright.Playwright{
			Version: cfg.frameworkVersion,
		},
		Suites: []playwright.Suite{
			{
				Name:         fmt.Sprintf("playwright - %s - %s", cfg.platformName, cfg.browserName),
				PlatformName: cfg.platformName,
				Params: playwright.SuiteConfig{
					BrowserName: cfg.browserName,
					Project:     cfg.playwrightProject,
				},
				TestMatch: cfg.testMatch,
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When:      cfg.artifactWhen,
				Directory: "./artifacts",
				Match:     []string{"*"},
			},
		},
	}
}

//go:embed sauceignore/playwright.sauceignore
var sauceignorePlaywright string
