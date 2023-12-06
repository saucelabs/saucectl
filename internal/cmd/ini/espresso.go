package ini

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func EspressoCmd() *cobra.Command {
	cfg := &initConfig{
		frameworkName: espresso.Kind,
	}

	cmd := &cobra.Command{
		Use:          "espresso",
		Short:        "Bootstrap an Espresso project.",
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

	cmd.Flags().StringVar(&cfg.app, "app", "", "Path to application under test.")
	cmd.Flags().StringVar(&cfg.testApp, "test-app", "", "Path to test application.")
	cmd.Flags().StringSliceVar(&cfg.otherApps, "other-apps", []string{}, "Path to additional applications.")
	cmd.Flags().StringVar(&cfg.artifactWhenStr, "artifacts-download-when", "fail", "When to download artifacts.")
	cmd.Flags().Var(&cfg.emulatorFlag, "emulator", "Android emulator to use for testing.")
	cmd.Flags().Var(&cfg.deviceFlag, "device", "Real device to use for testing.")
	return cmd
}

func configureEspresso(cfg *initConfig) interface{} {
	var devices []config.Device
	var emulators []config.Emulator

	if !noPrompt || cfg.emulatorFlag.Changed {
		emulators = append(emulators, cfg.emulator)
	}
	if !noPrompt || cfg.deviceFlag.Changed {
		devices = append(devices, cfg.device)
	}

	return espresso.Project{
		TypeDef: config.TypeDef{
			APIVersion: espresso.APIVersion,
			Kind:       espresso.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Concurrency: cfg.concurrency,
		},
		Espresso: espresso.Espresso{
			App:       cfg.app,
			TestApp:   cfg.testApp,
			OtherApps: cfg.otherApps,
		},
		Suites: []espresso.Suite{
			{
				Name:      fmt.Sprintf("espresso - %s - %s", cfg.device.Name, cfg.emulator.Name),
				Devices:   devices,
				Emulators: emulators,
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When:      cfg.artifactWhen,
				Match:     []string{"*"},
				Directory: "artifacts",
			},
		},
	}
}
