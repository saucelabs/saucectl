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

func configureCypress() error {
	var err error
	region, err := ask(regionSelector)
	if err != nil {
		return err
	}

	version, err := askVersion("cypress")
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

	platformName, mode, browserName, err := askPlatform()
	if err != nil {
		return err
	}

	downloadConfig, err := askDownloadConfig()
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
			Region:      region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: rootDir,
		Cypress: cypress.Cypress{
			Version:    version,
			ConfigFile: cypressJson,
		},
		Suites: []cypress.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: platformName,
				Browser:      browserName,
				Mode:         mode,
			},
		},
		Artifacts: downloadConfig,
	}

	return saveConfiguration(cfg)
}
