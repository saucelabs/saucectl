package xcuit

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
)

type deviceType string

const (
	ANY    deviceType = "any"
	PHONE  deviceType = "phone"
	TABLET deviceType = "tablet"
)

// Project represents the xcuit project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Xcuit          Xcuit              `yaml:"xcuit,omitempty" json:"xcuit"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
}

// Xcuit represents xcuit apps configuration.
type Xcuit struct {
	App     string `yaml:"app,omitempty" json:"app"`
	TestApp string `yaml:"testApp,omitempty" json:"testApp"`
}

// TestOptions represents the xcuit test filter options configuration.
type TestOption struct {
	Class  string `yaml:"class,omitempty" json:"class"`
	Method string `yaml:"method,omitempty" json:"method"`
}

// Suite represents the xcuit test suite configuration.
type Suite struct {
	Name        string       `yaml:"name,omitempty" json:"name"`
	Devices     []Device     `yaml:"devices,omitempty" json:"devices"`
	TestOptions []TestOption `yaml:"testOptions,omitempty" json:"testOptions"`
}

// Device represents device configuration.
type Device struct {
	ID              string  `yaml:"id,omitempty" json:"id"`
	Name            string  `yaml:"name,omitempty" json:"name"`
	PlatformVersion string  `yaml:"platformVersion,omitempty" json:"platformVersion"`
	Options         Options `yaml:"options,omitempty" json:"options"`
}

// Options represents device options configuration.
type Options struct {
	CarrierConnectivity *bool      `yaml:"carrierConnectivity,omitempty" json:"carrierConnectivity"`
	DeviceType          deviceType `yaml:"deviceType,omitempty" json:"deviceType"`
	Private             *bool      `yaml:"private,omitempty" json:"private"`
}

// FromFile creates a new xcuit Project based on the filepath cfgPath.
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
	if p.Xcuit.App == "" {
		return errors.New("missing path to app .ipa")
	}
	if !strings.HasSuffix(p.Xcuit.App, ".ipa") {
		return fmt.Errorf("invaild application file: %s, make sure extension is .ipa", p.Xcuit.App)
	}
	_, err := os.Stat(p.Xcuit.App)
	if os.IsNotExist(err) {
		return fmt.Errorf("application file: %s does not exists", p.Xcuit.App)
	}

	if p.Xcuit.TestApp == "" {
		return errors.New("missing path to the bundle with tests")
	}
	_, err = os.Stat(p.Xcuit.TestApp)
	if os.IsNotExist(err) {
		return fmt.Errorf("bundle with tests: %s does not exists", p.Xcuit.TestApp)
	}

	if len(p.Suites) == 0 {
		return errors.New("no suites defined")
	}

	for _, suite := range p.Suites {
		if len(suite.Devices) == 0 {
			return fmt.Errorf("missing devices configuration for suite: %s", suite.Name)
		}
		//for didx, device := range suite.Devices {
		//	if device.Name == "" {
		//		return fmt.Errorf("missing device name for suite: %s. Devices index: %d", suite.Name, didx)
		//	}
		//	if !strings.Contains(strings.ToLower(device.Name), "emulator") {
		//		return fmt.Errorf("missing `emulator` in device name: %s, real device cloud is unsupported right now", device.Name)
		//	}
		//	if len(device.PlatformVersions) == 0 {
		//		// TODO - update message when handling device.Id
		//		return fmt.Errorf("missing platform versions for device: %s", device.Name)
		//	}
		//}
	}

	return nil
}
