package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
)


func configureCypress(ini initiator) error {
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

	var cypressJson string
	err = ini.askFile("Cypress configuration file:", hasValidExt(".json"), completeBasic, &cypressJson)
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
	cfg := cypress.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindCypress,
		},
		Sauce: config.SauceConfig{
			Region:      ini.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: rootDir,
		Cypress: cypress.Cypress{
			Version:    ini.frameworkVersion,
			ConfigFile: cypressJson,
		},
		Suites: []cypress.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: ini.platformName,
				Browser:      ini.browserName,
				Mode:         ini.mode,
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
