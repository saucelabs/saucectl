package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

func configureXCUITest(cfg *initConfig) interface{} {
	return xcuitest.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindXcuitest,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		Xcuitest: xcuitest.Xcuitest{
			App:     cfg.app,
			TestApp: cfg.testApp,
		},
		Suites: []xcuitest.Suite{
			{
				Name:    "My First Suite",
				Devices: []config.Device{cfg.device},
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
