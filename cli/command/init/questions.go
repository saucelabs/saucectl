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

func (ini *initiator) configure() error {
	err := ini.askFramework()
	if err != nil {
		return err
	}

	err = ini.askRegion()
	if err != nil {
		return err
	}

	if needsCredentials() {
		// TODO: Implement
	}

	if needsVersion(ini.frameworkName) {
		err = ini.askVersion()
		if err != nil {
			return err
		}
	}

	if needsRootDir(ini.frameworkName) {
		err = ini.askFile("Root project directory:", isDirectory, nil, &ini.rootDir)
		if err != nil {
			return err
		}
	}

	if needsCypressJson(ini.frameworkName) {
		err = ini.askFile("Cypress configuration file:", extValidator(ini.frameworkName), completeBasic, &ini.cypressJson)
		if err != nil {
			return err
		}
	}

	if needsPlatform(ini.frameworkName) {
		err = ini.askPlatform()
		if err != nil {
			return err
		}
	}

	if needsApps(ini.frameworkName) {
		err = ini.askFile("Application to test:", extValidator(ini.frameworkName), completeBasic, &ini.app)
		if err != nil {
			return err
		}

		err = ini.askFile("Application to test:", extValidator(ini.frameworkName), completeBasic, &ini.testApp)
		if err != nil {
			return err
		}
	}

	if needsDevice(ini.frameworkName) {
		err = ini.askDevice()
		if err != nil {
			return err
		}

	}

	if needsEmulator(ini.frameworkName) {
		err = ini.askEmulator()
		if err != nil {
			return err
		}
	}

	err = ini.askDownloadWhen()
	if err != nil {
		return err
	}
	return nil
}


func (ini *initiator) askRegion() error {
	p := &survey.Select{
		Message: "Select region:",
		Options: []string{region.USWest1.String(), region.EUCentral1.String()},
		Default: region.USWest1.String(),
	}


	err := survey.AskOne(p, &ini.region, survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initiator) askFramework() error {
	values, err := ini.infoReader.Frameworks()
	if err != nil {
		return err
	}

	p := &survey.Select{
		Message: "Select framework:",
		Options: values,
	}

	err = survey.AskOne(p, &ini.frameworkName, survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if ini.frameworkName == "" {
		return errors.New("interrupting configuration")
	}
	ini.frameworkName = strings.ToLower(ini.frameworkName)
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

func (ini *initiator) askDownloadWhen() error {
	q := &survey.Select{
		Message: "Download artifacts:",
		Default: whenStrings[0],
		Options: whenStrings,
	}
	var when string
	err := survey.AskOne(q, &when,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required))
	if err != nil {
		return err
	}
	ini.artifactWhen = mapWhen[when]
	return nil
}


func (ini *initiator) askDevice() error {
	// TODO: Check if device exists !
	q := &survey.Input{
		Message: "Type device name:",
	}
	err := survey.AskOne(q, &ini.device.Name, survey.WithShowCursor(true))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initiator) askEmulator() error {
	// TODO: Propose selection of emulators !
	q := &survey.Input{
		Message: "Type emulator name:",
	}
	err := survey.AskOne(q, &ini.emulator.Name, survey.WithShowCursor(true))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initiator) askPlatform() error {
	// Select Platform
	platforms, _ := ini.infoReader.Platforms(ini.frameworkName, ini.region, ini.frameworkVersion)
	q := &survey.Select{
		Message: "Select platform:",
		Options: platforms,
	}
	err := survey.AskOne(q, &ini.platformName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required))
	if err != nil {
		return err
	}

	ini.mode = "sauce"
	if ini.platformName == "docker" {
		ini.platformName = ""
		ini.mode = "docker"
	}

	// Select browser
	browsers, _ := ini.infoReader.Browsers(ini.frameworkName, ini.region, ini.frameworkVersion, ini.platformName)
	q = &survey.Select{
		Message: "Select Browser:",
		Options: browsers,
	}
	err = survey.AskOne(q, &ini.browserName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initiator) askVersion() error {
	versions, err := ini.infoReader.Versions(ini.frameworkName, ini.region)
	if err != nil {
		return err
	}
	q := &survey.Select{
		Message: fmt.Sprintf("Select %s version:", ini.frameworkName),
		Options: versions,
	}

	err = survey.AskOne(q, &ini.frameworkVersion,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required))
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
		survey.WithValidator(val)); err != nil {
		return err
	}
	return nil
}