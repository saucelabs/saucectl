package playwright

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
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
	ShowConsoleLog bool                   `yaml:"showConsoleLog" json:"-"`
	DryRun         bool                   `yaml:"-" json:"-"`
	ConfigFilePath string                 `yaml:"-" json:"-"`
	CLIFlags       map[string]interface{} `yaml:"-" json:"-"`
	Sauce          config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	Playwright     Playwright             `yaml:"playwright,omitempty" json:"playwright"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite                `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite              `yaml:"suites,omitempty" json:"suites"`
	BeforeExec    []string             `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker        config.Docker        `yaml:"docker,omitempty" json:"docker"`
	Npm           config.Npm           `yaml:"npm,omitempty" json:"npm"`
	RootDir       string               `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string               `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Defaults      config.Defaults      `yaml:"defaults,omitempty" json:"defaults"`
	Env           map[string]string    `yaml:"env,omitempty" json:"env"`
	Notifications config.Notifications `yaml:"notifications,omitempty" json:"-"`
}

// Playwright represents crucial playwright configuration that is required for setting up a project.
type Playwright struct {
	Version    string `yaml:"version,omitempty" json:"version,omitempty"`
	ConfigFile string `yaml:"configFile,omitempty" json:"configFile,omitempty"`
}

// Suite represents the playwright test suite configuration.
type Suite struct {
	Name              string            `yaml:"name,omitempty" json:"name"`
	Mode              string            `yaml:"mode,omitempty" json:"-"`
	Timeout           time.Duration     `yaml:"timeout,omitempty" json:"timeout"`
	PlaywrightVersion string            `yaml:"playwrightVersion,omitempty" json:"playwrightVersion,omitempty"`
	TestMatch         []string          `yaml:"testMatch,omitempty" json:"testMatch,omitempty"`
	PlatformName      string            `yaml:"platformName,omitempty" json:"platformName,omitempty"`
	Params            SuiteConfig       `yaml:"params,omitempty" json:"param,omitempty"`
	ScreenResolution  string            `yaml:"screenResolution,omitempty" json:"screenResolution,omitempty"`
	Env               map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	NumShards         int               `yaml:"numShards,omitempty" json:"-"`
	Shard             string            `yaml:"shard,omitempty" json:"-"`
}

// SuiteConfig represents the configuration specific to a suite
type SuiteConfig struct {
	BrowserName string `yaml:"browserName,omitempty" json:"browserName,omitempty"`

	// Fields appeared in v1.12+
	Headless      bool   `yaml:"headless,omitempty" json:"headless,omitempty"`
	GlobalTimeout int    `yaml:"globalTimeout,omitempty" json:"globalTimeout,omitempty"`
	Timeout       int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Grep          string `yaml:"grep,omitempty" json:"grep,omitempty"`
	RepeatEach    int    `yaml:"repeatEach,omitempty" json:"repeatEach,omitempty"`
	Retries       int    `yaml:"retries,omitempty" json:"retries,omitempty"`
	MaxFailures   int    `yaml:"maxFailures,omitempty" json:"maxFailures,omitempty"`
	Project       string `yaml:"project" json:"project,omitempty"`

	// Shard is set by saucectl (not user) based on Suite.NumShards.
	Shard string `yaml:"-" json:"shard,omitempty"`

	// Deprecated fields in v1.12+
	HeadFul             bool `yaml:"headful,omitempty" json:"headful,omitempty"`
	ScreenshotOnFailure bool `yaml:"screenshotOnFailure,omitempty" json:"screenshotOnFailure,omitempty"`
	SlowMo              int  `yaml:"slowMo,omitempty" json:"slowMo,omitempty"`
	Video               bool `yaml:"video,omitempty" json:"video,omitempty"`

	// Will be deprecated since `headless` is introduced
	Headed bool `yaml:"headed,omitempty" json:"headed,omitempty"`
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

	// Default rootDir to .
	if p.RootDir == "" {
		p.RootDir = "."
		msg.LogRootDirWarning()
	}

	if p.Defaults.Timeout < 0 {
		p.Defaults.Timeout = 0
	}

	p.Sauce.Tunnel.SetDefaults()

	for k := range p.Suites {
		s := &p.Suites[k]
		if s.PlatformName == "" {
			s.PlatformName = "Windows 10"
		}

		if s.Timeout <= 0 {
			s.Timeout = p.Defaults.Timeout
		}

		if s.Env == nil {
			s.Env = map[string]string{}
		}
		for envK, envV := range s.Env {
			s.Env[envK] = os.ExpandEnv(envV)
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

// ShardSuites applies sharding by NumShards or by Shard (based on pattern)
func ShardSuites(p *Project) error {
	if err := checkShards(p); err != nil {
		return err
	}

	// either sharding by NumShards or by Shard will be applied
	p.Suites = shardSuitesByNumShards(p.Suites)
	shardedSuites, err := shardInSuites(p.RootDir, p.Suites, p.Sauce.Concurrency)
	if err != nil {
		return err
	}
	p.Suites = shardedSuites

	return nil
}

func checkShards(p *Project) error {
	errMsg := "suite name: %s numShards and shard can't be used at the same time"
	for _, suite := range p.Suites {
		if suite.NumShards >= 2 && suite.Shard != "" {
			return fmt.Errorf(errMsg, suite.Name)
		}
	}

	return nil
}

// shardInSuites divides suites into shards based on the pattern.
func shardInSuites(rootDir string, suites []Suite, concurrencyCount int) ([]Suite, error) {
	var shardedSuites []Suite

	for _, s := range suites {
		if s.Shard != "spec" && s.Shard != "concurrency" {
			shardedSuites = append(shardedSuites, s)
			continue
		}
		testFiles, err := fpath.FindFiles(rootDir, s.TestMatch, fpath.FindByRegex)
		if err != nil {
			return []Suite{}, err
		}
		if len(testFiles) == 0 {
			msg.SuiteSplitNoMatch(s.Name, rootDir, s.TestMatch)
			return []Suite{}, fmt.Errorf("suite '%s' patterns have no matching files", s.Name)
		}
		if s.Shard == "spec" {
			for _, f := range testFiles {
				replica := s
				replica.Name = fmt.Sprintf("%s - %s", s.Name, f)
				replica.TestMatch = []string{f}
				shardedSuites = append(shardedSuites, replica)
			}
		}
		if s.Shard == "concurrency" {
			groups := concurrency.SplitTestFiles(testFiles, concurrencyCount)
			for i, group := range groups {
				replica := s
				replica.Name = fmt.Sprintf("%s - %d/%d", s.Name, i+1, len(groups))
				replica.TestMatch = group
				shardedSuites = append(shardedSuites, replica)
			}
		}
	}
	return shardedSuites, nil
}

// shardSuitesByNumShards applies sharding by replacing the original suites with the appropriate number of replicas according to
// the numShards setting on each suite. A suite is only sharded if numShards > 1.
func shardSuitesByNumShards(suites []Suite) []Suite {
	var shardedSuites []Suite
	for _, s := range suites {
		// Use the original suite if there is nothing to shard.
		if s.NumShards <= 1 {
			shardedSuites = append(shardedSuites, s)
			continue
		}

		for i := 1; i <= s.NumShards; i++ {
			replica := s
			replica.Params.Shard = fmt.Sprintf("%d/%d", i, s.NumShards)
			replica.Name = fmt.Sprintf("%s (shard %s)", replica.Name, replica.Params.Shard)
			shardedSuites = append(shardedSuites, replica)
		}
	}
	return shardedSuites
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

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p *Project) error {
	p.Playwright.Version = config.StandardizeVersionFormat(p.Playwright.Version)
	if p.Playwright.Version == "" {
		return errors.New(msg.MissingFrameworkVersionConfig)
	}

	// Check rootDir exists.
	if p.RootDir != "" {
		if _, err := os.Stat(p.RootDir); err != nil {
			return fmt.Errorf(msg.UnableToLocateRootDir, p.RootDir)
		}
	}

	if err := checkSupportedBrowsers(p); err != nil {
		return err
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New(msg.MissingRegion)
	}

	return nil
}

func checkSupportedBrowsers(p *Project) error {
	for _, suite := range p.Suites {
		if suite.Params.BrowserName != "" && !isSupportedBrowser(suite.Params.BrowserName) {
			return fmt.Errorf(msg.UnsupportedBrowser, suite.Params.BrowserName, strings.Join(supportedBrwsList, ", "))
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
	return fmt.Errorf(msg.SuiteNameNotFound, suiteName)
}

func IsSharded(suites []Suite) bool {
	for _, s := range suites {
		if s.NumShards > 1 || s.Shard != "" {
			return true
		}
	}
	return false
}
