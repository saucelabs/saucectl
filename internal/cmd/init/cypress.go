package init

import (
	// imports embed to load .sauceignore
	_ "embed"
	"fmt"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
)

func configureCypress(cfg *initConfig) interface{} {
	return cypress.Project{
		TypeDef: config.TypeDef{
			APIVersion: cypress.APIVersion,
			Kind:       cypress.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: cfg.concurrency,
		},
		RootDir: ".",
		Cypress: cypress.Cypress{
			Version:    cfg.frameworkVersion,
			ConfigFile: cfg.cypressJSON,
		},
		Suites: []cypress.Suite{
			{
				Name:         fmt.Sprintf("cypress - %s - %s", firstNotEmpty(cfg.platformName, cfg.mode) , cfg.browserName),
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
