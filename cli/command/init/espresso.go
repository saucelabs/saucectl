package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
)

func configureEspresso(ini initiator) interface{} {
	return espresso.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindEspresso,
		},
		Sauce: config.SauceConfig{
			Region:      ini.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		Espresso: espresso.Espresso{
			App:     ini.app,
			TestApp: ini.testApp,
		},
		Suites: []espresso.Suite{
			{
				//TODO: Authorize to name you suite
				Name:      "My First Suite",
				// TODO: Check before adding element
				Devices:   []config.Device{ini.device},
				Emulators: []config.Emulator{ini.emulator},
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When: ini.artifactWhen,
				Match: []string{"*"},
				Directory: "artifacts",
			},
		},
	}
}
