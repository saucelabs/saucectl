package ini

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"github.com/spf13/pflag"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/fatih/color"
	"github.com/saucelabs/saucectl/internal/concurrency"
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

var fallbackAndroidVirtualDevices = []vmd.VirtualDevice{
	{Name: "Android GoogleAPI Emulator", OSVersion: []string{"11.0", "10.0"}},
}

type initializer struct {
	stdio        terminal.Stdio
	infoReader   framework.MetadataService
	deviceReader devices.Reader
	vmdReader    vmd.Reader
	ccyReader    concurrency.Reader

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

	rc := rdc.New(r.APIBaseURL(), creds.Username, creds.AccessKey, rdcTimeout, config.ArtifactDownload{})

	rs := resto.New(r.APIBaseURL(), creds.Username, creds.AccessKey, restoTimeout)

	return &initializer{
		stdio:        stdio,
		infoReader:   &tc,
		deviceReader: &rc,
		vmdReader:    &rs,
		ccyReader:    &rs,
	}
}

func (ini *initializer) configure() (*initConfig, error) {
	fName, err := ini.askFramework()
	if err != nil {
		return &initConfig{}, fmt.Errorf(msg.UnableToFetchFrameworkList)
	}

	switch fName {
	case cypress.Kind:
		return ini.initializeCypress()
	case playwright.Kind:
		return ini.initializePlaywright()
	case puppeteer.Kind:
		return ini.initializePuppeteer()
	case testcafe.Kind:
		return ini.initializeTestcafe()
	case espresso.Kind:
		return ini.initializeEspresso()
	case xcuitest.Kind:
		return ini.initializeXCUITest()
	default:
		return &initConfig{}, fmt.Errorf("unsupported framework %v", fName)
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
		Options: []string{region.USWest1.String(), region.EUCentral1.String(), region.APACSoutheast1.String()},
		Default: region.USWest1.String(),
	}

	err := survey.AskOne(p, &r, survey.WithStdio(stdio.In, stdio.Out, stdio.Err))
	if err != nil {
		return "", err
	}
	return r, nil
}

func (ini *initializer) checkCredentials(region string) error {
	_, err := ini.infoReader.Frameworks(context.Background())
	if err != nil && err.Error() == "unexpected status '401' from test-composer: Unauthorized\n" {
		println()
		color.HiRed("It appears that your credentials are incorrect.")
		fmt.Printf("Use %s to update your account settings.\n", color.HiBlueString("saucectl configure"))
		println()
		return errors.New(msg.InvalidCredentials)
	}
	if err != nil && strings.Contains(err.Error(), "context deadline exceeded") {
		println()
		color.HiRed("saucectl cannot reach Sauce Labs infrastructure.")
		fmt.Printf("Check your connection and that you can access %s.\n", color.HiBlueString("https://api.%s.saucelabs.com", region))
		println()
		return errors.New(msg.UnableToCheckCredentials)
	}
	return err
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

// vmdToMaps returns a list of virtual devices, and a map containing all supported platform versions.
func vmdToMaps(vmds []vmd.VirtualDevice) ([]string, map[string][]string) {
	var vmdNames []string
	vmdOSVersions := map[string][]string{}
	for _, e := range vmds {
		vmdNames = append(vmdNames, e.Name)
		vmdOSVersions[e.Name] = e.OSVersion
	}

	sort.Strings(vmdNames)
	for _, v := range vmdOSVersions {
		sortVersions(v)
	}
	return vmdNames, vmdOSVersions
}

func (ini *initializer) askEmulator(cfg *initConfig, vmds []vmd.VirtualDevice) error {
	vmdNames, vmdOSVersions := vmdToMaps(vmds)

	q := &survey.Select{
		Message: "Select emulator:",
		Options: vmdNames,
	}
	err := survey.AskOne(q, &cfg.emulator.Name,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}

	q = &survey.Select{
		Message: "Select platform version:",
		Options: vmdOSVersions[cfg.emulator.Name],
	}
	var emulatorVersion string
	err = survey.AskOne(q, &emulatorVersion,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	cfg.emulator.PlatformVersions = []string{emulatorVersion}
	return err
}

// metaToVersions returns a list of versions for a list of meta.
func metaToVersions(metadatas []framework.Metadata) []string {
	var versions []string
	for _, v := range metadatas {
		versions = append(versions, v.FrameworkVersion)
	}
	return versions
}

// metaToBrowsers returns a sorted list of browsers, and a map containing all supported platform those browsers.
func metaToBrowsers(metadatas []framework.Metadata, frameworkName, frameworkVersion string) ([]string, map[string][]string) {
	var browsers []string
	platforms := map[string][]string{}

	var platformsToMap []framework.Platform
	hasDocker := false
	for _, v := range metadatas {
		if v.FrameworkVersion == frameworkVersion {
			platformsToMap = v.Platforms

			if v.DockerImage != "" {
				hasDocker = true
			}
		}
	}

	for _, p := range platformsToMap {
		if frameworkName == testcafe.Kind && p.PlatformName == "ios" {
			continue
		}
		for _, browserName := range correctBrowsers(p.BrowserNames) {
			if _, ok := platforms[browserName]; !ok {
				browsers = append(browsers, browserName)
				platforms[browserName] = []string{}
			}
			platforms[browserName] = append(platforms[browserName], p.PlatformName)
		}
	}

	for _, v := range platforms {
		sort.Strings(v)
	}

	// ensure that docker is the last platform in the drop-down.
	if hasDocker {
		for _, browserName := range dockerBrowsers(frameworkName) {
			if _, ok := platforms[browserName]; !ok {
				browsers = append(browsers, browserName)
				platforms[browserName] = []string{}
			}
			platforms[browserName] = append(platforms[browserName], "docker")
		}
	}

	sort.Strings(browsers)
	return browsers, platforms
}

func correctBrowsers(browsers []string) []string {
	var cb []string
	for _, browserName := range browsers {
		cb = append(cb, correctBrowser(browserName))
	}
	return cb
}

func correctBrowser(browserName string) string {
	switch browserName {
	case "playwright-chromium":
		return "chromium"
	case "playwright-firefox":
		return "firefox"
	case "playwright-webkit":
		return "webkit"
	case "googlechrome":
		return "chrome"
	default:
		return browserName
	}
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
	browsers, platforms := metaToBrowsers(metadatas, cfg.frameworkName, cfg.frameworkVersion)

	// Select browser
	q := &survey.Select{
		Message: "Select browser:",
		Options: browsers,
	}
	err := survey.AskOne(q, &cfg.browserName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}

	q = &survey.Select{
		Message: "Select platform:",
		Options: platforms[cfg.browserName],
	}
	err = survey.AskOne(q, &cfg.platformName,
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
	cfg := &initConfig{frameworkName: cypress.Kind}

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
	cfg := &initConfig{frameworkName: playwright.Kind}

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
	cfg := &initConfig{frameworkName: testcafe.Kind}

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
	cfg := &initConfig{frameworkName: puppeteer.Kind}

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
	cfg := &initConfig{frameworkName: espresso.Kind}

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
		println()
		color.HiRed("saucectl is unable to fetch the emulators list.")
		fmt.Printf("You will be able to choose only in a subset of available emulators.\n")
		fmt.Printf("To get the complete list, check your connection and try again.\n")
		println()
		virtualDevices = fallbackAndroidVirtualDevices
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
	cfg := &initConfig{frameworkName: xcuitest.Kind}

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

func checkFrameworkVersion(metadatas []framework.Metadata, frameworkName, frameworkVersion string) error {
	var supported []string
	for _, fm := range metadatas {
		if fm.FrameworkVersion == frameworkVersion {
			return nil
		}
		supported = append(supported, fm.FrameworkVersion)
	}
	return fmt.Errorf("%s %s is not supported. Supported versions are: %s", frameworkName, frameworkVersion, strings.Join(supported, ", "))
}

func checkBrowserAndPlatform(metadatas []framework.Metadata, frameworkName, frameworkVersion, browserName, platformName string) error {
	browsers, platforms := metaToBrowsers(metadatas, frameworkName, frameworkVersion)
	if ok := sliceContainsString(browsers, browserName); !ok {
		return fmt.Errorf("%s: unsupported browser. Supported browsers are: %s", browserName, strings.Join(browsers, ", "))
	}
	if ok := sliceContainsString(platforms[browserName], platformName); !ok {
		return fmt.Errorf("%s: unsupported browser on %s", browserName, platformName)
	}
	return nil
}

func checkArtifactDownloadSetting(when string) (config.When, error) {
	switch when {
	case "pass":
		return config.WhenPass, nil
	case "fail":
		return config.WhenFail, nil
	case "always":
		return config.WhenAlways, nil
	case "never":
		return config.WhenNever, nil
	default:
		return "", fmt.Errorf("%s: unknown download condition", when)
	}
}

func checkEmulators(vmds []vmd.VirtualDevice, emulator config.Emulator) (config.Emulator, []error) {
	var errs []error

	d := vmd.VirtualDevice{}
	for _, dev := range vmds {
		if strings.EqualFold(dev.Name, emulator.Name) {
			d = dev
			break
		}
	}
	if d.Name == "" {
		return config.Emulator{}, []error{fmt.Errorf("emulator: %s does not exists", emulator.Name)}
	}
	for _, p := range emulator.PlatformVersions {
		if !sliceContainsString(d.OSVersion, p) {
			errs = append(errs, fmt.Errorf("emulator: %s does not support platform %s", emulator.Name, p))
		}
	}
	if len(errs) > 0 {
		return config.Emulator{}, errs
	}
	return config.Emulator{
		Name:             d.Name,
		PlatformVersions: emulator.PlatformVersions,
		PlatformName:     emulator.PlatformName,
		Orientation:      emulator.Orientation,
	}, []error{}
}

func (ini *initializer) initializeBatchCypress(initCfg *initConfig) (*initConfig, []error) {
	initCfg.frameworkName = cypress.Kind
	var errs []error

	if initCfg.frameworkVersion == "" {
		errs = append(errs, fmt.Errorf(msg.MissingFrameworkVersion, initCfg.frameworkName))
	}
	if initCfg.cypressJSON == "" {
		errs = append(errs, errors.New(msg.MissingCypressConfig))
	}
	if initCfg.platformName == "" {
		errs = append(errs, errors.New(msg.MissingPlatformName))
	}
	if initCfg.browserName == "" {
		errs = append(errs, errors.New(msg.MissingBrowserName))
	}

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), initCfg.frameworkName)
	if err != nil {
		errs = append(errs, err)
		return &initConfig{}, errs
	}

	frameworkVersionSupported := true
	if initCfg.frameworkVersion != "" {
		if err = checkFrameworkVersion(frameworkMetadatas, initCfg.frameworkName, initCfg.frameworkVersion); err != nil {
			errs = append(errs, err)
			frameworkVersionSupported = false
		}
	}

	if initCfg.cypressJSON != "" {
		verifier := extValidator(initCfg.frameworkName)
		if err := verifier(initCfg.cypressJSON); err != nil {
			errs = append(errs, err)
		}
	}

	if frameworkVersionSupported && initCfg.platformName != "" && initCfg.browserName != "" {
		initCfg.platformName = strings.ToLower(initCfg.platformName)
		initCfg.browserName = strings.ToLower(initCfg.browserName)
		if err = checkBrowserAndPlatform(frameworkMetadatas, initCfg.frameworkName, initCfg.frameworkVersion, initCfg.browserName, initCfg.platformName); err != nil {
			errs = append(errs, err)
		}
	}

	if initCfg.artifactWhenStr != "" {
		initCfg.artifactWhenStr = strings.ToLower(initCfg.artifactWhenStr)
		if initCfg.artifactWhen, err = checkArtifactDownloadSetting(initCfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	return initCfg, errs
}

func (ini *initializer) initializeBatchEspresso(f *pflag.FlagSet, initCfg *initConfig) (*initConfig, []error) {
	initCfg.frameworkName = espresso.Kind
	var errs []error
	var err error

	if initCfg.app == "" {
		errs = append(errs, errors.New(msg.MissingApp))
	}
	if initCfg.testApp == "" {
		errs = append(errs, errors.New(msg.MissingTestApp))
	}
	if !f.Changed("device") && !f.Changed("emulator") {
		errs = append(errs, errors.New(msg.MissingDeviceOrEmulator))
	}
	if initCfg.artifactWhenStr != "" {
		initCfg.artifactWhenStr = strings.ToLower(initCfg.artifactWhenStr)
		if initCfg.artifactWhen, err = checkArtifactDownloadSetting(initCfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}

	if initCfg.app != "" {
		verifier := extValidator(initCfg.frameworkName)
		if err = verifier(initCfg.app); err != nil {
			errs = append(errs, fmt.Errorf("app: %s", err))
		}
	}
	if initCfg.testApp != "" {
		verifier := extValidator(initCfg.frameworkName)
		if err = verifier(initCfg.app); err != nil {
			errs = append(errs, fmt.Errorf("testApp: %s", err))
		}
	}
	if f.Changed("emulator") {
		emulators, err := ini.vmdReader.GetVirtualDevices(context.Background(), vmd.AndroidEmulator)
		if err != nil {
			errs = append(errs, fmt.Errorf(""))
		}
		var lerrs []error
		if initCfg.emulator, lerrs = checkEmulators(emulators, initCfg.emulatorFlag.Emulator); len(lerrs) > 0 {
			errs = append(errs, lerrs...)
		}
	}
	if f.Changed("device") {
		initCfg.device = initCfg.deviceFlag.Device
	}
	return initCfg, errs
}

func (ini *initializer) initializeBatchPlaywright(initCfg *initConfig) (*initConfig, []error) {
	initCfg.frameworkName = playwright.Kind
	var errs []error

	if initCfg.frameworkVersion == "" {
		errs = append(errs, fmt.Errorf(msg.MissingFrameworkVersion, initCfg.frameworkName))
	}
	if initCfg.platformName == "" {
		errs = append(errs, errors.New(msg.MissingPlatformName))
	}
	if initCfg.browserName == "" {
		errs = append(errs, errors.New(msg.MissingBrowserName))
	}

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), initCfg.frameworkName)
	if err != nil {
		errs = append(errs, err)
		return &initConfig{}, errs
	}

	frameworkVersionSupported := true
	if initCfg.frameworkVersion != "" {
		if err = checkFrameworkVersion(frameworkMetadatas, initCfg.frameworkName, initCfg.frameworkVersion); err != nil {
			errs = append(errs, err)
			frameworkVersionSupported = false
		}
	}

	if frameworkVersionSupported && initCfg.platformName != "" && initCfg.browserName != "" {
		initCfg.platformName = strings.ToLower(initCfg.platformName)
		initCfg.browserName = strings.ToLower(initCfg.browserName)
		if err = checkBrowserAndPlatform(frameworkMetadatas, initCfg.frameworkName, initCfg.frameworkVersion, initCfg.browserName, initCfg.platformName); err != nil {
			errs = append(errs, err)
		}
	}

	if initCfg.artifactWhenStr != "" {
		initCfg.artifactWhenStr = strings.ToLower(initCfg.artifactWhenStr)
		if initCfg.artifactWhen, err = checkArtifactDownloadSetting(initCfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	return initCfg, errs
}

func (ini *initializer) initializeBatchPuppeteer(initCfg *initConfig) (*initConfig, []error) {
	initCfg.frameworkName = puppeteer.Kind
	var errs []error

	if initCfg.frameworkVersion == "" {
		errs = append(errs, fmt.Errorf(msg.MissingFrameworkVersion, initCfg.frameworkName))
	}
	if initCfg.platformName == "" {
		errs = append(errs, errors.New(msg.MissingPlatformName))
	}
	if initCfg.browserName == "" {
		errs = append(errs, errors.New(msg.MissingBrowserName))
	}

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), initCfg.frameworkName)
	if err != nil {
		errs = append(errs, err)
		return &initConfig{}, errs
	}

	frameworkVersionSupported := true
	if initCfg.frameworkVersion != "" {
		if err = checkFrameworkVersion(frameworkMetadatas, initCfg.frameworkName, initCfg.frameworkVersion); err != nil {
			errs = append(errs, err)
			frameworkVersionSupported = false
		}
	}

	if frameworkVersionSupported && initCfg.platformName != "" && initCfg.browserName != "" {
		initCfg.platformName = strings.ToLower(initCfg.platformName)
		initCfg.browserName = strings.ToLower(initCfg.browserName)
		if err = checkBrowserAndPlatform(frameworkMetadatas, initCfg.frameworkName, initCfg.frameworkVersion, initCfg.browserName, initCfg.platformName); err != nil {
			errs = append(errs, err)
		}
	}

	if initCfg.artifactWhenStr != "" {
		initCfg.artifactWhenStr = strings.ToLower(initCfg.artifactWhenStr)
		if initCfg.artifactWhen, err = checkArtifactDownloadSetting(initCfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	return initCfg, errs
}

func (ini *initializer) initializeBatchTestcafe(initCfg *initConfig) (*initConfig, []error) {
	initCfg.frameworkName = testcafe.Kind
	var errs []error

	if initCfg.frameworkVersion == "" {
		errs = append(errs, fmt.Errorf(msg.MissingFrameworkVersion, initCfg.frameworkName))
	}
	if initCfg.platformName == "" {
		errs = append(errs, errors.New(msg.MissingPlatformName))
	}
	if initCfg.browserName == "" {
		errs = append(errs, errors.New(msg.MissingBrowserName))
	}

	frameworkMetadatas, err := ini.infoReader.Versions(context.Background(), initCfg.frameworkName)
	if err != nil {
		errs = append(errs, err)
		return &initConfig{}, errs
	}

	frameworkVersionSupported := true
	if initCfg.frameworkVersion != "" {
		if err = checkFrameworkVersion(frameworkMetadatas, initCfg.frameworkName, initCfg.frameworkVersion); err != nil {
			errs = append(errs, err)
			frameworkVersionSupported = false
		}
	}

	if frameworkVersionSupported && initCfg.platformName != "" && initCfg.browserName != "" {
		initCfg.platformName = strings.ToLower(initCfg.platformName)
		initCfg.browserName = strings.ToLower(initCfg.browserName)
		if err = checkBrowserAndPlatform(frameworkMetadatas, initCfg.frameworkName, initCfg.frameworkVersion, initCfg.browserName, initCfg.platformName); err != nil {
			errs = append(errs, err)
		}
	}

	if initCfg.artifactWhenStr != "" {
		initCfg.artifactWhenStr = strings.ToLower(initCfg.artifactWhenStr)
		if initCfg.artifactWhen, err = checkArtifactDownloadSetting(initCfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	return initCfg, errs
}

func (ini *initializer) initializeBatchXcuitest(f *pflag.FlagSet, initCfg *initConfig) (*initConfig, []error) {
	initCfg.frameworkName = xcuitest.Kind
	var errs []error
	var err error

	if initCfg.app == "" {
		errs = append(errs, errors.New(msg.MissingApp))
	}
	if initCfg.testApp == "" {
		errs = append(errs, errors.New(msg.MissingTestApp))
	}
	if !f.Changed("device") {
		errs = append(errs, errors.New(msg.MissingDevice))
	}
	if initCfg.artifactWhenStr != "" {
		initCfg.artifactWhenStr = strings.ToLower(initCfg.artifactWhenStr)
		if initCfg.artifactWhen, err = checkArtifactDownloadSetting(initCfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	if initCfg.app != "" {
		verifier := extValidator(initCfg.frameworkName)
		if err = verifier(initCfg.app); err != nil {
			errs = append(errs, fmt.Errorf("app: %s", err))
		}
	}
	if initCfg.testApp != "" {
		verifier := extValidator(initCfg.frameworkName)
		if err = verifier(initCfg.app); err != nil {
			errs = append(errs, fmt.Errorf("testApp: %s", err))
		}
	}
	if f.Changed("device") {
		initCfg.device = initCfg.deviceFlag.Device
	}
	return initCfg, errs
}
