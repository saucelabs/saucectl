package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/puppeteer"
)

func configurePuppeteer() error {
	var err error
	region, err := ask(regionSelector)
	if err != nil {
		return err
	}

	version, err := askVersion("puppeteer")
	if err != nil {
		return err
	}

	rootDir, err := askString("root dir", "", func(ans interface{}) error {
		return nil
	}, nil)
	if err != nil {
		return err
	}

	_, _, browserName, err := askPlatform()
	if err != nil {
		return err
	}

	downloadConfig, err := askDownloadConfig()
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
			Region:      region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: rootDir,
		Puppeteer: puppeteer.Puppeteer{
			Version: version,
		},
		Suites: []puppeteer.Suite{
			{
				Name:    "My First Suite", //TODO: Authorize to name you suite
				Browser: browserName,
			},
		},
		Artifacts: downloadConfig,
	}

	return saveConfiguration(cfg)
}
