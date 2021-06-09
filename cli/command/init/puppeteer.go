package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/puppeteer"
)

func configurePuppeteer(ini initiator) error {
	err := ini.askRegion()
	if err != nil {
		return err
	}

	err = ini.askVersion()
	if err != nil {
		return err
	}

	var rootDir string
	err = ini.askFile("Root project directory:", isDirectory, nil, &rootDir)
	if err != nil {
		return err
	}

	err = ini.askPlatform()
	if err != nil {
		return err
	}

	err = ini.askDownloadWhen()
	if err != nil {
		return err
	}

	/* build config file */
	cfg := puppeteer.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindPuppeteer,
		},
		Sauce: config.SauceConfig{
			Region:      ini.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: rootDir,
		Puppeteer: puppeteer.Puppeteer{
			Version: ini.frameworkVersion,
		},
		Suites: []puppeteer.Suite{
			{
				Name:    "My First Suite", //TODO: Authorize to name you suite
				Browser: ini.browserName,
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

	return saveConfiguration(cfg)
}
