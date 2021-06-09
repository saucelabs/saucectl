package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

func configureXCUITest(ini initiator) error {
	err := ini.askRegion()
	if err != nil {
		return err
	}

	var app string
	err = ini.askFile("Application to test:", hasValidExt(".app", ".ipa"), completeBasic, &app)
	if err != nil {
		return err
	}

	var testApp string
	err = ini.askFile("Test application:", hasValidExt(".app", ".ipa"), completeBasic, &testApp)
	if err != nil {
		return err
	}

	err = ini.askDevice()
	if err != nil {
		return err
	}

	err = ini.askDownloadWhen()
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
			Region:      ini.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		Xcuitest: xcuitest.Xcuitest{
			App:     app,
			TestApp: testApp,
		},
		Suites: []xcuitest.Suite{
			{
				Name:    "My First Suite",
				Devices: []config.Device{ini.device},
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When:      ini.artifactWhen,
				Directory: "./artifacts",
				Match:     []string{"*"},
			},
		},
	}
	return saveConfiguration(cfg)
}
