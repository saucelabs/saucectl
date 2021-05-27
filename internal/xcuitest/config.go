package xcuitest

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
)

var supportedDeviceTypes = []string{"ANY", "PHONE", "TABLET"}

// Project represents the xcuitest project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Xcuitest       Xcuitest           `yaml:"xcuitest,omitempty" json:"xcuitest"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
}

// Xcuitest represents xcuitest apps configuration.
type Xcuitest struct {
	App     string `yaml:"app,omitempty" json:"app"`
	TestApp string `yaml:"testApp,omitempty" json:"testApp"`
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

	f, err := os.Open(cfgPath)
	if err != nil {
		return Project{}, fmt.Errorf("failed to locate project config: %v", err)
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&p); err != nil {
		return Project{}, fmt.Errorf("failed to parse project config: %v", err)
	}
	p.ConfigFilePath = cfgPath

	if p.Sauce.Concurrency < 1 {
		// Default concurrency is 2
		p.Sauce.Concurrency = 2
	}

	return p, nil
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p Project) error {
	if p.Xcuitest.App == "" {
		return errors.New("missing path to app .ipa")
	}
	if !strings.HasSuffix(p.Xcuitest.App, ".ipa") {
		return fmt.Errorf("invalid application file: %s, make sure extension is .ipa", p.Xcuitest.App)
	}

	if p.Xcuitest.TestApp == "" {
		return errors.New("missing path to test app .ipa")
	}
	if !strings.HasSuffix(p.Xcuitest.TestApp, ".ipa") {
		return fmt.Errorf("invalid application test file: %s, make sure extension is .ipa", p.Xcuitest.TestApp)
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

			if device.Options.DeviceType != "" && !isSupportedDeviceType(device.Options.DeviceType) {
				return fmt.Errorf("deviceType: %s is unsupported for suite: %s. Devices index: %d. Supported device types: %s",
					device.Options.DeviceType, suite.Name, didx, strings.Join(supportedDeviceTypes, ","))
			}
		}
	}

	return nil
}

// SetDeviceDefaultValues sets device default values.
func SetDeviceDefaultValues(p *Project) {
	for _, suite := range p.Suites {
		for _, device := range suite.Devices {
			device.PlatformName = "iOS"

			// device type only supports uppercase values
			device.Options.DeviceType = strings.ToUpper(device.Options.DeviceType)
		}
	}
}

func isSupportedDeviceType(deviceType string) bool {
	for _, dt := range supportedDeviceTypes {
		if dt == deviceType {
			return true
		}
	}

	return false
}
