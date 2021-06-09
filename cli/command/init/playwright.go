package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/playwright"
)

func configurePlaywright() error {
	var err error
	region, err := ask(regionSelector)
	if err != nil {
		return err
	}

	version, err := askVersion("playwright")
	if err != nil {
		return err
	}

	rootDir, err := askString("root dir", "", func(ans interface{}) error {
		return nil
	}, nil)
	if err != nil {
		return err
	}

	platformName, mode, browserName, err := askPlatform()
	if err != nil {
		return err
	}

	downloadConfig, err := askDownloadConfig()
	if err != nil {
		return err
	}

	/* build config file */
	cfg := playwright.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindPlaywright,
		},
		Sauce: config.SauceConfig{
			Region:      region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: rootDir,
		Playwright: playwright.Playwright{
			Version: version,
		},
		Suites: []playwright.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: platformName,
				Params: playwright.SuiteConfig{
					BrowserName: browserName,
				},
				Mode: mode,
			},
		},
		Artifacts: downloadConfig,
	}

	return saveConfiguration(cfg)
}
