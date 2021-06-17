package init

import (
	// imports embed to load .sauceignore
	_ "embed"
	"fmt"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/puppeteer"
)

func configurePuppeteer(cfg *initConfig) interface{} {
	return puppeteer.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindPuppeteer,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: cfg.concurrency,
		},
		RootDir: ".",
		Puppeteer: puppeteer.Puppeteer{
			Version: cfg.frameworkVersion,
		},
		Suites: []puppeteer.Suite{
			{
				Name:    fmt.Sprintf("puppeteer - %s - %s", firstAvailable(cfg.platformName, cfg.mode), cfg.browserName),
				Browser: cfg.browserName,
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

//go:embed sauceignore/puppeteer.sauceignore
var sauceignorePuppeteer string
