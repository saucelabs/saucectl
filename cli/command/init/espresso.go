package init

import (
	"fmt"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"os"
	"path/filepath"
	"strings"
)

func isAnAPK(s interface{}) error {
	val := s.(string)
	if !strings.HasSuffix(val, ".apk") {
		return fmt.Errorf("application must be an .apk")
	}
	_, err := os.Stat(val)
	if err != nil {
		return fmt.Errorf("%s: %v", val, err)
	}
	return nil
}

func completeAPK(toComplete string) []string {
	files, _ := filepath.Glob(fmt.Sprintf("%s%s", toComplete, "*"))
	return files
}

func configureEspresso() error {
	var err error
	region, err := ask(regionSelector)
	if err != nil {
		return err
	}

	app, err := askString("Application to test", "", isAnAPK, completeAPK)
	if err != nil {
		return err
	}

	testApp, err := askString("Test application", "", isAnAPK, completeAPK)
	if err != nil {
		return err
	}

	device, err := askDevice()
	if err != nil {
		return err
	}

	emulator, err := askEmulator()
	if err != nil {
		return err
	}
	downloadConfig, err := askDownloadConfig()
	if err != nil {
		return err
	}

	/* build config file */
	cfg := espresso.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindEspresso,
		},
		Sauce: config.SauceConfig{
			Region:      region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		Espresso: espresso.Espresso{
			App:     app,
			TestApp: testApp,
		},
		Suites: []espresso.Suite{
			{
				Name:      "My First Suite", //TODO: Authorize to name you suite
				Devices:   []config.Device{device},
				Emulators: []config.Emulator{emulator},
			},
		},
		Artifacts: downloadConfig,
	}

	return saveConfiguration(cfg)
}
