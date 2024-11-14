package ini

import (
	// imports embed to load .sauceignore
	_ "embed"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func CypressCmd() *cobra.Command {
	cfg := &initConfig{
		frameworkName: cypress.Kind,
	}

	cmd := &cobra.Command{
		Use:          "cypress",
		Short:        "Bootstrap a Cypress project.",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, _ []string) {
			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
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

	cmd.Flags().StringVar(&cfg.frameworkVersion, "version", "", "Cypress version.")
	cmd.Flags().StringVar(&cfg.cypressConfigFile, "cypress-config", "", "Path to the cypress config file.")
	cmd.Flags().StringVar(&cfg.platformName, "platform", "", "OS name and version, such as 'Windows 11' or 'macOS 13'.")
	cmd.Flags().StringVar(&cfg.browserName, "browser", "", "Browser name.")
	cmd.Flags().StringVar(&cfg.artifactWhenStr, "artifacts-when", "fail", "When to download artifacts.")

	return cmd
}

func configureCypress(cfg *initConfig) interface{} {
	return v1.Project{
		TypeDef: config.TypeDef{
			APIVersion: v1.APIVersion,
			Kind:       cypress.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: cfg.concurrency,
		},
		RootDir: ".",
		Cypress: v1.Cypress{
			Version:    cfg.frameworkVersion,
			ConfigFile: cfg.cypressConfigFile,
		},
		Suites: []v1.Suite{
			{
				Name:         fmt.Sprintf("cypress - %s - %s", cfg.platformName, cfg.browserName),
				PlatformName: cfg.platformName,
				Browser:      cfg.browserName,
				Config: v1.SuiteConfig{
					TestingType: "e2e",
					SpecPattern: []string{"**/*.*"},
				},
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

//go:embed sauceignore/cypress.sauceignore
var sauceignoreCypress string
