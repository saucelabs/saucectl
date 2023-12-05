package ini

import (
	// imports embed to load .sauceignore
	_ "embed"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func TestCafeCmd() *cobra.Command {
	cfg := &initConfig{
		frameworkName: testcafe.Kind,
	}

	cmd := &cobra.Command{
		Use:          "testcafe",
		Short:        "Bootstrap a TestCafe project.",
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

	cmd.Flags().StringVarP(&cfg.frameworkVersion, "framework-version", "v", "", "Framework version.")
	cmd.Flags().StringVarP(&cfg.platformName, "platform", "p", "", "Platform name.")
	cmd.Flags().StringVarP(&cfg.browserName, "browser", "b", "", "Browser name.")
	cmd.Flags().StringVar(&cfg.artifactWhenStr, "artifacts.download.when", "fail", "Defines when to download artifacts")
	cmd.Flags().Var(&cfg.simulatorFlag, "simulator", "The iOS simulator to use for testing.")

	return cmd
}

func configureTestcafe(cfg *initConfig) interface{} {
	return testcafe.Project{
		TypeDef: config.TypeDef{
			APIVersion: testcafe.APIVersion,
			Kind:       testcafe.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: cfg.concurrency,
		},
		RootDir: ".",
		Testcafe: testcafe.Testcafe{
			Version: cfg.frameworkVersion,
		},
		Suites: []testcafe.Suite{
			{
				Name:         fmt.Sprintf("testcafe - %s - %s", cfg.platformName, cfg.browserName),
				PlatformName: cfg.platformName,
				BrowserName:  cfg.browserName,
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

//go:embed sauceignore/testcafe.sauceignore
var sauceignoreTestcafe string
