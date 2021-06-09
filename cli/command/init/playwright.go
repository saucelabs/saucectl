package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/playwright"
)

func configurePlaywright(ini initiator) error {
	err := ini.askRegion()
	if err != nil {
		return err
	}

	err = ini.askVersion()
	if err != nil {
		return err
	}

	rootDir, err := askString("root dir", "", func(ans interface{}) error {
		return nil
	}, nil)
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
	cfg := playwright.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindPlaywright,
		},
		Sauce: config.SauceConfig{
			Region:      ini.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: rootDir,
		Playwright: playwright.Playwright{
			Version: ini.frameworkVersion,
		},
		Suites: []playwright.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: ini.platformName,
				Params: playwright.SuiteConfig{
					BrowserName: ini.browserName,
				},
				Mode: ini.mode,
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When: ini.artifactWhen,
				Directory: "./artifacts",
				Match: []string{"*"},
			},
		},
	}

	return saveConfiguration(cfg)
}
