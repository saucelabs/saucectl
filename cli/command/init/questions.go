package init

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/region"
	"strings"
)

// Check routines
func needsCredentials() bool {
	return false
}

func isNativeFramework(framework string) bool {
	return framework == config.KindEspresso || framework == config.KindXcuitest
}

func needsApps(framework string) bool {
	return isNativeFramework(framework)
}

func needsCypressJson(framework string) bool {
	return framework == config.KindCypress
}

func needsDevice(framework string) bool {
	return isNativeFramework(framework)
}

func needsEmulator(framework string) bool {
	return framework == config.KindEspresso
}

func needsPlatform(framework string) bool {
	return !isNativeFramework(framework)
}

func needsRootDir(framework string) bool {
	return !isNativeFramework(framework)
}

func needsVersion(framework string) bool {
	return !isNativeFramework(framework)
}

func (ini *initiator) configure() (*initConfig, error) {
	cfg := &initConfig{}

	err := ini.askFramework(cfg)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askRegion(cfg)
	if err != nil {
		return &initConfig{}, err
	}

	if needsCredentials() {
		// TODO: Implement
	}

	if needsVersion(cfg.frameworkName) {
		err = ini.askVersion(cfg)
		if err != nil {
			return &initConfig{}, err
		}
	}

	if needsRootDir(cfg.frameworkName) {
		err = ini.askFile("Root project directory:", isDirectory, nil, &cfg.rootDir)
		if err != nil {
			return &initConfig{}, err
		}
	}

	if needsCypressJson(cfg.frameworkName) {
		err = ini.askFile("Cypress configuration file:", extValidator(cfg.frameworkName), completeBasic, &cfg.cypressJson)
		if err != nil {
			return &initConfig{}, err
		}
	}

	if needsPlatform(cfg.frameworkName) {
		err = ini.askPlatform(cfg)
		if err != nil {
			return &initConfig{}, err
		}
	}

	if needsApps(cfg.frameworkName) {
		err = ini.askFile("Application to test:", extValidator(cfg.frameworkName), completeBasic, &cfg.app)
		if err != nil {
			return &initConfig{}, err
		}

		err = ini.askFile("Application to test:", extValidator(cfg.frameworkName), completeBasic, &cfg.testApp)
		if err != nil {
			return &initConfig{}, err
		}
	}

	if needsDevice(cfg.frameworkName) {
		err = ini.askDevice(cfg)
		if err != nil {
			return &initConfig{}, err
		}

	}

	if needsEmulator(cfg.frameworkName) {
		err = ini.askEmulator(cfg)
		if err != nil {
			return &initConfig{}, err
		}
	}

	err = ini.askDownloadWhen(cfg)
	if err != nil {
		return &initConfig{}, err
	}
	return cfg, nil
}


func (ini *initiator) askRegion(cfg *initConfig) error {
	p := &survey.Select{
		Message: "Select region:",
		Options: []string{region.USWest1.String(), region.EUCentral1.String()},
		Default: region.USWest1.String(),
	}


	err := survey.AskOne(p, &cfg.region, survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initiator) askFramework(cfg *initConfig) error {
	values, err := ini.infoReader.Frameworks()
	if err != nil {
		return err
	}

	p := &survey.Select{
		Message: "Select framework:",
		Options: values,
	}

	err = survey.AskOne(p, &cfg.frameworkName, survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if cfg.frameworkName == "" {
		return errors.New("interrupting configuration")
	}
	cfg.frameworkName = strings.ToLower(cfg.frameworkName)
	return err
}

type completor func(string) []string

/* When translation */
var whenStrings = []string{
	"when tests are failing",
	"when tests are passing",
	"never",
	"always",
}
var mapWhen = map[string]config.When{
	"when tests are failing": config.WhenFail,
	"when tests are passing": config.WhenPass,
	"never":                  config.WhenNever,
	"always":                 config.WhenAlways,
}

func (ini *initiator) askDownloadWhen(cfg *initConfig) error {
	q := &survey.Select{
		Message: "Download artifacts:",
		Default: whenStrings[0],
		Options: whenStrings,
	}
	q.WithStdio(ini.stdio)

	var when string
	err := survey.AskOne(q, &when,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	cfg.artifactWhen = mapWhen[when]
	return nil
}


func (ini *initiator) askDevice(cfg *initConfig) error {
	// TODO: Check if device exists !
	q := &survey.Input{
		Message: "Type device name:",
	}
	err := survey.AskOne(q, &cfg.device.Name,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initiator) askEmulator(cfg *initConfig) error {
	// TODO: Propose selection of emulators !
	q := &survey.Input{
		Message: "Type emulator name:",
	}
	err := survey.AskOne(q, &cfg.emulator.Name,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initiator) askPlatform(cfg *initConfig) error {
	// Select Platform
	platforms, _ := ini.infoReader.Platforms(cfg.frameworkName, cfg.region, cfg.frameworkVersion)
	q := &survey.Select{
		Message: "Select platform:",
		Options: platforms,
	}
	err := survey.AskOne(q, &cfg.platformName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}

	cfg.mode = "sauce"
	if cfg.platformName == "docker" {
		cfg.platformName = ""
		cfg.mode = "docker"
	}

	// Select browser
	browsers, _ := ini.infoReader.Browsers(cfg.frameworkName, cfg.region, cfg.frameworkVersion, cfg.platformName)
	q = &survey.Select{
		Message: "Select Browser:",
		Options: browsers,
	}
	err = survey.AskOne(q, &cfg.browserName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initiator) askVersion(cfg *initConfig) error {
	versions, err := ini.infoReader.Versions(cfg.frameworkName, cfg.region)
	if err != nil {
		return err
	}
	q := &survey.Select{
		Message: fmt.Sprintf("Select %s version:", cfg.frameworkName),
		Options: versions,
	}

	err = survey.AskOne(q, &cfg.frameworkVersion,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	return nil
}


func (ini *initiator) askFile(message string, val survey.Validator, comp completor, targetValue *string) error {
	q := &survey.Input{
		Message: message,
		Suggest: comp,
	}

	if err := survey.AskOne(q, targetValue,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithValidator(val),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err)); err != nil {
		return err
	}
	return nil
}