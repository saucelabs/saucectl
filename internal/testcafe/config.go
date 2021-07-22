package testcafe

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"os"
	"regexp"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "testcafe"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// appleDeviceRegex is a device name matching regex for apple devices (mainly ipad/iphone).
var appleDeviceRegex = regexp.MustCompile(`(?i)(iP)(hone|ad)[\w\s\d]*(Simulator)?`)

// Project represents the testcafe project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	DryRun         bool `yaml:"-" json:"-"`
	ShowConsoleLog bool
	ConfigFilePath string                 `yaml:"-" json:"-"`
	CommandLine    map[string]interface{} `yaml:"-" json:"-"`
	Sauce          config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite             `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite           `yaml:"suites,omitempty" json:"suites"`
	BeforeExec    []string          `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker        config.Docker     `yaml:"docker,omitempty" json:"docker"`
	Testcafe      Testcafe          `yaml:"testcafe,omitempty" json:"testcafe"`
	Npm           config.Npm        `yaml:"npm,omitempty" json:"npm"`
	RootDir       string            `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string            `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts  `yaml:"artifacts,omitempty" json:"artifacts"`
	Defaults      config.Defaults   `yaml:"defaults,omitempty" json:"defaults"`
	Env           map[string]string `yaml:"env,omitempty" json:"env"`
}

// Suite represents the testcafe test suite configuration.
type Suite struct {
	Name             string            `yaml:"name,omitempty" json:"name"`
	BrowserName      string            `yaml:"browserName,omitempty" json:"browserName"`
	BrowserVersion   string            `yaml:"browserVersion,omitempty" json:"browserVersion"`
	Src              []string          `yaml:"src,omitempty" json:"src"`
	Screenshots      Screenshots       `yaml:"screenshots,omitempty" json:"screenshots"`
	PlatformName     string            `yaml:"platformName,omitempty" json:"platformName"`
	ScreenResolution string            `yaml:"screenResolution,omitempty" json:"screenResolution"`
	Env              map[string]string `yaml:"env,omitempty" json:"env"`
	// Deprecated as of TestCafe v1.10.0 https://testcafe.io/documentation/402638/reference/configuration-file#tsconfigpath
	TsConfigPath       string   `yaml:"tsConfigPath,omitempty" json:"tsConfigPath"`
	ClientScripts      []string `yaml:"clientScripts,omitempty" json:"clientScripts,omitempty"`
	SkipJsErrors       bool     `yaml:"skipJsErrors,omitempty" json:"skipJsErrors"`
	QuarantineMode     bool     `yaml:"quarantineMode,omitempty" json:"quarantineMode"`
	SkipUncaughtErrors bool     `yaml:"skipUncaughtErrors,omitempty" json:"skipUncaughtErrors"`
	SelectorTimeout    int      `yaml:"selectorTimeout,omitempty" json:"selectorTimeout"`
	AssertionTimeout   int      `yaml:"assertionTimeout,omitempty" json:"assertionTimeout"`
	PageLoadTimeout    int      `yaml:"pageLoadTimeout,omitempty" json:"pageLoadTimeout"`
	Speed              float64  `yaml:"speed,omitempty" json:"speed"`
	StopOnFirstFail    bool     `yaml:"stopOnFirstFail,omitempty" json:"stopOnFirstFail"`
	DisablePageCaching bool     `yaml:"disablePageCaching,omitempty" json:"disablePageCaching"`
	DisableScreenshots bool     `yaml:"disableScreenshots,omitempty" json:"disableScreenshots"`
	DisableVideo       bool     `yaml:"disableVideo,omitempty" json:"disableVideo"` // This field is for sauce, not for native testcafe config.
	Mode               string   `yaml:"mode,omitempty" json:"-"`
	// Deprecated. Reserved for future use for actual devices.
	Devices    []config.Simulator `yaml:"devices,omitempty" json:"devices"`
	Simulators []config.Simulator `yaml:"emulators,omitempty" json:"emulators"`
}

// Screenshots represents screenshots configuration.
type Screenshots struct {
	TakeOnFails bool `yaml:"takeOnFails,omitempty" json:"takeOnFails"`
	FullPage    bool `yaml:"fullPage,omitempty" json:"fullPage"`
}

// Testcafe represents the configuration for testcafe.
type Testcafe struct {
	// Version represents the testcafe framework version.
	Version string `yaml:"version,omitempty" json:"version"`
}

// FromFile creates a new testcafe project based on the filepath.
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
		// Default concurrency is 2
		p.Sauce.Concurrency = 2
	}

	// Set default docker file transfer to mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}

	// Default rootDir to .
	if p.RootDir == "" {
		p.RootDir = "."
		msg.LogRootDirWarning()
	}

	for k := range p.Suites {
		suite := &p.Suites[k]
		// If value is 0, it's assumed that the value has not been filled.
		// So we define it to the default value: 1 (full speed).
		// Expected values for TestCafe are between .01 and 1.
		if suite.Speed < .01 || suite.Speed > 1 {
			suite.Speed = 1
		}
		// Set default timeout. ref: https://devexpress.github.io/testcafe/documentation/reference/configuration-file.html#selectortimeout
		if suite.SelectorTimeout <= 0 {
			suite.SelectorTimeout = 10000
		}
		if suite.AssertionTimeout <= 0 {
			suite.AssertionTimeout = 3000
		}
		if suite.PageLoadTimeout <= 0 {
			suite.PageLoadTimeout = 3000
		}

		// If this suite is targeting devices, then the platformName on the device takes precedence and we can skip the
		// defaults on the suite level.
		if suite.PlatformName == "" && len(suite.Simulators) == 0 {
			suite.PlatformName = "Windows 10"

			if strings.ToLower(suite.BrowserName) == "safari" {
				suite.PlatformName = "macOS 11.00"
			}
		}

		for j := range suite.Simulators {
			sim := &suite.Simulators[j]
			if sim.PlatformName == "" && appleDeviceRegex.MatchString(sim.Name) {
				sim.PlatformName = "iOS"
			}
		}
	}

	// Apply global env vars onto every suite.
	for k, v := range p.Env {
		for ks := range p.Suites {
			s := &p.Suites[ks]
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = os.ExpandEnv(v)
		}
	}
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p *Project) error {
	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New("no sauce region set")
	}

	p.Testcafe.Version = config.StandardizeVersionFormat(p.Testcafe.Version)
	if p.Testcafe.Version == "" {
		return errors.New("missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}

	for _, v := range p.Suites {
		// Force the user to migrate.
		if len(v.Devices) != 0 {
			return errors.New("the 'devices' keyword in your config is now reserved for real devices, please use 'simulators' instead")
		}
	}

	return nil
}

// SplitSuites divided Suites to dockerSuites and sauceSuites
func SplitSuites(p Project) (Project, Project) {
	var dockerSuites []Suite
	var sauceSuites []Suite
	for _, s := range p.Suites {
		if s.Mode == "docker" || (s.Mode == "" && p.Defaults.Mode == "docker") {
			dockerSuites = append(dockerSuites, s)
		} else {
			sauceSuites = append(sauceSuites, s)
		}
	}

	dockerProject := p
	dockerProject.Suites = dockerSuites
	sauceProject := p
	sauceProject.Suites = sauceSuites

	return dockerProject, sauceProject
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
