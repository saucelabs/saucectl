package espresso

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
)

// Project represents the espresso project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Espresso       Espresso           `yaml:"espresso,omitempty" json:"espresso"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
}

// Espresso represents espresso apps configuration.
type Espresso struct {
	App     string `yaml:"app,omitempty" json:"app"`
	TestApp string `yaml:"testApp,omitempty" json:"testApp"`
}

// TestOptions represents the espresso test filter options configuration.
type TestOptions struct {
	NotClass   []string `yaml:"notClass,omitempty" json:"notClass"`
	Class      []string `yaml:"class,omitempty" json:"class"`
	Package    string   `yaml:"package,omitempty" json:"package"`
	Size       string   `yaml:"size,omitempty" json:"size"`
	Annotation string   `yaml:"annotation,omitempty" json:"annotation"`
}

// Suite represents the espresso test suite configuration.
type Suite struct {
	Name        string          `yaml:"name,omitempty" json:"name"`
	Devices     []config.Device `yaml:"devices,omitempty" json:"devices"`
	TestOptions TestOptions     `yaml:"testOptions,omitempty" json:"testOptions"`
}

// Android constant
const Android = "Android"

// FromFile creates a new cypress Project based on the filepath cfgPath.
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
	if p.Espresso.App == "" {
		return errors.New("missing path to app .apk")
	}
	if !strings.HasSuffix(p.Espresso.App, ".apk") {
		return fmt.Errorf("invaild application file: %s, make sure extension is .apk", p.Espresso.App)
	}

	if p.Espresso.TestApp == "" {
		return errors.New("missing path to test app .apk")
	}
	if !strings.HasSuffix(p.Espresso.TestApp, ".apk") {
		return fmt.Errorf("invaild test application file: %s, make sure extension is .apk", p.Espresso.TestApp)
	}

	if len(p.Suites) == 0 {
		return errors.New("no suites defined")
	}

	for sidx, suite := range p.Suites {
		if len(suite.Devices) == 0 {
			return fmt.Errorf("missing devices configuration for suite: %s", suite.Name)
		}
		for didx, device := range suite.Devices {
			if device.Name == "" {
				return fmt.Errorf("missing device name for suite: %s. Devices index: %d", suite.Name, didx)
			}
			if !strings.Contains(strings.ToLower(device.Name), "emulator") {
				return fmt.Errorf("missing `emulator` in device name: %s, real device cloud is unsupported right now", device.Name)
			}
			if len(device.PlatformVersions) == 0 {
				// TODO - update message when handling device.Id
				return fmt.Errorf("missing platform versions for device: %s", device.Name)
			}
			if device.PlatformName == "" || device.PlatformName != Android {
				p.Suites[sidx].Devices[didx].PlatformName = Android
			}
		}
	}

	return nil
}
