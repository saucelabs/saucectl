package init

import (
	// imports embed to load .sauceignore
	_ "embed"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/testcafe"
)

func configureTestcafe(cfg *initConfig) interface{} {
	return testcafe.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindTestcafe,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: ".",
		Testcafe: testcafe.Testcafe{
			Version: cfg.frameworkVersion,
		},
		Suites: []testcafe.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: cfg.platformName,
				BrowserName:  cfg.browserName,
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

//go:embed sauceignore/testcafe.sauceignore
var sauceignoreTestcafe string
