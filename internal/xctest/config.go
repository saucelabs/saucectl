package xctest

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "xctest"

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
	Xctest         Xctest                 `yaml:"xctest,omitempty" json:"xctest"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite     Suite             `yaml:"suite,omitempty" json:"-"`
	Suites    []Suite           `yaml:"suites,omitempty" json:"suites"`
	Artifacts config.Artifacts  `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters config.Reporters  `yaml:"reporters,omitempty" json:"-"`
	Env       map[string]string `yaml:"env,omitempty" json:"-"`
	EnvFlag   map[string]string `yaml:"-" json:"-"`
}

// Xctest represents xctest apps configuration.
type Xctest struct {
	App                      string   `yaml:"app,omitempty" json:"app"`
	AppDescription           string   `yaml:"appDescription,omitempty" json:"appDescription"`
	XCTestRunFile            string   `yaml:"xcTestRunFile,omitempty" json:"xcTestRunFile"`
	XCTestRunFileDescription string   `yaml:"xcTestRunFileDescription,omitempty" json:"xcTestRunFileDescription"`
	OtherApps                []string `yaml:"otherApps,omitempty" json:"otherApps"`
}

// TestOptions represents the xcuitest test filter options configuration.
type TestOptions struct {
	NotClass                          []string `yaml:"notClass,omitempty" json:"notClass"`
	Class                             []string `yaml:"class,omitempty" json:"class"`
	TestLanguage                      string   `yaml:"testLanguage,omitempty" json:"testLanguage"`
	TestRegion                        string   `yaml:"testRegion,omitempty" json:"testRegion"`
	TestTimeoutsEnabled               string   `yaml:"testTimeoutsEnabled,omitempty" json:"testTimeoutsEnabled"`
	MaximumTestExecutionTimeAllowance int      `yaml:"maximumTestExecutionTimeAllowance,omitempty" json:"maximumTestExecutionTimeAllowance"`
	DefaultTestExecutionTimeAllowance int      `yaml:"defaultTestExecutionTimeAllowance,omitempty" json:"defaultTestExecutionTimeAllowance"`
	StatusBarOverrideTime             string   `yaml:"statusBarOverrideTime,omitempty" json:"statusBarOverrideTime"`
}

// ToMap converts the TestOptions to a map where the keys are derived from json struct tags.
func (t TestOptions) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	v := reflect.ValueOf(t)
	tt := v.Type()

	count := v.NumField()
	for i := 0; i < count; i++ {
		if v.Field(i).CanInterface() {
			tag := tt.Field(i).Tag
			tname, ok := tag.Lookup("json")
			if ok && tname != "-" {
				fv := v.Field(i).Interface()
				ft := v.Field(i).Type()
				switch ft.Kind() {
				// Convert int to string to match chef expectation that all test option values are strings
				case reflect.Int:
					// Conventionally, test options with value "" will be ignored.
					if fv.(int) == 0 {
						m[tname] = ""
					} else {
						m[tname] = fmt.Sprintf("%v", fv)
					}
				default:
					m[tname] = fv
				}
			}
		}
	}
	return m
}

// Suite represents the xcuitest test suite configuration.
type Suite struct {
	Name                     string             `yaml:"name,omitempty" json:"name"`
	App                      string             `yaml:"app,omitempty" json:"app"`
	AppDescription           string             `yaml:"appDescription,omitempty" json:"appDescription"`
	XCTestRunFile            string             `yaml:"xcTestRunFile,omitempty" json:"xcTestRunFile"`
	XCTestRunFileDescription string             `yaml:"xcTestRunFileDescription,omitempty" json:"xcTestRunFileDescription"`
	OtherApps                []string           `yaml:"otherApps,omitempty" json:"otherApps"`
	Timeout                  time.Duration      `yaml:"timeout,omitempty" json:"timeout"`
	Devices                  []config.Device    `yaml:"devices,omitempty" json:"devices"`
	Simulators               []config.Simulator `yaml:"simulators,omitempty" json:"simulators"`
	TestOptions              TestOptions        `yaml:"testOptions,omitempty" json:"testOptions"`
	AppSettings              config.AppSettings `yaml:"appSettings,omitempty" json:"appSettings"`
	PassThreshold            int                `yaml:"passThreshold,omitempty" json:"-"`
	SmartRetry               config.SmartRetry  `yaml:"smartRetry,omitempty" json:"-"`
	Shard                    string             `yaml:"shard,omitempty" json:"-"`
	TestListFile             string             `yaml:"testListFile,omitempty" json:"-"`
	Env                      map[string]string  `yaml:"env,omitempty" json:"-"`
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

	for i := range p.Suites {
		suite := &p.Suites[i]

		for id := range suite.Devices {
			suite.Devices[id].PlatformName = "iOS"

			// device type only supports uppercase values
			suite.Devices[id].Options.DeviceType = strings.ToUpper(suite.Devices[id].Options.DeviceType)
		}
		for id := range suite.Simulators {
			suite.Simulators[id].PlatformName = "iOS"
		}

		if suite.Timeout <= 0 {
			suite.Timeout = p.Defaults.Timeout
		}

		if suite.XCTestRunFile == "" {
			suite.XCTestRunFile = p.Xctest.XCTestRunFile
			suite.XCTestRunFileDescription = p.Xctest.XCTestRunFileDescription
		}
		if suite.App == "" {
			suite.App = p.Xctest.App
			suite.AppDescription = p.Xctest.AppDescription
		}
		if len(suite.OtherApps) == 0 {
			suite.OtherApps = append(suite.OtherApps, p.Xctest.OtherApps...)
		}
		if suite.PassThreshold < 1 {
			suite.PassThreshold = 1
		}

		// Precedence: --env flag > root-level env vars > suite-level env vars.
		for _, env := range []map[string]string{p.Env, p.EnvFlag} {
			for k, v := range env {
				if suite.Env == nil {
					suite.Env = map[string]string{}
				}
				suite.Env[k] = v
			}
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

	if p.Sauce.LaunchOrder != "" && p.Sauce.LaunchOrder != config.LaunchOrderFailRate {
		return fmt.Errorf(msg.InvalidLaunchingOption, p.Sauce.LaunchOrder, string(config.LaunchOrderFailRate))
	}

	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
	}

	for _, suite := range p.Suites {
		if len(suite.Devices) == 0 && len(suite.Simulators) == 0 {
			return fmt.Errorf(msg.MissingXcuitestDeviceConfig, suite.Name)
		}
		if len(suite.Devices) > 0 && len(suite.Simulators) > 0 {
			return fmt.Errorf("suite cannot have both simulators and devices")
		}

		validAppExt := []string{".app"}
		if len(suite.Devices) > 0 {
			validAppExt = append(validAppExt, ".ipa")
		} else if len(suite.Simulators) > 0 {
			validAppExt = append(validAppExt, ".zip")
		}
		if suite.App == "" {
			return errors.New(msg.MissingXcuitestAppPath)
		}
		if err := apps.Validate("application", suite.App, validAppExt); err != nil {
			return err
		}

		if suite.XCTestRunFile == "" {
			return errors.New(msg.MissingXCTestFileAppPath)
		}
		if err := apps.Validate("test configuration", suite.XCTestRunFile, []string{".xctestrun"}); err != nil {
			return err
		}

		for _, app := range suite.OtherApps {
			if err := apps.Validate("other application", app, validAppExt); err != nil {
				return err
			}
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
		if p.Sauce.Retries < suite.PassThreshold-1 {
			return fmt.Errorf(msg.InvalidPassThreshold)
		}
		config.ValidateSmartRetry(suite.SmartRetry)
	}
	if p.Sauce.Retries < 0 {
		log.Warn().Int("retries", p.Sauce.Retries).Msg(msg.InvalidReries)
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

// ShardSuites applies sharding by provided testListFile.
func ShardSuites(p *Project) error {
	var suites []Suite
	for _, s := range p.Suites {
		if s.Shard == "concurrency" {
			shardedSuites, err := shardByConcurrency(s, p.Sauce.Concurrency)
			if err != nil {
				return fmt.Errorf("failed to get tests from testListFile(%q): %v", s.TestListFile, err)
			}
			suites = append(suites, shardedSuites...)
		} else if s.Shard == "testList" {
			shardedSuites, err := shardByTestList(s)
			if err != nil {
				return fmt.Errorf("failed to get tests from testListFile(%q): %v", s.TestListFile, err)
			}
			suites = append(suites, shardedSuites...)
		} else {
			suites = append(suites, s)
		}
	}
	p.Suites = suites

	return nil
}

func shardByConcurrency(suite Suite, ccy int) ([]Suite, error) {
	readFile, err := os.Open(suite.TestListFile)
	if err != nil {
		return nil, err
	}
	defer readFile.Close()

	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)
	var tests []string
	for fileScanner.Scan() {
		text := strings.TrimSpace(fileScanner.Text())
		if text == "" {
			continue
		}
		tests = append(tests, text)
	}
	if len(tests) == 0 {
		return nil, errors.New("empty file")
	}

	buckets := concurrency.BinPack(tests, ccy)
	var suites []Suite
	for i, b := range buckets {
		currSuite := suite
		currSuite.Name = fmt.Sprintf("%s - %d/%d", suite.Name, i+1, len(buckets))
		currSuite.TestOptions.Class = b
		suites = append(suites, currSuite)
	}
	return suites, nil
}

func shardByTestList(suite Suite) ([]Suite, error) {
	readFile, err := os.Open(suite.TestListFile)
	if err != nil {
		return nil, err
	}
	defer readFile.Close()

	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)
	var tests []string
	for fileScanner.Scan() {
		text := strings.TrimSpace(fileScanner.Text())
		if text == "" {
			continue
		}
		tests = append(tests, text)
	}
	if len(tests) == 0 {
		return nil, errors.New("empty file")
	}
	var suites []Suite
	for _, t := range tests {
		currSuite := suite
		currSuite.Name = fmt.Sprintf("%s - %s", suite.Name, t)
		currSuite.TestOptions.Class = []string{t}
		suites = append(suites, currSuite)
	}
	return suites, nil
}

func GetShardTypes(suites []Suite) []string {
	var set = map[string]bool{}
	for _, s := range suites {
		if s.Shard != "" {
			set[s.Shard] = true
		}
	}
	var values []string
	for k := range set {
		values = append(values, k)
	}
	return values
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
