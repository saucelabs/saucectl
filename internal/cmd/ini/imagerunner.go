package ini

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func ImageRunnerCmd() *cobra.Command {
	cfg := &initConfig{
		frameworkName: imagerunner.Kind,
	}

	cmd := &cobra.Command{
		Use:          "imagerunner",
		Short:        "Bootstrap an Image Runner (Sauce Orchestrate) project.",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, _ []string) {
			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
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

	cmd.Flags().StringVar(&cfg.dockerImage, "image", "", "Docker image to use.")
	cmd.Flags().StringVar(&cfg.workload, "workload", "", "Type of work performed. Options: 'webdriver', 'other'.")
	cmd.Flags().StringVar(&cfg.artifactWhenStr, "artifacts-when", "fail", "When to download artifacts.")

	return cmd
}

func configureImageRunner(cfg *initConfig) interface{} {
	return imagerunner.Project{
		TypeDef: config.TypeDef{
			APIVersion: imagerunner.APIVersion,
			Kind:       imagerunner.Kind,
		},
		Sauce: config.SauceConfig{
			Region: cfg.region,
		},
		Suites: []imagerunner.Suite{
			{
				Name:  fmt.Sprintf("imagerunner - %s", cfg.dockerImage),
				Image: cfg.dockerImage,
				ImagePullAuth: imagerunner.ImagePullAuth{
					User:  "${DOCKER_USERNAME}",
					Token: "${DOCKER_PASSWORD}",
				},
				Workload: cfg.workload,
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

func displayExtraInfoImageRunner() {
	fmt.Println()
	color.HiGreen("Before running your tests, you need to set the following environment variables:")
	color.Green("  - DOCKER_USERNAME")
	color.Green("  - DOCKER_PASSWORD")
	fmt.Println()
}
