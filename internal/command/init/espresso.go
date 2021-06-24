package init

import (
	"fmt"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
)

func configureEspresso(cfg *initConfig) interface{} {
	return espresso.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindEspresso,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: cfg.concurrency,
		},
		Espresso: espresso.Espresso{
			App:     cfg.app,
			TestApp: cfg.testApp,
		},
		Suites: []espresso.Suite{
			{
				Name:      fmt.Sprintf("espresso - %s - %s", cfg.device.Name , cfg.emulator.Name),
				Devices:   []config.Device{cfg.device},
				Emulators: []config.Emulator{cfg.emulator},
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When: cfg.artifactWhen,
				Match: []string{"*"},
				Directory: "artifacts",
			},
		},
	}
}
