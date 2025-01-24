package ini

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/fatih/color"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/vmd"
	"github.com/saucelabs/saucectl/internal/xctest"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"github.com/spf13/pflag"
)

var androidDevicesPatterns = []string{
	"Amazon Kindle Fire .*", "Google Pixel .*", "HTC .*", "Huawei .*",
	"LG .*", "Motorola .*", "OnePlus .*", "Samsung .*", "Sony .*",
}

var iOSDevicesPatterns = []string{
	"iPad .*",
	"iPhone .*",
}

var fallbackAndroidVirtualDevices = []vmd.VirtualDevice{
	{Name: "Android GoogleAPI Emulator", OSVersion: []string{"11.0", "10.0"}},
}

var fallbackIOSVirtualDevices = []vmd.VirtualDevice{
	{Name: "iPhone Simulator", OSVersion: []string{"16.2"}},
}

type initializer struct {
	stdio        terminal.Stdio
	infoReader   framework.MetadataService
	deviceReader devices.Reader
	vmdReader    vmd.Reader
	userService  iam.UserService
	cfg          *initConfig
}

// newInitializer creates a new initializer instance.
func newInitializer(stdio terminal.Stdio, creds iam.Credentials, cfg *initConfig) *initializer {
	r := region.FromString(cfg.region)
	tc := http.NewTestComposer(r.APIBaseURL(), creds, testComposerTimeout)
	rc := http.NewRDCService(r, creds.Username, creds.AccessKey, rdcTimeout)
	rs := http.NewResto(r, creds.Username, creds.AccessKey, restoTimeout)
	us := http.NewUserService(r.APIBaseURL(), creds, 5*time.Second)

	return &initializer{
		stdio:        stdio,
		infoReader:   &tc,
		deviceReader: &rc,
		vmdReader:    &rs,
		userService:  &us,
		cfg:          cfg,
	}
}

func (ini *initializer) configure(ctx context.Context) error {
	switch ini.cfg.frameworkName {
	case cypress.Kind:
		return ini.initializeCypress(ctx)
	case playwright.Kind:
		return ini.initializePlaywright(ctx)
	case testcafe.Kind:
		return ini.initializeTestcafe(ctx)
	case espresso.Kind:
		return ini.initializeEspresso(ctx)
	case xcuitest.Kind:
		return ini.initializeXCUITest(ctx)
	case xctest.Kind:
		return ini.initializeXCTest()
	case imagerunner.Kind:
		return ini.initializeImageRunner()
	default:
		return fmt.Errorf("unsupported framework %q", ini.cfg.frameworkName)
	}
}

func askCredentials(stdio terminal.Stdio) (iam.Credentials, error) {
	creds := iam.Credentials{}
	q := &survey.Input{Message: "Sauce Labs username:"}

	err := survey.AskOne(q, &creds.Username,
		survey.WithValidator(survey.Required),
		survey.WithShowCursor(true),
		survey.WithStdio(stdio.In, stdio.Out, stdio.Err))
	if err != nil {
		return creds, err
	}

	q = &survey.Input{Message: "Sauce Labs access key:"}
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

func (ini *initializer) checkCredentials(ctx context.Context, region string) error {
	_, err := ini.infoReader.Frameworks(ctx)
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

func (ini *initializer) askDownloadWhen() error {
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
	ini.cfg.artifactWhen = mapWhen[when]
	return nil
}

func (ini *initializer) askDevice(suggestions []string) error {
	q := &survey.Select{
		Message: "Select device pattern:",
		Options: suggestions,
	}
	return survey.AskOne(q, &ini.cfg.device.Name,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err),
	)
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

func (ini *initializer) askSimulator(vmds []vmd.VirtualDevice) error {
	vmdNames, vmdOSVersions := vmdToMaps(vmds)

	q := &survey.Select{
		Message: "Select simulator:",
		Options: vmdNames,
	}
	err := survey.AskOne(q, &ini.cfg.simulator.Name,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}

	q = &survey.Select{
		Message: "Select platform version:",
		Options: vmdOSVersions[ini.cfg.simulator.Name],
	}
	var simulatorVersion string
	err = survey.AskOne(q, &simulatorVersion,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	ini.cfg.simulator.PlatformVersions = []string{simulatorVersion}
	return err
}

func (ini *initializer) askEmulator(vmds []vmd.VirtualDevice) error {
	vmdNames, vmdOSVersions := vmdToMaps(vmds)

	q := &survey.Select{
		Message: "Select emulator:",
		Options: vmdNames,
	}
	err := survey.AskOne(q, &ini.cfg.emulator.Name,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}

	q = &survey.Select{
		Message: "Select platform version:",
		Options: vmdOSVersions[ini.cfg.emulator.Name],
	}
	var emulatorVersion string
	err = survey.AskOne(q, &emulatorVersion,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	ini.cfg.emulator.PlatformVersions = []string{emulatorVersion}
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
	for _, v := range metadatas {
		if v.FrameworkVersion == frameworkVersion {
			platformsToMap = v.Platforms
		}
	}

	for _, p := range platformsToMap {
		p.PlatformName = normalizePlatform(p.PlatformName)

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

	sort.Strings(browsers)
	return browsers, platforms
}

func normalizePlatform(platform string) string {
	r := strings.NewReplacer("macos", "macOS", "windows", "Windows")
	return r.Replace(platform)
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

func (ini *initializer) askPlatform(metadatas []framework.Metadata) error {
	browsers, platforms := metaToBrowsers(metadatas, ini.cfg.frameworkName, ini.cfg.frameworkVersion)

	// Select browser
	q := &survey.Select{
		Message: "Select browser:",
		Options: browsers,
	}
	err := survey.AskOne(q, &ini.cfg.browserName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}

	q = &survey.Select{
		Message: "Select platform:",
		Options: platforms[ini.cfg.browserName],
	}
	err = survey.AskOne(q, &ini.cfg.platformName,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	return nil
}

func (ini *initializer) askVersion(metadatas []framework.Metadata) error {
	versions := metaToVersions(metadatas)

	q := &survey.Select{
		Message: fmt.Sprintf("Select %s version:", ini.cfg.frameworkName),
		Options: versions,
	}

	err := survey.AskOne(q, &ini.cfg.frameworkVersion,
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

func (ini *initializer) askDockerImage(message string, val survey.Validator, targetValue *string) error {
	q := &survey.Input{
		Message: message,
	}

	return survey.AskOne(q, targetValue,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithValidator(val),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
}

var Workloads = []string{
	"webdriver",
	"other",
}

func (ini *initializer) askWorkload() error {
	q := &survey.Select{
		Message: "Set workload:",
		Default: Workloads[0],
		Options: Workloads,
	}
	q.WithStdio(ini.stdio)

	var workload string
	err := survey.AskOne(q, &workload,
		survey.WithShowCursor(true),
		survey.WithValidator(survey.Required),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err))
	if err != nil {
		return err
	}
	ini.cfg.workload = workload
	return nil
}

func (ini *initializer) initializeCypress(ctx context.Context) error {
	frameworkMetadatas, err := ini.infoReader.Versions(ctx, ini.cfg.frameworkName)
	if err != nil {
		return err
	}

	err = ini.askVersion(frameworkMetadatas)
	if err != nil {
		return err
	}

	err = ini.askFile(
		"Cypress configuration file:",
		frameworkExtValidator(ini.cfg.frameworkName, ini.cfg.frameworkVersion),
		completeBasic,
		&ini.cfg.cypressConfigFile,
	)
	if err != nil {
		return err
	}

	err = ini.askPlatform(frameworkMetadatas)
	if err != nil {
		return err
	}

	return ini.askDownloadWhen()
}

func (ini *initializer) initializePlaywright(ctx context.Context) error {
	frameworkMetadatas, err := ini.infoReader.Versions(ctx, ini.cfg.frameworkName)
	if err != nil {
		return err
	}

	err = ini.askVersion(frameworkMetadatas)
	if err != nil {
		return err
	}

	err = ini.askPlatform(frameworkMetadatas)
	if err != nil {
		return err
	}

	err = survey.AskOne(
		&survey.Input{
			Message: "Playwright project name. " +
				"Leave empty if your configuration does not contain projects:",
			Default: "",
			Help:    "See https://playwright.dev/docs/test-projects",
		},
		&ini.cfg.playwrightProject,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err),
	)
	if err != nil {
		return err
	}

	var pattern string
	err = survey.AskOne(
		&survey.Input{
			Message: "Test file pattern to match against:",
			Default: ".*.spec.js",
			Help:    "See https://playwright.dev/docs/test-projects",
		},
		&pattern,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err),
	)
	if err != nil {
		return err
	}
	ini.cfg.testMatch = []string{pattern}

	return ini.askDownloadWhen()
}

func (ini *initializer) initializeTestcafe(ctx context.Context) error {
	frameworkMetadatas, err := ini.infoReader.Versions(ctx, ini.cfg.frameworkName)
	if err != nil {
		return err
	}

	err = ini.askVersion(frameworkMetadatas)
	if err != nil {
		return err
	}

	err = ini.askPlatform(frameworkMetadatas)
	if err != nil {
		return err
	}

	err = ini.askDownloadWhen()
	if err != nil {
		return err
	}
	return nil
}

func (ini *initializer) initializeEspresso(ctx context.Context) error {
	err := ini.askFile(
		"Application to test:",
		frameworkExtValidator(ini.cfg.frameworkName, ""),
		completeBasic,
		&ini.cfg.app,
	)
	if err != nil {
		return err
	}

	err = ini.askFile(
		"Test application:",
		frameworkExtValidator(ini.cfg.frameworkName, ""),
		completeBasic,
		&ini.cfg.testApp,
	)
	if err != nil {
		return err
	}

	err = ini.askDevice(androidDevicesPatterns)
	if err != nil {
		return err
	}

	virtualDevices, err := ini.vmdReader.GetVirtualDevices(ctx, vmd.AndroidEmulator)
	if err != nil {
		println()
		color.HiRed("saucectl is unable to fetch the emulators list.")
		fmt.Printf("You will be able to choose only in a subset of available emulators.\n")
		fmt.Printf("To get the complete list, check your connection and try again.\n")
		println()
		virtualDevices = fallbackAndroidVirtualDevices
	}

	err = ini.askEmulator(virtualDevices)
	if err != nil {
		return err
	}

	err = ini.askDownloadWhen()
	if err != nil {
		return err
	}
	return nil
}

func (ini *initializer) initializeXCUITest(ctx context.Context) error {
	q := &survey.Select{
		Message: "Select target:",
		Options: []string{
			"Real Devices",
			"Virtual Devices",
		},
	}

	var target string
	err := survey.AskOne(q, &target,
		survey.WithShowCursor(true),
		survey.WithStdio(ini.stdio.In, ini.stdio.Out, ini.stdio.Err),
		survey.WithValidator(survey.Required),
	)
	if err != nil {
		return err
	}

	if target == "Real Devices" {
		err = ini.askDevice(iOSDevicesPatterns)
		if err != nil {
			return err
		}
		err = ini.askFile("Application to test:", extValidator([]string{".ipa", ".app"}), completeBasic, &ini.cfg.app)
		if err != nil {
			return err
		}

		err = ini.askFile("Test application:", extValidator([]string{".ipa", ".app"}), completeBasic, &ini.cfg.testApp)
		if err != nil {
			return err
		}
	} else if target == "Virtual Devices" {
		virtualDevices, err := ini.vmdReader.GetVirtualDevices(ctx, vmd.IOSSimulator)
		if err != nil {
			println()
			color.HiRed("saucectl is unable to fetch the simulators list.")
			fmt.Println("You will be able to choose only in a subset of available simulators.")
			fmt.Println("To get the complete list, check your connection and try again.")
			println()
			virtualDevices = fallbackIOSVirtualDevices
		}

		err = ini.askSimulator(virtualDevices)
		if err != nil {
			return err
		}

		err = ini.askFile("Application to test:", extValidator([]string{".zip", ".app"}), completeBasic, &ini.cfg.app)
		if err != nil {
			return err
		}

		err = ini.askFile("Test application:", extValidator([]string{".zip", ".app"}), completeBasic, &ini.cfg.testApp)
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

func (ini *initializer) initializeXCTest() error {
	var err error

	err = ini.askDevice(iOSDevicesPatterns)
	if err != nil {
		return err
	}
	err = ini.askFile("Application to test:", extValidator([]string{".ipa", ".app"}), completeBasic, &ini.cfg.app)
	if err != nil {
		return err
	}

	err = ini.askFile("XCTestRun file:", extValidator([]string{".xctestrun"}), completeBasic, &ini.cfg.xctestRunFile)
	if err != nil {
		return err
	}

	err = ini.askDownloadWhen()
	if err != nil {
		return err
	}

	return nil
}

func (ini *initializer) initializeImageRunner() error {
	if err := ini.askDockerImage(
		"Docker Image to use:",
		dockerImageValidator(),
		&ini.cfg.dockerImage,
	); err != nil {
		return err
	}

	if err := ini.askWorkload(); err != nil {
		return err
	}

	return ini.askDownloadWhen()
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

func (ini *initializer) initializeBatchCypress(ctx context.Context) []error {
	var errs []error

	if ini.cfg.frameworkVersion == "" {
		errs = append(errs, fmt.Errorf(msg.MissingFrameworkVersion, ini.cfg.frameworkName))
	}
	if ini.cfg.cypressConfigFile == "" {
		errs = append(errs, errors.New(msg.MissingCypressConfig))
	}
	if ini.cfg.platformName == "" {
		errs = append(errs, errors.New(msg.MissingPlatformName))
	}
	if ini.cfg.browserName == "" {
		errs = append(errs, errors.New(msg.MissingBrowserName))
	}

	frameworkMetadatas, err := ini.infoReader.Versions(ctx, ini.cfg.frameworkName)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	frameworkVersionSupported := true
	if ini.cfg.frameworkVersion != "" {
		if err = checkFrameworkVersion(frameworkMetadatas, ini.cfg.frameworkName, ini.cfg.frameworkVersion); err != nil {
			errs = append(errs, err)
			frameworkVersionSupported = false
		}
	}

	if ini.cfg.cypressConfigFile != "" {
		verifier := frameworkExtValidator(ini.cfg.frameworkName, "")
		if err := verifier(ini.cfg.cypressConfigFile); err != nil {
			errs = append(errs, err)
		}
	}

	if frameworkVersionSupported && ini.cfg.platformName != "" && ini.cfg.browserName != "" {
		ini.cfg.browserName = strings.ToLower(ini.cfg.browserName)
		if err = checkBrowserAndPlatform(
			frameworkMetadatas,
			ini.cfg.frameworkName,
			ini.cfg.frameworkVersion,
			ini.cfg.browserName,
			ini.cfg.platformName,
		); err != nil {
			errs = append(errs, err)
		}
	}

	if ini.cfg.artifactWhenStr != "" {
		ini.cfg.artifactWhenStr = strings.ToLower(ini.cfg.artifactWhenStr)
		if ini.cfg.artifactWhen, err = checkArtifactDownloadSetting(ini.cfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (ini *initializer) initializeBatchEspresso(ctx context.Context, f *pflag.FlagSet) []error {
	var errs []error
	var err error

	if ini.cfg.app == "" {
		errs = append(errs, errors.New(msg.MissingApp))
	}
	if ini.cfg.testApp == "" {
		errs = append(errs, errors.New(msg.MissingTestApp))
	}
	if !f.Changed("device") && !f.Changed("emulator") {
		errs = append(errs, errors.New(msg.MissingDeviceOrEmulator))
	}
	if ini.cfg.artifactWhenStr != "" {
		ini.cfg.artifactWhenStr = strings.ToLower(ini.cfg.artifactWhenStr)
		if ini.cfg.artifactWhen, err = checkArtifactDownloadSetting(ini.cfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}

	if ini.cfg.app != "" {
		verifier := frameworkExtValidator(ini.cfg.frameworkName, "")
		if err = verifier(ini.cfg.app); err != nil {
			errs = append(errs, fmt.Errorf("app: %s", err))
		}
	}
	if ini.cfg.testApp != "" {
		verifier := frameworkExtValidator(ini.cfg.frameworkName, "")
		if err = verifier(ini.cfg.app); err != nil {
			errs = append(errs, fmt.Errorf("testApp: %s", err))
		}
	}
	if f.Changed("emulator") {
		emulators, err := ini.vmdReader.GetVirtualDevices(ctx, vmd.AndroidEmulator)
		if err != nil {
			errs = append(errs, fmt.Errorf(""))
		}
		var lerrs []error
		if ini.cfg.emulator, lerrs = checkEmulators(emulators, ini.cfg.emulatorFlag.Emulator); len(lerrs) > 0 {
			errs = append(errs, lerrs...)
		}
	}
	if f.Changed("device") {
		ini.cfg.device = ini.cfg.deviceFlag.Device
	}
	return errs
}

func (ini *initializer) initializeBatchPlaywright(ctx context.Context) []error {
	var errs []error

	if ini.cfg.frameworkVersion == "" {
		errs = append(errs, fmt.Errorf(msg.MissingFrameworkVersion, ini.cfg.frameworkName))
	}
	if ini.cfg.platformName == "" {
		errs = append(errs, errors.New(msg.MissingPlatformName))
	}
	if ini.cfg.browserName == "" {
		errs = append(errs, errors.New(msg.MissingBrowserName))
	}

	frameworkMetadatas, err := ini.infoReader.Versions(ctx, ini.cfg.frameworkName)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	frameworkVersionSupported := true
	if ini.cfg.frameworkVersion != "" {
		if err = checkFrameworkVersion(frameworkMetadatas, ini.cfg.frameworkName, ini.cfg.frameworkVersion); err != nil {
			errs = append(errs, err)
			frameworkVersionSupported = false
		}
	}

	if frameworkVersionSupported && ini.cfg.platformName != "" && ini.cfg.browserName != "" {
		ini.cfg.browserName = strings.ToLower(ini.cfg.browserName)
		if err = checkBrowserAndPlatform(
			frameworkMetadatas,
			ini.cfg.frameworkName,
			ini.cfg.frameworkVersion,
			ini.cfg.browserName,
			ini.cfg.platformName,
		); err != nil {
			errs = append(errs, err)
		}
	}

	if ini.cfg.artifactWhenStr != "" {
		ini.cfg.artifactWhenStr = strings.ToLower(ini.cfg.artifactWhenStr)
		if ini.cfg.artifactWhen, err = checkArtifactDownloadSetting(ini.cfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (ini *initializer) initializeBatchTestcafe(ctx context.Context) []error {
	var errs []error

	if ini.cfg.frameworkVersion == "" {
		errs = append(errs, fmt.Errorf(msg.MissingFrameworkVersion, ini.cfg.frameworkName))
	}
	if ini.cfg.platformName == "" {
		errs = append(errs, errors.New(msg.MissingPlatformName))
	}
	if ini.cfg.browserName == "" {
		errs = append(errs, errors.New(msg.MissingBrowserName))
	}

	frameworkMetadatas, err := ini.infoReader.Versions(ctx, ini.cfg.frameworkName)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	frameworkVersionSupported := true
	if ini.cfg.frameworkVersion != "" {
		if err = checkFrameworkVersion(frameworkMetadatas, ini.cfg.frameworkName, ini.cfg.frameworkVersion); err != nil {
			errs = append(errs, err)
			frameworkVersionSupported = false
		}
	}

	if frameworkVersionSupported && ini.cfg.platformName != "" && ini.cfg.browserName != "" {
		ini.cfg.browserName = strings.ToLower(ini.cfg.browserName)
		if err = checkBrowserAndPlatform(
			frameworkMetadatas,
			ini.cfg.frameworkName,
			ini.cfg.frameworkVersion,
			ini.cfg.browserName,
			ini.cfg.platformName,
		); err != nil {
			errs = append(errs, err)
		}
	}

	if ini.cfg.artifactWhenStr != "" {
		ini.cfg.artifactWhenStr = strings.ToLower(ini.cfg.artifactWhenStr)
		if ini.cfg.artifactWhen, err = checkArtifactDownloadSetting(ini.cfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (ini *initializer) initializeBatchXcuitest(f *pflag.FlagSet) []error {
	var errs []error
	var err error

	if ini.cfg.app == "" {
		errs = append(errs, errors.New(msg.MissingApp))
	}
	if ini.cfg.testApp == "" {
		errs = append(errs, errors.New(msg.MissingTestApp))
	}
	if !(f.Changed("simulator") || f.Changed("device")) {
		errs = append(errs, errors.New(msg.MissingDeviceOrSimulator))
	}
	if ini.cfg.artifactWhenStr != "" {
		ini.cfg.artifactWhenStr = strings.ToLower(ini.cfg.artifactWhenStr)
		if ini.cfg.artifactWhen, err = checkArtifactDownloadSetting(ini.cfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	validExt := []string{".app"}
	if f.Changed("simulator") {
		validExt = append(validExt, ".zip")
	} else {
		validExt = append(validExt, ".ipa")
	}
	if ini.cfg.app != "" {
		verifier := extValidator(validExt)
		if err = verifier(ini.cfg.app); err != nil {
			errs = append(errs, fmt.Errorf("app: %s", err))
		}
	}
	if ini.cfg.testApp != "" {
		verifier := extValidator(validExt)
		if err = verifier(ini.cfg.app); err != nil {
			errs = append(errs, fmt.Errorf("testApp: %s", err))
		}
	}
	if f.Changed("device") {
		ini.cfg.device = ini.cfg.deviceFlag.Device
	}
	if f.Changed("simulator") {
		ini.cfg.simulator = ini.cfg.simulatorFlag.Simulator
	}
	return errs
}

func (ini *initializer) initializeBatchXctest(f *pflag.FlagSet) []error {
	var errs []error
	var err error

	if ini.cfg.app == "" {
		errs = append(errs, errors.New(msg.MissingApp))
	}
	if ini.cfg.xctestRunFile == "" {
		errs = append(errs, errors.New(msg.MissingXCTestFileAppPath))
	}
	if !f.Changed("device") {
		errs = append(errs, errors.New(msg.MissingDevice))
	}
	if ini.cfg.artifactWhenStr != "" {
		ini.cfg.artifactWhenStr = strings.ToLower(ini.cfg.artifactWhenStr)
		if ini.cfg.artifactWhen, err = checkArtifactDownloadSetting(ini.cfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	validAppExt := []string{".app"}
	if f.Changed("simulator") {
		validAppExt = append(validAppExt, ".zip")
	} else {
		validAppExt = append(validAppExt, ".ipa")
	}
	if ini.cfg.app != "" {
		verifier := extValidator(validAppExt)
		if err = verifier(ini.cfg.app); err != nil {
			errs = append(errs, fmt.Errorf("app: %s", err))
		}
	}
	if ini.cfg.xctestRunFile != "" {
		verifier := extValidator([]string{".xctestrun"})
		if err = verifier(ini.cfg.xctestRunFile); err != nil {
			errs = append(errs, fmt.Errorf("xctestRunFile: %s", err))
		}
	}
	if f.Changed("device") {
		ini.cfg.device = ini.cfg.deviceFlag.Device
	}
	return errs
}

func (ini *initializer) initializeBatchImageRunner() []error {
	var errs []error
	var err error

	if ini.cfg.dockerImage == "" {
		errs = append(errs, errors.New(msg.MissingDockerImage))
	}
	if ini.cfg.dockerImage != "" {
		verifier := dockerImageValidator()
		if err = verifier(ini.cfg.dockerImage); err != nil {
			errs = append(errs, fmt.Errorf("dockerImage: %s", err))
		}
	}
	if ini.cfg.artifactWhenStr != "" {
		ini.cfg.artifactWhenStr = strings.ToLower(ini.cfg.artifactWhenStr)
		if ini.cfg.artifactWhen, err = checkArtifactDownloadSetting(ini.cfg.artifactWhenStr); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
