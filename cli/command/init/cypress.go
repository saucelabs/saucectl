package init

import (
	// imports embed to load .sauceignore
	_ "embed"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
)

func configureCypress(cfg *initConfig) interface{} {
	return cypress.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindCypress,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: ".",
		Cypress: cypress.Cypress{
			Version:    cfg.frameworkVersion,
			ConfigFile: cfg.cypressJSON,
		},
		Suites: []cypress.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: cfg.platformName,
				Browser:      cfg.browserName,
				Mode:         cfg.mode,
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

//go:embed sauceignore/cypress.sauceignore
var sauceignoreCypress string
