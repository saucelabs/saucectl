package xcuitest

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "xcuitest"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the xcuitest project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	Defaults       config.Defaults        `yaml:"defaults,omitempty" json:"defaults"`
	ConfigFilePath string                 `yaml:"-" json:"-"`
	ShowConsoleLog bool                   `yaml:"showConsoleLog" json:"-"`
	DryRun         bool                   `yaml:"-" json:"-"`
	CLIFlags       map[string]interface{} `yaml:"-" json:"-"`
	Sauce          config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	Xcuitest       Xcuitest               `yaml:"xcuitest,omitempty" json:"xcuitest"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite                `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite              `yaml:"suites,omitempty" json:"suites"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Notifications config.Notifications `yaml:"notifications,omitempty" json:"-"`
}

// Xcuitest represents xcuitest apps configuration.
type Xcuitest struct {
	App       string   `yaml:"app,omitempty" json:"app"`
	TestApp   string   `yaml:"testApp,omitempty" json:"testApp"`
	OtherApps []string `yaml:"otherApps,omitempty" json:"otherApps"`
}

// TestOptions represents the xcuitest test filter options configuration.
type TestOptions struct {
	NotClass []string `yaml:"notClass,omitempty" json:"notClass"`
	Class    []string `yaml:"class,omitempty" json:"class"`
}

// Suite represents the xcuitest test suite configuration.
type Suite struct {
	Name        string             `yaml:"name,omitempty" json:"name"`
	TestApp     string             `yaml:"testApp,omitempty" json:"testApp"`
	Timeout     time.Duration      `yaml:"timeout,omitempty" json:"timeout"`
	Devices     []config.Device    `yaml:"devices,omitempty" json:"devices"`
	TestOptions TestOptions        `yaml:"testOptions,omitempty" json:"testOptions"`
	AppSettings config.AppSettings `yaml:"appSettings,omityEmpty" json:"appSettings"`
}

// IOS constant
const IOS = "iOS"

// FromFile creates a new xcuitest Project based on the filepath cfgPath.
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

	for ks, suite := range p.Suites {
		for id := range suite.Devices {
			suite.Devices[id].PlatformName = "iOS"

			// device type only supports uppercase values
			suite.Devices[id].Options.DeviceType = strings.ToUpper(suite.Devices[id].Options.DeviceType)
		}

		if suite.Timeout <= 0 {
			p.Suites[ks].Timeout = p.Defaults.Timeout
		}

		if suite.TestApp == "" {
			p.Suites[ks].TestApp = p.Xcuitest.TestApp
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

	if p.Xcuitest.App == "" {
		return errors.New(msg.MissingXcuitestAppPath)
	}
	if err := apps.Validate("application", p.Xcuitest.App, []string{".ipa", ".app"}); err != nil {
		return err
	}

	if p.Xcuitest.TestApp == "" {
		return errors.New(msg.MissingXcuitestTestAppPath)
	}
	if err := apps.Validate("test application", p.Xcuitest.TestApp, []string{".ipa", ".app"}); err != nil {
		return err
	}

	for _, app := range p.Xcuitest.OtherApps {
		if err := apps.Validate("other application", app, []string{".ipa", ".app"}); err != nil {
			return err
		}
	}

	if p.Sauce.LaunchOrder != "" && p.Sauce.LaunchOrder != config.LaunchOrderFailRate {
		return fmt.Errorf(msg.InvalidLaunchingOption, p.Sauce.LaunchOrder, string(config.LaunchOrderFailRate))
	}

	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
	}

	for _, suite := range p.Suites {
		if len(suite.Devices) == 0 {
			return fmt.Errorf(msg.MissingXcuitestDeviceConfig, suite.Name)
		}
		for didx, device := range suite.Devices {
			if device.ID == "" && device.Name == "" {
				return fmt.Errorf(msg.MissingDeviceConfig, suite.Name, didx)
			}

			if device.Options.DeviceType != "" && !config.IsSupportedDeviceType(device.Options.DeviceType) {
				return fmt.Errorf(msg.InvalidDeviceType,
					device.Options.DeviceType, suite.Name, didx, strings.Join(config.SupportedDeviceTypes, ","))
			}
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

// SortByHistory sorts the suites in the order of job history
func SortByHistory(suites []Suite, history insights.JobHistory) []Suite {
	hash := map[string]Suite{}
	for _, s := range suites {
		hash[s.Name] = s
	}
	var res []Suite
	for _, s := range history.TestCases {
		if v, ok := hash[s.Name]; ok {
			res = append(res, v)
			delete(hash, s.Name)
		}
	}
	for _, v := range suites {
		if _, ok := hash[v.Name]; ok {
			res = append(res, v)
		}
	}
	return res
}
