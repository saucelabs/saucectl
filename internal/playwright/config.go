package playwright

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/region"
	"os"
	"strings"

	"github.com/saucelabs/saucectl/internal/config"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "playwright"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

var supportedBrwsList = []string{"chromium", "firefox", "webkit"}

// Project represents the playwright project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ShowConsoleLog bool
	ConfigFilePath string             `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Playwright     Playwright         `yaml:"playwright,omitempty" json:"playwright"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite             `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite           `yaml:"suites,omitempty" json:"suites"`
	BeforeExec    []string          `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker        config.Docker     `yaml:"docker,omitempty" json:"docker"`
	Npm           config.Npm        `yaml:"npm,omitempty" json:"npm"`
	RootDir       string            `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string            `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts  `yaml:"artifacts,omitempty" json:"artifacts"`
	Defaults      config.Defaults   `yaml:"defaults,omitempty" json:"defaults"`
	Env           map[string]string `yaml:"env,omitempty" json:"env"`
}

// Playwright represents crucial playwright configuration that is required for setting up a project.
type Playwright struct {
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// Suite represents the playwright test suite configuration.
type Suite struct {
	Name              string            `yaml:"name,omitempty" json:"name"`
	Mode              string            `yaml:"mode,omitempty" json:"-"`
	PlaywrightVersion string            `yaml:"playwrightVersion,omitempty" json:"playwrightVersion,omitempty"`
	TestMatch         []string          `yaml:"testMatch,omitempty" json:"testMatch,omitempty"`
	PlatformName      string            `yaml:"platformName,omitempty" json:"platformName,omitempty"`
	Params            SuiteConfig       `yaml:"params,omitempty" json:"param,omitempty"`
	ScreenResolution  string            `yaml:"screenResolution,omitempty" json:"screenResolution,omitempty"`
	Env               map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// SuiteConfig represents the configuration specific to a suite
type SuiteConfig struct {
	BrowserName string `yaml:"browserName,omitempty" json:"browserName,omitempty"`

	// Fields appeared in v1.12+
	Headed        bool   `yaml:"headed,omitempty" json:"headed,omitempty"`
	GlobalTimeout int    `yaml:"globalTimeout,omitempty" json:"globalTimeout,omitempty"`
	Timeout       int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Grep          string `yaml:"grep,omitempty" json:"grep,omitempty"`
	RepeatEach    int    `yaml:"repeatEach,omitempty" json:"repeatEach,omitempty"`
	Retries       int    `yaml:"retries,omitempty" json:"retries,omitempty"`
	MaxFailures   int    `yaml:"maxFailures,omitempty" json:"maxFailures,omitempty"`
	Shard         string `yaml:"shard,omitempty" json:"shard,omitempty"`

	// Deprecated fields in v1.12+
	HeadFul             bool `yaml:"headful,omitempty" json:"headful,omitempty"`
	ScreenshotOnFailure bool `yaml:"screenshotOnFailure,omitempty" json:"screenshotOnFailure,omitempty"`
	SlowMo              int  `yaml:"slowMo,omitempty" json:"slowMo,omitempty"`
	Video               bool `yaml:"video,omitempty" json:"video,omitempty"`
}

// FromFile creates a new playwright Project based on the filepath cfgPath.
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

	// Set default docker file transfer to mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}

	// Apply global env vars onto every suite.
	for k, v := range p.Env {
		for ks := range p.Suites {
			s := &p.Suites[ks]
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = os.ExpandEnv(v)

			if s.PlatformName == "" {
				s.PlatformName = "Windows 10"
			}
		}
	}
}

// SplitSuites divided Suites to dockerSuites and sauceSuites
func SplitSuites(p Project) (Project, Project) {
	var dockerSuites []Suite
	var sauceSuites []Suite
	for _, s := range p.Suites {
		if s.Mode == "docker" {
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

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p *Project) error {
	p.Playwright.Version = config.StandardizeVersionFormat(p.Playwright.Version)
	if p.Playwright.Version == "" {
		return errors.New("missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}

	// Check rootDir exists.
	if p.RootDir != "" {
		if _, err := os.Stat(p.RootDir); err != nil {
			return fmt.Errorf("unable to locate the rootDir folder %s", p.RootDir)
		}
	}

	if err := checkSupportedBrowsers(p); err != nil {
		return err
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New("no sauce region set")
	}

	return nil
}

func checkSupportedBrowsers(p *Project) error {
	errMsg := "browserName: %s is not supported. List of supported browsers: %s"

	for _, suite := range p.Suites {
		if suite.Params.BrowserName != "" && !isSupportedBrowser(suite.Params.BrowserName) {
			return fmt.Errorf(errMsg, suite.Params.BrowserName, strings.Join(supportedBrwsList, ", "))
		}
	}

	return nil
}

func isSupportedBrowser(browser string) bool {
	for _, supportedBr := range supportedBrwsList {
		if supportedBr == browser {
			return true
		}
	}

	return false
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
