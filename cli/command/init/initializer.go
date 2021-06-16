package init

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/AlecAivazis/survey/v2/terminal"

	"github.com/AlecAivazis/survey/v2"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/vmd"
)

var androidDevicesPatterns = []string{
	"Amazon Kindle Fire .*", "Google Pixel .*", "HTC .*", "Huawei .*",
	"LG .*", "Motorola .*", "OnePlus .*", "Samsung .*", "Sony .*",
}

var iOSDevicesPatterns = []string{"iPad .*", "iPhone .*"}

type initializer struct {
	stdio        terminal.Stdio
	infoReader   framework.MetadataService
	deviceReader devices.Reader
	vmdReader    vmd.Reader

	frameworks        []string
	frameworkMetadata []framework.Metadata
}

// newInitializer creates a new initializer instance.
func newInitializer(stdio terminal.Stdio, creds credentials.Credentials, regio string) *initializer {
	r := region.FromString(regio)
	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         r.APIBaseURL(),
		Credentials: creds,
	}

	rc := rdc.Client{
		HTTPClient: &http.Client{Timeout: rdcTimeout},
		URL:        r.APIBaseURL(),
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	rs := resto.Client{
		HTTPClient: &http.Client{Timeout: restoTimeout},
		URL:        r.APIBaseURL(),
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	return &initializer{
		stdio:        stdio,
		infoReader:   &tc,
		deviceReader: &rc,
		vmdReader:    &rs,
	}
}

func (ini *initializer) configure() (*initConfig, error) {
	fName, err := ini.askFramework()
	if err != nil {
		return &initConfig{}, err
	}

	switch fName {
	case config.KindCypress:
		return ini.initializeCypress()
	case config.KindPlaywright:
		return ini.initializePlaywright()
	case config.KindPuppeteer:
		return ini.initializePuppeteer()
	case config.KindTestcafe:
		return ini.initializeTestcafe()
	case config.KindEspresso:
		return ini.initializeEspresso()
	case config.KindXcuitest:
		return ini.initializeXCUITest()
	default:
		return &initConfig{}, fmt.Errorf("unsupported framework %v", frameworkName)
	}
}

func askCredentials(stdio terminal.Stdio) (credentials.Credentials, error) {
	creds := credentials.Credentials{}
	q := &survey.Input{Message: "SauceLabs username:"}

	err := survey.AskOne(q, &creds.Username,
		survey.WithValidator(survey.Required),
		survey.WithShowCursor(true),
		survey.WithStdio(stdio.In, stdio.Out, stdio.Err))
	if err != nil {
		return creds, err
	}

	q = &survey.Input{Message: "SauceLabs access key:"}
	err = survey.AskOne(q, &creds.AccessKey,
		survey.WithValidator(survey.Required),
		survey.WithShowCursor(true),
		survey.WithStdio(stdio.In, stdio.Out, stdio.Err))
	if err != nil {
		return creds, err
	}
	return creds, nil
}

func askRegion(stdio terminal.Stdio) (string, error) {
	var r string
	p := &survey.Select{
		Message: "Select region:",
		Options: []string{region.USWest1.String(), region.EUCentral1.String()},
		Default: region.USWest1.String(),
	}

	err := survey.AskOne(p, &r, survey.WithStdio(stdio.In, stdio.Out, stdio.Err))
	if err != nil {
		return "", err
	}
	return r, nil
}

func (ini *initializer) askFramework() (string, error) {
	values, err := ini.infoReader.Frameworks(context.Background())
	if err != nil {
		return "", err
	}

	var frameworks []string
	for _, f := range values {
		frameworks = append(frameworks, f.Name)
	}

	p := &survey.Select{
		Message: "Select framework:",
		Options: frameworks,
	}

	var selectedFramework string
	err = survey.AskOne(p, &selectedFramework, survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if selectedFramework == "" {
		return "", errors.New("interrupting configuration")
	}
	return strings.ToLower(selectedFramework), err
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

func (ini *initializer) askDownloadWhen(cfg *initConfig) error {
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

func (ini *initializer) askDevice(cfg *initConfig, suggestions []string) error {
	q := &survey.Select{
		Message: "Select device pattern:",
		Options: suggestions,
	}
	return survey.AskOne(q, &cfg.device.Name,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
}

func (ini *initializer) askEmulator(cfg *initConfig, vmds []vmd.VirtualDevice) error {
	var vmdNames []string
	for _, v := range vmds {
		vmdNames = append(vmdNames, v.Name)
	}
	q := &survey.Select{
		Message: "Select emulator:",
		Options: uniqSorted(vmdNames),
	}
	return survey.AskOne(q, &cfg.emulator.Name,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
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
			if m.DockerImage != "" {
				platforms = append(platforms, "docker")
			}
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
				return correctBrowsers(frameworkName, p.BrowserNames)
			}
		}
	}
	return []string{}
}

func correctBrowsers(frameworkName string, browsers []string) []string {
	if frameworkName != config.KindPlaywright {
		return browsers
	}
	var cb []string
	for _, b := range browsers {
		cb = append(cb, strings.TrimPrefix(b, "playwright-"))
	}
	return cb
}

func dockerBrowsers(framework string) []string {
	switch framework {
	case "playwright":
		return []string{"chromium", "firefox"}
	default:
		return []string{"chrome", "firefox"}
	}
}

func (ini *initializer) askPlatform(cfg *initConfig, metadatas []framework.Metadata) error {
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

func (ini *initializer) askVersion(cfg *initConfig, metadatas []framework.Metadata) error {
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

func (ini *initializer) askFile(message string, val survey.Validator, comp completor, targetValue *string) error {
	q := &survey.Input{
		Message: message,
		Suggest: comp,
	}

	return survey.AskOne(q, targetValue,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithValidator(val),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
}

func (ini *initializer) initializeCypress() (*initConfig, error) {
	cfg := &initConfig{frameworkName: config.KindCypress}

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), cfg.frameworkName)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askVersion(cfg, frameworkMetadatas)
	if err != nil {
			return &initConfig{}, err
		}

	err = ini.askFile("Cypress configuration file:", extValidator(cfg.frameworkName), completeBasic, &cfg.cypressJSON)
	if err != nil {
			return &initConfig{}, err
		}

	err = ini.askPlatform(cfg, frameworkMetadatas)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askDownloadWhen(cfg)
	if err != nil {
		return &initConfig{}, err
	}
	return cfg, nil
}

func (ini *initializer) initializePlaywright() (*initConfig, error) {
	cfg := &initConfig{frameworkName: config.KindPlaywright}

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), cfg.frameworkName)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askVersion(cfg, frameworkMetadatas)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askPlatform(cfg, frameworkMetadatas)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askDownloadWhen(cfg)
	if err != nil {
		return &initConfig{}, err
	}
	return cfg, nil
}

func (ini *initializer) initializeTestcafe() (*initConfig, error) {
	cfg := &initConfig{frameworkName: config.KindTestcafe}

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), cfg.frameworkName)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askVersion(cfg, frameworkMetadatas)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askPlatform(cfg, frameworkMetadatas)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askDownloadWhen(cfg)
	if err != nil {
		return &initConfig{}, err
	}
	return cfg, nil
}

func (ini *initializer) initializePuppeteer() (*initConfig, error) {
	cfg := &initConfig{frameworkName: config.KindPuppeteer}

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), cfg.frameworkName)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askVersion(cfg, frameworkMetadatas)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askPlatform(cfg, frameworkMetadatas)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askDownloadWhen(cfg)
	if err != nil {
		return &initConfig{}, err
	}
	return cfg, nil
}

func (ini *initializer) initializeEspresso() (*initConfig, error) {
	cfg := &initConfig{frameworkName: config.KindEspresso}

	err := ini.askFile("Application to test:", extValidator(cfg.frameworkName), completeBasic, &cfg.app)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askFile("Test application:", extValidator(cfg.frameworkName), completeBasic, &cfg.testApp)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askDevice(cfg, androidDevicesPatterns)
	if err != nil {
		return &initConfig{}, err
	}

	virtualDevices, err := ini.vmdReader.GetVirtualDevices(context.Background(), vmd.AndroidEmulator)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askEmulator(cfg, virtualDevices)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askDownloadWhen(cfg)
	if err != nil {
		return &initConfig{}, err
	}
	return cfg, nil
}

func (ini *initializer) initializeXCUITest() (*initConfig, error) {
	cfg := &initConfig{frameworkName: config.KindXcuitest}

	err := ini.askFile("Application to test:", extValidator(cfg.frameworkName), completeBasic, &cfg.app)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askFile("Test application:", extValidator(cfg.frameworkName), completeBasic, &cfg.testApp)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askDevice(cfg, iOSDevicesPatterns)
	if err != nil {
		return &initConfig{}, err
	}

	err = ini.askDownloadWhen(cfg)
	if err != nil {
		return &initConfig{}, err
	}

	return cfg, nil
}