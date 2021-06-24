package init

import (
	"fmt"
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
			Concurrency: cfg.concurrency,
		},
		Xcuitest: xcuitest.Xcuitest{
			App:     cfg.app,
			TestApp: cfg.testApp,
		},
		Suites: []xcuitest.Suite{
			{
				Name:    fmt.Sprintf("xcuitest - %s", cfg.device.Name),
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
