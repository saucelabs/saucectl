package init

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/region"
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

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), cfg.frameworkName)
	if err != nil {
		return &initConfig{}, err
	}

	if needsVersion(cfg.frameworkName) {
		err = ini.askVersion(cfg, frameworkMetadatas)
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
		err = ini.askPlatform(cfg, frameworkMetadatas)
		if err != nil {
			return &initConfig{}, err
		}
	}

	if needsApps(cfg.frameworkName) {
		err = ini.askFile("Application to test:", extValidator(cfg.frameworkName), completeBasic, &cfg.app)
		if err != nil {
			return &initConfig{}, err
		}

		err = ini.askFile("Test application:", extValidator(cfg.frameworkName), completeBasic, &cfg.testApp)
		if err != nil {
			return &initConfig{}, err
		}
	}

	if needsDevice(cfg.frameworkName) {
		osName := "ANDROID"
		if cfg.frameworkName == config.KindXcuitest {
			osName = "IOS"
		}
		devices, err := ini.deviceReader.GetDevices(context.Background(), osName)
		if err != nil {
			return &initConfig{}, err
		}

		err = ini.askDevice(cfg, devices)
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
	values, err := ini.infoReader.Frameworks(context.Background())
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

func (ini *initiator) askDevice(cfg *initConfig, devs []devices.Device) error {
	var deviceNames []string
	for _, d := range devs {
		deviceNames = append(deviceNames, d.Name)
	}
	q := &survey.Select{
		Message: "Select device:",
		Options: deviceNames,
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

func metaToVersions(metadatas []framework.Metadata) []string {
	var versions []string
	for _, v := range metadatas {
		versions = append(versions, v.FrameworkVersion)
	}
	return versions
}

func metaToPlatforms(metadatas []framework.Metadata, version string) []string {
	var platforms []string
	for _, m := range metadatas {
		if m.FrameworkVersion == version {
			for _, p := range m.Platforms {
				platforms = append(platforms, p.PlatformName)
			}
		}
		if m.DockerImage != "" {
			platforms = append(platforms, "docker")
		}
	}
	return platforms
}

func metaToBrowsers(metadatas []framework.Metadata, frameworkName, frameworkVersion, platformName string) []string {
	if platformName == "docker" {
		return dockerBrowsers(frameworkName)
	}

	// It's not optimum to have double iteration, but since the set it pretty small this will be insignificant.
	// It's helping for readability.
	for _, v := range metadatas {
		for _, p := range v.Platforms {
			if v.FrameworkVersion == frameworkVersion && p.PlatformName == platformName {
				return p.BrowserNames
			}
		}
	}
	return []string{}
}


func dockerBrowsers(framework string) []string {
	switch framework {
	case "playwright":
		return []string{"chromium", "firefox"}
	default:
		return []string{"chrome", "firefox"}
	}
}

func (ini *initiator) askPlatform(cfg *initConfig, metadatas []framework.Metadata) error {
	platformChoices := metaToPlatforms(metadatas, cfg.frameworkVersion)

	q := &survey.Select{
		Message: "Select platform:",
		Options: platformChoices,
	}
	err := survey.AskOne(q, &cfg.platformName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}

	// Select browser
	browserChoices := metaToBrowsers(metadatas, cfg.frameworkName, cfg.frameworkVersion, cfg.platformName)
	q = &survey.Select{
		Message: "Select Browser:",
		Options: browserChoices,
	}
	err = survey.AskOne(q, &cfg.browserName,
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
	return nil
}

func (ini *initiator) askVersion(cfg *initConfig, metadatas []framework.Metadata) error {
	versions := metaToVersions(metadatas)

	q := &survey.Select{
		Message: fmt.Sprintf("Select %s version:", cfg.frameworkName),
		Options: versions,
	}

	err := survey.AskOne(q, &cfg.frameworkVersion,
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
