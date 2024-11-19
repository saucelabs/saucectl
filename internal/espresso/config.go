package espresso

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/insights"
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
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	Defaults       config.Defaults        `yaml:"defaults" json:"defaults"`
	ShowConsoleLog bool                   `yaml:"showConsoleLog" json:"-"`
	DryRun         bool                   `yaml:"-" json:"-"`
	ConfigFilePath string                 `yaml:"-" json:"-"`
	CLIFlags       map[string]interface{} `yaml:"-" json:"-"`
	Sauce          config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	Espresso       Espresso               `yaml:"espresso,omitempty" json:"espresso"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite     Suite            `yaml:"suite,omitempty" json:"-"`
	Suites    []Suite          `yaml:"suites,omitempty" json:"suites"`
	Artifacts config.Artifacts `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters config.Reporters `yaml:"reporters,omitempty" json:"-"`
}

// Espresso represents espresso apps configuration.
type Espresso struct {
	App                string   `yaml:"app,omitempty" json:"app"`
	AppDescription     string   `yaml:"appDescription,omitempty" json:"appDescription"`
	TestApp            string   `yaml:"testApp,omitempty" json:"testApp"`
	TestAppDescription string   `yaml:"testAppDescription,omitempty" json:"testAppDescription"`
	OtherApps          []string `yaml:"otherApps,omitempty" json:"otherApps"`
}

// ShardConfig represents the configuration for sharding. The config values come
// from Suite.TestOptions.
type ShardConfig struct {
	Shards int
	Index  int
}

// Suite represents the espresso test suite configuration.
type Suite struct {
	Name               string                 `yaml:"name,omitempty" json:"name"`
	TestApp            string                 `yaml:"testApp,omitempty" json:"testApp"`
	TestAppDescription string                 `yaml:"testAppDescription,omitempty" json:"testAppDescription"`
	Devices            []config.Device        `yaml:"devices,omitempty" json:"devices"`
	Emulators          []config.Emulator      `yaml:"emulators,omitempty" json:"emulators"`
	TestOptions        map[string]interface{} `yaml:"testOptions,omitempty" json:"testOptions"`
	Timeout            time.Duration          `yaml:"timeout,omitempty" json:"timeout"`
	AppSettings        config.AppSettings     `yaml:"appSettings,omitempty" json:"appSettings"`
	PassThreshold      int                    `yaml:"passThreshold,omitempty" json:"-"`
	SmartRetry         config.SmartRetry      `yaml:"smartRetry,omitempty" json:"-"`
}

func (s *Suite) ShardConfig() ShardConfig {
	shards := 0
	index := 0

	if numShards, ok := s.TestOptions["numShards"]; ok {
		if v, err := strconv.Atoi(fmt.Sprintf("%v", numShards)); err == nil {
			shards = v
		}
	}

	if shardIndex, ok := s.TestOptions["shardIndex"]; ok {
		if v, err := strconv.Atoi(fmt.Sprintf("%v", shardIndex)); err == nil {
			index = v
		}
	}

	return ShardConfig{
		Shards: shards,
		Index:  index,
	}
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

	for i := range p.Suites {
		suite := &p.Suites[i]

		for j := range suite.Devices {
			// Android is the only choice.
			suite.Devices[j].PlatformName = Android
			suite.Devices[j].Options.DeviceType = strings.ToUpper(p.Suites[i].Devices[j].Options.DeviceType)
		}
		for j := range suite.Emulators {
			suite.Emulators[j].PlatformName = Android
		}

		if suite.Timeout <= 0 {
			suite.Timeout = p.Defaults.Timeout
		}

		if suite.TestApp == "" {
			suite.TestApp = p.Espresso.TestApp
			suite.TestAppDescription = p.Espresso.TestAppDescription
		}
		if suite.PassThreshold < 1 {
			suite.PassThreshold = 1
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

	if ok := config.ValidateVisibility(p.Sauce.Visibility); !ok {
		return fmt.Errorf(msg.InvalidVisibility, p.Sauce.Visibility, strings.Join(config.ValidVisibilityValues, ","))
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

	if p.Sauce.LaunchOrder != "" && p.Sauce.LaunchOrder != config.LaunchOrderFailRate {
		return fmt.Errorf(msg.InvalidLaunchingOption, p.Sauce.LaunchOrder, string(config.LaunchOrderFailRate))
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
		if regio == region.USEast4 && len(suite.Emulators) > 0 {
			return errors.New(msg.NoEmulatorSupport)
		}
		if p.Sauce.Retries < suite.PassThreshold-1 {
			return fmt.Errorf(msg.InvalidPassThreshold)
		}
		config.ValidateSmartRetry(suite.SmartRetry)
		if v, ok := suite.TestOptions["numShards"]; ok {
			_, err := strconv.Atoi(fmt.Sprintf("%v", v))
			if err != nil {
				return fmt.Errorf("invalid numShards in test option: %v", err)
			}
		}
	}
	if p.Sauce.Retries < 0 {
		log.Warn().Int("retries", p.Sauce.Retries).Msg(msg.InvalidReries)
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

func GetShardTypes(suites []Suite) []string {
	var set = map[string]bool{}
	for _, suite := range suites {
		if v, ok := suite.TestOptions["numShards"]; ok {
			num, _ := strconv.Atoi(fmt.Sprintf("%v", v))
			set["numShards"] = num > 0
		}
	}
	var values []string
	for k := range set {
		values = append(values, k)
	}
	return values
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

// IsSmartRetried checks if the suites contain a smartRetried suite
func (p *Project) IsSmartRetried() bool {
	for _, s := range p.Suites {
		if s.SmartRetry.IsRetryFailedOnly() {
			return true
		}
	}
	return false
}
