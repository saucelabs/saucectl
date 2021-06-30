package espresso

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/region"
	"os"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
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
	NotClass         []string `yaml:"notClass,omitempty" json:"notClass"`
	Class            []string `yaml:"class,omitempty" json:"class"`
	Package          string   `yaml:"package,omitempty" json:"package"`
	Size             string   `yaml:"size,omitempty" json:"size"`
	Annotation       string   `yaml:"annotation,omitempty" json:"annotation"`
	ShardIndex       int      `json:"shardIndex"`
	NumShards        int      `yaml:"numShards,omitempty" json:"numShards"`
	ClearPackageData bool     `yaml:"clearPackageData,omitempty" json:"clearPackageData"`
}

// Suite represents the espresso test suite configuration.
type Suite struct {
	Name        string            `yaml:"name,omitempty" json:"name"`
	Devices     []config.Device   `yaml:"devices,omitempty" json:"devices"`
	Emulators   []config.Emulator `yaml:"emulators,omitempty" json:"emulators"`
	TestOptions TestOptions       `yaml:"testOptions,omitempty" json:"testOptions"`
}

// Android constant
const Android = "Android"

// FromFile creates a new cypress Project based on the filepath cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	if cfgPath == "" {
		return Project{}, nil
	}

	f, err := os.Open(cfgPath)
	if err != nil {
		return Project{}, fmt.Errorf("failed to locate project config: %v", err)
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&p); err != nil {
		return Project{}, fmt.Errorf("failed to parse project config: %v", err)
	}
	p.ConfigFilePath = cfgPath

	p.Espresso.App = os.ExpandEnv(p.Espresso.App)
	p.Espresso.TestApp = os.ExpandEnv(p.Espresso.TestApp)

	if p.Kind != Kind && p.APIVersion != APIVersion {
		return p, config.ErrUnknownCfg
	}

	return p, nil
}

// SetDefaults applies config defaults in case the user has left them blank.
func SetDefaults(p *Project) {
	if p.Sauce.Concurrency < 1 {
		p.Sauce.Concurrency = 2
	}

	for i, suite := range p.Suites {
		for j := range suite.Devices {
			// Android is the only choice.
			p.Suites[i].Devices[j].PlatformName = Android
			p.Suites[i].Devices[j].Options.DeviceType = strings.ToUpper(p.Suites[i].Devices[j].Options.DeviceType)
		}
		for j := range suite.Emulators {
			p.Suites[i].Emulators[j].PlatformName = Android
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

	for _, suite := range p.Suites {
		if len(suite.Devices) == 0 && len(suite.Emulators) == 0 {
			return fmt.Errorf("missing devices or emulators configuration for suite: %s", suite.Name)
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
			return fmt.Errorf("missing device name or ID for suite: %s. Devices index: %d", suiteName, didx)
		}
		if device.Options.DeviceType != "" && !config.IsSupportedDeviceType(device.Options.DeviceType) {
			return fmt.Errorf("deviceType: %s is unsupported for suite: %s. Devices index: %d. Supported device types: %s",
				device.Options.DeviceType, suiteName, didx, strings.Join(config.SupportedDeviceTypes, ","))
		}
	}
	return nil
}

func validateEmulators(suiteName string, emulators []config.Emulator) error {
	for eidx, emulator := range emulators {
		if emulator.Name == "" {
			return fmt.Errorf("missing emulator name for suite: %s. Emulators index: %d", suiteName, eidx)
		}
		if !strings.Contains(strings.ToLower(emulator.Name), "emulator") {
			return fmt.Errorf("missing `emulator` in emulator name: %s. Suite name: %s. Emulators index: %d", emulator.Name, suiteName, eidx)
		}
		if len(emulator.PlatformVersions) == 0 {
			return fmt.Errorf("missing platform versions for emulator: %s. Suite name: %s. Emulators index: %d", emulator.Name, suiteName, eidx)
		}
	}
	return nil
}
