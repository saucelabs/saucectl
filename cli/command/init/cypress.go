package init

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
)

func isAJSON(s interface{}) error {
	val := s.(string)
	if !strings.HasSuffix(val, ".json") {
		return fmt.Errorf("configuration must be a .json")
	}
	_, err := os.Stat(val)
	if err != nil {
		return fmt.Errorf("%s: %v", val, err)
	}
	return nil
}

func completeJSON(toComplete string) []string {
	files, _ := filepath.Glob(fmt.Sprintf("%s%s", toComplete, "*"))
	return files
}

func configureCypress(ini initiator) error {
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

	cypressJson, err := askString("Cypress configuration file", "", isAJSON, completeJSON)
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
