package init

import (
	// imports embed to load .sauceignore
	_ "embed"
	"fmt"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/playwright"
)

func configurePlaywright(cfg *initConfig) interface{} {
	return playwright.Project{
		TypeDef: config.TypeDef{
			APIVersion: playwright.APIVersion,
			Kind:       playwright.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: cfg.concurrency,
		},
		RootDir: ".",
		Playwright: playwright.Playwright{
			Version: cfg.frameworkVersion,
		},
		Suites: []playwright.Suite{
			{
				Name:         fmt.Sprintf("playwright - %s - %s", firstNotEmpty(cfg.platformName, cfg.mode), cfg.browserName),
				PlatformName: cfg.platformName,
				Params: playwright.SuiteConfig{
					BrowserName: cfg.browserName,
				},
				Mode: cfg.mode,
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

//go:embed sauceignore/playwright.sauceignore
var sauceignorePlaywright string
