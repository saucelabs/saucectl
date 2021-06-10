package init

import (
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
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: cfg.rootDir,
		Puppeteer: puppeteer.Puppeteer{
			Version: cfg.frameworkVersion,
		},
		Suites: []puppeteer.Suite{
			{
				Name:    "My First Suite", //TODO: Authorize to name you suite
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
