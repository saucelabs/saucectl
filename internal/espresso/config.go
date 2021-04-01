package espresso

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
	"os"
)

// Project represents the espresso project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ShowConsoleLog bool
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Espresso       Espresso           `yaml:"cypress,omitempty" json:"cypress"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
}

// Project represents the espresso apps configuration.
type Espresso struct {
	App		string	`yaml:"app,omitempty" json:"app"`
	TestApp	string	`yaml:"testApp,omitempty" json:"testApp"`
}

// TestOptions represents the espresso test filter options configuration.
type TestOptions struct {
	NotClass	[]string	`yaml:"notClass,omitempty" json:"notClass"`
	Class	   	[]string	`yaml:"class,omitempty" json:"class"`
	Package		string		`yaml:"package,omitempty" json:"package"`
	Size		string		`yaml:"size,omitempty" json:"size"`
	Annotation	string		`yaml:"annotation,omitempty" json:"annotation"`
}

// Suite represents the espresso test suite configuration.
type Suite struct {
	Name             string         `yaml:"name,omitempty" json:"name"`
	PlatformName     string         `yaml:"platformName,omitempty" json:"platformName"`
	PlatformVersion  string         `yaml:"platformVersion,omitempty" json:"platformVersion"`
	Device           config.Device	`yaml:"device,omitempty" json:"device"`
	TestOptions      TestOptions	`yaml:"testOptions,omitempty" json:"testOptions"`
}

const Android = "Android"

// FromFile creates a new cypress Project based on the filepath cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	f, err := os.Open(cfgPath)
	defer f.Close()
	if err != nil {
		return Project{}, fmt.Errorf("failed to locate project config: %v", err)
	}

	if err = yaml.NewDecoder(f).Decode(&p); err != nil {
		return Project{}, fmt.Errorf("failed to parse project config: %v", err)
	}

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

	if p.Espresso.TestApp == "" {
		return errors.New("missing path to test app .apk")
	}

	if len(p.Suites) == 0 {
		return errors.New("no suites defined")
	}

	for idx, suite := range p.Suites {
		if suite.Device.Name == "" || suite.Device.Id == "" {
			return fmt.Errorf("missing device for suite: %s", suite.Name)
		}

		if suite.PlatformVersion == "" {
			return fmt.Errorf("missing platform version for suite: %s. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers", suite.Name)
		}

		if suite.PlatformName == "" || suite.PlatformName != Android {
			p.Suites[idx].PlatformName = Android
		}
	}

	return nil
}