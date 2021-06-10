package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

func configureXCUITest(ini initiator) interface{} {
	return xcuitest.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindXcuitest,
		},
		Sauce: config.SauceConfig{
			Region:      ini.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		Xcuitest: xcuitest.Xcuitest{
			App:     ini.app,
			TestApp: ini.testApp,
		},
		Suites: []xcuitest.Suite{
			{
				Name:    "My First Suite",
				Devices: []config.Device{ini.device},
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When:      ini.artifactWhen,
				Directory: "./artifacts",
				Match:     []string{"*"},
			},
		},
	}
}
