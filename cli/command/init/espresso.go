package init

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
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

func configureEspresso(ini initiator) error {
	err := ini.askRegion()
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

	err = ini.askDevice()
	if err != nil {
		return err
	}

	err = ini.askEmulator()
	if err != nil {
		return err
	}

	err = ini.askDownloadWhen()
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
			Region:      ini.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		Espresso: espresso.Espresso{
			App:     app,
			TestApp: testApp,
		},
		Suites: []espresso.Suite{
			{
				//TODO: Authorize to name you suite
				Name:      "My First Suite",
				// TODO: Check before adding element
				Devices:   []config.Device{ini.device},
				Emulators: []config.Emulator{ini.emulator},
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When: ini.artifactWhen,
				Match: []string{"*"},
				Directory: "artifacts",
			},
		},
	}

	return saveConfiguration(cfg)
}
