package ini

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"github.com/spf13/cobra"
)

func XCUITestCmd() *cobra.Command {
	cfg := &initConfig{
		frameworkName: xcuitest.Kind,
	}

	cmd := &cobra.Command{
		Use:          "xcuitest",
		Short:        "Bootstrap an XCUITest project.",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, _ []string) {
			tracker := segment.DefaultClient

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
	cmd.Flags().StringVar(&cfg.testApp, "test-app", "", "Path to test application.")
	cmd.Flags().StringSliceVar(&cfg.otherApps, "other-apps", []string{}, "Path to additional applications.")
	cmd.Flags().StringVar(&cfg.artifactWhenStr, "artifacts-when", "fail", "When to download artifacts.")
	cmd.Flags().Var(&cfg.simulatorFlag, "simulator", "The iOS simulator to use for testing.")
	cmd.Flags().Var(&cfg.deviceFlag, "device", "The device to use for testing.")

	return cmd
}

func configureXCUITest(cfg *initConfig) interface{} {
	suites := []xcuitest.Suite{}
	if cfg.device.Name != "" {
		suites = append(suites, xcuitest.Suite{
			Name:      fmt.Sprintf("xcuitest - %s", cfg.device.Name),
			Devices:   []config.Device{cfg.device},
			App:       cfg.app,
			TestApp:   cfg.testApp,
			OtherApps: cfg.otherApps,
		})
	}
	if cfg.simulator.Name != "" {
		suites = append(suites, xcuitest.Suite{
			Name:       fmt.Sprintf("xcuitest - %s", cfg.simulator.Name),
			Simulators: []config.Simulator{cfg.simulator},
			App:        cfg.app,
			TestApp:    cfg.testApp,
			OtherApps:  cfg.otherApps,
		})
	}
	return xcuitest.Project{
		TypeDef: config.TypeDef{
			APIVersion: xcuitest.APIVersion,
			Kind:       xcuitest.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Concurrency: cfg.concurrency,
		},
		Xcuitest: xcuitest.Xcuitest{
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
