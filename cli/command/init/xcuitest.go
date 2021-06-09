package init

import (
	"fmt"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"os"
	"path/filepath"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
)

func isAnIPAOrApp(s interface{}) error {
	val := s.(string)
	if !strings.HasSuffix(val, ".ipa") && !strings.HasSuffix(val, ".app") {
		return fmt.Errorf("application must be an .ipa or .apk")
	}
	_, err := os.Stat(val)
	if err != nil {
		return fmt.Errorf("%s: %v", val, err)
	}
	return nil
}

func completeIPA(toComplete string) []string {
	files, _ := filepath.Glob(fmt.Sprintf("%s%s", toComplete, "*"))
	return files
}

func configureXCUITest() error {
	var err error
	region, err := ask(regionSelector)
	if err != nil {
		return err
	}

	app, err := askString("Application to test", "", isAnIPAOrApp, completeIPA)
	if err != nil {
		return err
	}

	testApp, err := askString("Test application", "", isAnIPAOrApp, completeIPA)
	if err != nil {
		return err
	}

	device, err := askDevice()
	if err != nil {
		return err
	}

	downloadConfig, err := askDownloadConfig()
	if err != nil {
		return err
	}

	/* build config file */
	cfg := xcuitest.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindXcuitest,
		},
		Sauce: config.SauceConfig{
			Region:      region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		Xcuitest: xcuitest.Xcuitest{
			App:     app,
			TestApp: testApp,
		},
		Suites: []xcuitest.Suite{
			{
				Name:      "My First Suite",
				Devices:   []config.Device{device},
			},
		},
		Artifacts: downloadConfig,
	}
	return saveConfiguration(cfg)
}
