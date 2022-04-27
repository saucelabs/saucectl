package espresso

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "espresso"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the espresso project configuration.
type Project struct {
	config.TypeDef   `yaml:",inline" mapstructure:",squash"`
	Defaults         config.Defaults        `yaml:"defaults" json:"defaults"`
	ShowConsoleLog   bool                   `yaml:"showConsoleLog" json:"-"`
	DryRun           bool                   `yaml:"-" json:"-"`
	ConfigFilePath   string                 `yaml:"-" json:"-"`
	CLIFlags         map[string]interface{} `yaml:"-" json:"-"`
	DisableLogResult bool                   `yaml:"-" json:"-"`
	Sauce            config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	Espresso         Espresso               `yaml:"espresso,omitempty" json:"espresso"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite                `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite              `yaml:"suites,omitempty" json:"suites"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Notifications config.Notifications `yaml:"notifications,omitempty" json:"-"`
}

// Espresso represents espresso apps configuration.
type Espresso struct {
	App       string   `yaml:"app,omitempty" json:"app"`
	TestApp   string   `yaml:"testApp,omitempty" json:"testApp"`
	OtherApps []string `yaml:"otherApps,omitempty" json:"otherApps"`
}

// TestOptions represents the espresso test filter options configuration.
type TestOptions struct {
	NotClass            []string `yaml:"notClass,omitempty" json:"notClass"`
	Class               []string `yaml:"class,omitempty" json:"class"`
	Package             string   `yaml:"package,omitempty" json:"package"`
	NotPackage          string   `yaml:"notPackage,omitempty" json:"notPackage"`
	Size                string   `yaml:"size,omitempty" json:"size"`
	Annotation          string   `yaml:"annotation,omitempty" json:"annotation"`
	NotAnnotation       string   `yaml:"notAnnotation,omitempty" json:"notAnnotation"`
	ShardIndex          int      `json:"shardIndex"`
	NumShards           int      `yaml:"numShards,omitempty" json:"numShards"`
	ClearPackageData    bool     `yaml:"clearPackageData,omitempty" json:"clearPackageData"`
	UseTestOrchestrator bool     `yaml:"useTestOrchestrator,omitempty" json:"useTestOrchestrator"`
}

// Suite represents the espresso test suite configuration.
type Suite struct {
	Name        string                 `yaml:"name,omitempty" json:"name"`
	Devices     []config.Device        `yaml:"devices,omitempty" json:"devices"`
	Emulators   []config.Emulator      `yaml:"emulators,omitempty" json:"emulators"`
	TestOptions map[string]interface{} `yaml:"testOptions,omitempty" json:"testOptions"`
	Timeout     time.Duration          `yaml:"timeout,omitempty" json:"timeout"`
	AppSettings config.AppSettings     `yaml:"appSettings,omityEmpty" json:"appSettings"`
}

// Android constant
const Android = "Android"

// FromFile creates a new cypress Project based on the filepath cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	if err := config.Unmarshal(cfgPath, &p); err != nil {
		return p, err
	}
	p.ConfigFilePath = cfgPath

	return p, nil
}

// SetDefaults applies config defaults in case the user has left them blank.
func SetDefaults(p *Project) {
	if p.Kind == "" {
		p.Kind = Kind
	}

	if p.APIVersion == "" {
		p.APIVersion = APIVersion
	}

	if p.Sauce.Concurrency < 1 {
		p.Sauce.Concurrency = 2
	}

	if p.Defaults.Timeout < 0 {
		p.Defaults.Timeout = 0
	}

	p.Sauce.Tunnel.SetDefaults()
	p.Sauce.Metadata.SetDefaultBuild()

	for i, suite := range p.Suites {
		for j := range suite.Devices {
			// Android is the only choice.
			p.Suites[i].Devices[j].PlatformName = Android
			p.Suites[i].Devices[j].Options.DeviceType = strings.ToUpper(p.Suites[i].Devices[j].Options.DeviceType)
		}
		for j := range suite.Emulators {
			p.Suites[i].Emulators[j].PlatformName = Android
		}

		if suite.Timeout <= 0 {
			p.Suites[i].Timeout = p.Defaults.Timeout
		}
	}
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p Project) error {
	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New(msg.MissingRegion)
	}

	if p.Espresso.App == "" {
		return errors.New(msg.MissingAppPath)
	}
	if err := apps.Validate("application", p.Espresso.App, []string{".apk", ".aab"}); err != nil {
		return err
	}

	if p.Espresso.TestApp == "" {
		return errors.New(msg.MissingTestAppPath)
	}
	if err := apps.Validate("test application", p.Espresso.TestApp, []string{".apk", ".aab"}); err != nil {
		return err
	}

	for _, app := range p.Espresso.OtherApps {
		if err := apps.Validate("other application", app, []string{".apk", ".aab"}); err != nil {
			return err
		}
	}

	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
	}

	for _, suite := range p.Suites {
		if len(suite.Devices) == 0 && len(suite.Emulators) == 0 {
			return fmt.Errorf(msg.MissingDevicesOrEmulatorConfig, suite.Name)
		}
		if err := validateDevices(suite.Name, suite.Devices); err != nil {
			return err
		}
		if err := validateEmulators(suite.Name, suite.Emulators); err != nil {
			return err
		}
	}

	return nil
}

func validateDevices(suiteName string, devices []config.Device) error {
	for didx, device := range devices {
		if device.Name == "" && device.ID == "" {
			return fmt.Errorf(msg.MissingDeviceConfig, suiteName, didx)
		}
		if device.Options.DeviceType != "" && !config.IsSupportedDeviceType(device.Options.DeviceType) {
			return fmt.Errorf(msg.InvalidDeviceType,
				device.Options.DeviceType, suiteName, didx, strings.Join(config.SupportedDeviceTypes, ","))
		}
	}
	return nil
}

func validateEmulators(suiteName string, emulators []config.Emulator) error {
	for eidx, emulator := range emulators {
		if emulator.Name == "" {
			return fmt.Errorf(msg.MissingEmulatorName, suiteName, eidx)
		}
		if !strings.Contains(strings.ToLower(emulator.Name), "emulator") {
			return fmt.Errorf(msg.InvalidEmulatorName, emulator.Name, suiteName, eidx)
		}
		if len(emulator.PlatformVersions) == 0 {
			return fmt.Errorf(msg.MissingEmulatorPlatformVersion, emulator.Name, suiteName, eidx)
		}
	}
	return nil
}

// FilterSuites filters out suites in the project that don't match the given suite name.
func FilterSuites(p *Project, suiteName string) error {
	for _, s := range p.Suites {
		if s.Name == suiteName {
			p.Suites = []Suite{s}
			return nil
		}
	}
	return fmt.Errorf(msg.SuiteNameNotFound, suiteName)
}

func IsSharded(suites []Suite) bool {
	for _, suite := range suites {
		if v, ok := suite.TestOptions["numShards"]; ok {
			val, err := strconv.Atoi(fmt.Sprintf("%v", v))
			return err == nil && val > 0
		}
	}
	return false
}
