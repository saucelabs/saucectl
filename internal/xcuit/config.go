package xcuit

import (
	"github.com/saucelabs/saucectl/internal/config"
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
