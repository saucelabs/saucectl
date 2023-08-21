package ini

import (
	"fmt"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

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
