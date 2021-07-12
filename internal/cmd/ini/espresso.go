package ini

import (
	"fmt"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
)

func configureEspresso(cfg *initConfig) interface{} {
	var devices []config.Device
	var emulators []config.Emulator

	if !cfg.batchMode || cfg.emulatorFlag.Changed {
		emulators = append(emulators, cfg.emulator)
	}
	if !cfg.batchMode || cfg.deviceFlag.Changed {
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
