package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/testcafe"
)

func configureTestcafe() error {
	var err error
	region, err := ask(regionSelector)
	if err != nil {
		return err
	}

	version, err := askVersion("testcafe")
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
	cfg := testcafe.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindTestcafe,
		},
		Sauce: config.SauceConfig{
			Region:      region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: rootDir,
		Testcafe: testcafe.Testcafe{
			Version: version,
		},
		Suites: []testcafe.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: platformName,
				BrowserName: browserName,
				Mode: mode,
			},
		},
		Artifacts: downloadConfig,
	}

	return saveConfiguration(cfg)
}
