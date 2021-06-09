package init

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/saucelabs/saucectl/internal/config"
)

var regionSelector = &survey.Select{
	Message: "Choose the sauce labs region:",
	Options: []string{"us-west-1", "eu-central-1"},
	Default: "us-west-1",
}

var frameworkSelector = &survey.Select{
	Message: "Choose your framework:",
	Options: []string{"Cypress", "Espresso", "Playwright", "Puppeteer", "TestCafe", "XCUITest"},
	Default: "Cypress",
}

func ask(p survey.Prompt) (string, error) {
	var value string
	err := survey.AskOne(p, &value)
	if value == "" {
		return "", errors.New("interrupting configuration")
	}
	return value, err
}

type completor func(string) []string

func askString(message string, def string, val survey.Validator, comp completor) (string, error) {
	q := &survey.Input{
		Message: fmt.Sprintf("%s:", message),
		Default: def,
		Suggest: comp,
	}

	var appPath string
	if err := survey.AskOne(q, &appPath,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithValidator(val)); err != nil {
		return "", err
	}
	return appPath, nil
}

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

func askDownloadWhen() (config.When, error) {
	q := &survey.Select{
		Message: "Download artifacts",
		Default: whenStrings[0],
		Options: whenStrings,
	}
	var when string
	err := survey.AskOne(q, &when,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required))
	if err != nil {
		return "", nil
	}
	return mapWhen[when], nil
}

func askYesNo(question string, def bool) (bool, error) {
	q := &survey.Confirm{
		Message: question,
		Default: def,
	}
	var resp bool
	err := survey.AskOne(q, &resp)
	return resp, err
}

func askDevice() (config.Device, error) {
	// TODO: Check if device exists !
	deviceName, err := askString("Type device name", "", survey.Required, nil)
	if err != nil {
		return config.Device{}, err
	}
	return config.Device{
		Name: deviceName,
	}, nil
}

func askEmulator() (config.Emulator, error) {
	// TODO: Propose selection of emulators !
	emulatorName, err := askString("Type emulator name", "", survey.Required, nil)
	if err != nil {
		return config.Emulator{}, err
	}
	return config.Emulator{
		Name: emulatorName,
	}, nil
}

func askDownloadConfig() (config.Artifacts, error) {
	when, err := askDownloadWhen()
	if err != nil {
		return config.Artifacts{}, err
	}

	return config.Artifacts{
		Download: config.ArtifactDownload{
			Directory: "./artifacts/",
			When:      when,
			Match:     []string{"*"},
		},
	}, nil
}

func askPlatform() (platformName string, mode string, browserName string, err error) {
	mode = "sauce"

	// FIXME: Display only supported platforms
	q := &survey.Select{
		Message: "Choose platform",
		Default: "Windows 10",
		Options: []string{"Windows 10", "docker", "macOS 11.0"},
	}
	err = survey.AskOne(q, &platformName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required))
	if err != nil {
		return "", "", "", err
	}
	if platformName == "docker" {
		platformName = ""
		mode = "docker"
	}

	// FIXME: Display only supported browsers
	q = &survey.Select{
		Message: "Choose Browser",
		Default: "chrome",
		Options: []string{"chrome", "firefox", "webkit"},
	}
	err = survey.AskOne(q, &browserName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required))
	if err != nil {
		return "", "", "", err
	}
	return
}

func askVersion(framework string) (string, error) {
	return "7.2.0", nil
}