package ini

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/xctest"
	"github.com/spf13/cobra"
)

func XCTestCmd() *cobra.Command {
	cfg := &initConfig{
		frameworkName: xctest.Kind,
	}

	cmd := &cobra.Command{
		Use:          "xctest",
		Short:        "Bootstrap an XCTest project.",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, _ []string) {
			tracker := usage.DefaultClient

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

	cmd.Flags().StringVar(&cfg.app, "app", "", "Path to application under test.")
	cmd.Flags().StringVar(&cfg.xctestRunFile, "xctest-run-file", "", "Path to xctest descriptor file.")
	cmd.Flags().StringSliceVar(&cfg.otherApps, "other-apps", []string{}, "Path to additional applications.")
	cmd.Flags().StringVar(&cfg.artifactWhenStr, "artifacts-when", "fail", "When to download artifacts.")
	// cmd.Flags().Var(&cfg.simulatorFlag, "simulator", "The iOS simulator to use for testing.")
	cmd.Flags().Var(&cfg.deviceFlag, "device", "The device to use for testing.")

	return cmd
}

func configureXCTest(cfg *initConfig) interface{} {
	suites := []xctest.Suite{}
	if cfg.device.Name != "" {
		suites = append(suites, xctest.Suite{
			Name:          fmt.Sprintf("xctest - %s", cfg.device.Name),
			Devices:       []config.Device{cfg.device},
			App:           cfg.app,
			XCTestRunFile: cfg.xctestRunFile,
			OtherApps:     cfg.otherApps,
		})
	}
	if cfg.simulator.Name != "" {
		suites = append(suites, xctest.Suite{
			Name:          fmt.Sprintf("xctest - %s", cfg.simulator.Name),
			Simulators:    []config.Simulator{cfg.simulator},
			App:           cfg.app,
			XCTestRunFile: cfg.xctestRunFile,
			OtherApps:     cfg.otherApps,
		})
	}
	return xctest.Project{
		TypeDef: config.TypeDef{
			APIVersion: xctest.APIVersion,
			Kind:       xctest.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Concurrency: cfg.concurrency,
		},
		Xctest: xctest.Xctest{
			OtherApps: cfg.otherApps,
		},
		Suites: suites,
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When:      cfg.artifactWhen,
				Directory: "./artifacts",
				Match:     []string{"*"},
			},
		},
	}
}
