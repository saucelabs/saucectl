package init

import (
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
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		Espresso: espresso.Espresso{
			App:     cfg.app,
			TestApp: cfg.testApp,
		},
		Suites: []espresso.Suite{
			{
				//TODO: Authorize to name you suite
				Name:      "My First Suite",
				// TODO: Check before adding element
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
