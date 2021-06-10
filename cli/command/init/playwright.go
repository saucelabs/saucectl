package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/playwright"
)

func configurePlaywright(ini initiator) interface{} {
	return playwright.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindPlaywright,
		},
		Sauce: config.SauceConfig{
			Region:      ini.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: ini.rootDir,
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
}
