package xcuitest

import (
	"errors"
	"fmt"
	"strings"

	"github.com/saucelabs/saucectl/internal/region"

	"github.com/saucelabs/saucectl/internal/config"
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
	ConfigFilePath string             `yaml:"-" json:"-"`
	DryRun         bool               `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Xcuitest       Xcuitest           `yaml:"xcuitest,omitempty" json:"xcuitest"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite     Suite            `yaml:"suite,omitempty" json:"-"`
	Suites    []Suite          `yaml:"suites,omitempty" json:"suites"`
	Artifacts config.Artifacts `yaml:"artifacts,omitempty" json:"artifacts"`
}

// Xcuitest represents xcuitest apps configuration.
type Xcuitest struct {
	App       string   `yaml:"app,omitempty" json:"app"`
	TestApp   string   `yaml:"testApp,omitempty" json:"testApp"`
	OtherApps []string `yaml:"otherApps,omitempty" json:"otherApps"`
}

// TestOptions represents the xcuitest test filter options configuration.
type TestOptions struct {
	Class []string `yaml:"class,omitempty" json:"class"`
}

// Suite represents the xcuitest test suite configuration.
type Suite struct {
	Name        string          `yaml:"name,omitempty" json:"name"`
	Devices     []config.Device `yaml:"devices,omitempty" json:"devices"`
	TestOptions TestOptions     `yaml:"testOptions,omitempty" json:"testOptions"`
}

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

	for _, suite := range p.Suites {
		for id := range suite.Devices {
			suite.Devices[id].PlatformName = "iOS"

			// device type only supports uppercase values
			suite.Devices[id].Options.DeviceType = strings.ToUpper(suite.Devices[id].Options.DeviceType)
		}
	}
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p Project) error {
	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New("no sauce region set")
	}

	if p.Xcuitest.App == "" {
		return errors.New("missing path to app .ipa")
	}
	if !strings.HasSuffix(p.Xcuitest.App, ".ipa") && !strings.HasSuffix(p.Xcuitest.App, ".app") {
		return fmt.Errorf("invalid application file: %s, make sure extension is .ipa or .app", p.Xcuitest.App)
	}

	if p.Xcuitest.TestApp == "" {
		return errors.New("missing path to test app .ipa")
	}
	if !strings.HasSuffix(p.Xcuitest.TestApp, ".ipa") && !strings.HasSuffix(p.Xcuitest.TestApp, ".app") {
		return fmt.Errorf("invalid application test file: %s, make sure extension is .ipa or .app", p.Xcuitest.TestApp)
	}

	if len(p.Suites) == 0 {
		return errors.New("no suites defined")
	}

	for _, suite := range p.Suites {
		if len(suite.Devices) == 0 {
			return fmt.Errorf("missing devices configuration for suite: %s", suite.Name)
		}
		for didx, device := range suite.Devices {
			if device.ID == "" && device.Name == "" {
				return fmt.Errorf("missing device name or id for suite: %s. Devices index: %d", suite.Name, didx)
			}

			if device.Options.DeviceType != "" && !config.IsSupportedDeviceType(device.Options.DeviceType) {
				return fmt.Errorf("deviceType: %s is unsupported for suite: %s. Devices index: %d. Supported device types: %s",
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
	return fmt.Errorf("no suite named '%s' found", suiteName)
}
