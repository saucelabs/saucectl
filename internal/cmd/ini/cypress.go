package ini

import (
	// imports embed to load .sauceignore
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/cypress/v1alpha"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func CypressCmd() *cobra.Command {
	cfg := &initConfig{
		frameworkName: cypress.Kind,
	}

	cmd := &cobra.Command{
		Use:          "cypress",
		Short:        "Bootstrap a Cypress project.",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
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

	cmd.Flags().StringVar(&cfg.frameworkVersion, "version", "", "Cypress version.")
	cmd.Flags().StringVar(&cfg.cypressJSON, "cypress-config", "", "Path to cypress.json.")
	cmd.Flags().StringVar(&cfg.platformName, "platform", "", "Platform name.")
	cmd.Flags().StringVar(&cfg.browserName, "browser", "", "Browser name.")
	cmd.Flags().StringVar(&cfg.artifactWhenStr, "artifacts-download-when", "fail", "When to download artifacts.")

	return cmd
}

func configureCypress(cfg *initConfig) interface{} {
	versions := strings.Split(cfg.frameworkVersion, ".")
	version, err := strconv.Atoi(versions[0])
	if err != nil {
		log.Err(err).Msg("failed to parse framework version")
	}
	if version < 10 {
		return v1alpha.Project{
			TypeDef: config.TypeDef{
				APIVersion: v1alpha.APIVersion,
				Kind:       cypress.Kind,
			},
			Sauce: config.SauceConfig{
				Region:      cfg.region,
				Sauceignore: ".sauceignore",
				Concurrency: cfg.concurrency,
			},
			RootDir: ".",
			Cypress: v1alpha.Cypress{
				Version:    cfg.frameworkVersion,
				ConfigFile: cfg.cypressJSON,
			},
			Suites: []v1alpha.Suite{
				{
					Name:         fmt.Sprintf("cypress - %s - %s", cfg.platformName, cfg.browserName),
					PlatformName: cfg.platformName,
					Browser:      cfg.browserName,
					Config: v1alpha.SuiteConfig{
						TestFiles: []string{"**/*.*"},
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
			ConfigFile: cfg.cypressJSON,
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
