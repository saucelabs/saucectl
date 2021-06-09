package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
)

func configureEspresso(ini initiator) error {
	err := ini.askRegion()
	if err != nil {
		return err
	}

	var app string
	err = ini.askFile("Application to test:", hasValidExt(".apk"), completeBasic, &app)
	if err != nil {
		return err
	}

	var testApp string
	err = ini.askFile("Application to test:", hasValidExt(".apk"), completeBasic, &testApp)
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
