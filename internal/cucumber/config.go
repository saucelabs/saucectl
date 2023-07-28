package cucumber

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucereport"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "playwright-cucumberjs"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents cucumber sauce config
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	DryRun         bool                   `yaml:"-" json:"-"`
	ShowConsoleLog bool                   `yaml:"showConsoleLog" json:"-"`
	ConfigFilePath string                 `yaml:"-" json:"-"`
	CLIFlags       map[string]interface{} `yaml:"-" json:"-"`
	Sauce          config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite                `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite              `yaml:"suites,omitempty" json:"suites"`
	BeforeExec    []string             `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Playwright    Playwright           `yaml:"playwright,omitempty" json:"playwright"`
	Npm           config.Npm           `yaml:"npm,omitempty" json:"npm"`
	RootDir       string               `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string               `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Defaults      config.Defaults      `yaml:"defaults,omitempty" json:"defaults"`
	Env           map[string]string    `yaml:"env,omitempty" json:"env"`
	Notifications config.Notifications `yaml:"notifications,omitempty" json:"-"`
}

// Playwright represents the playwright setting
type Playwright struct {
	// Version represents the playwright framework version.
	Version string `yaml:"version,omitempty" json:"version"`
}

// Suite represents the playwright-cucumberjs test suite configuration.
type Suite struct {
	Name             string            `yaml:"name,omitempty" json:"name"`
	BrowserName      string            `yaml:"browserName,omitempty" json:"browserName"`
	BrowserVersion   string            `yaml:"browserVersion,omitempty" json:"browserVersion"`
	PlatformName     string            `yaml:"platformName,omitempty" json:"platformName"`
	Env              map[string]string `yaml:"env,omitempty" json:"env"`
	Shard            string            `yaml:"shard,omitempty" json:"shard"`
	Mode             string            `yaml:"mode,omitemty" json:"mode"`
	Timeout          time.Duration     `yaml:"timeout,omitempty" json:"timeout"`
	ScreenResolution string            `yaml:"screenResolution,omitempty" json:"screenResolution"`
	PreExec          []string          `yaml:"preExec,omitempty" json:"preExec"`
	Options          Options           `yaml:"options,omitempty" json:"options"`
	PassThreshold    int               `yaml:"passThreshold,omitempty" json:"-"`
	SmartRetry       config.SmartRetry `yaml:"smartRetry,omitempty" json:"-"`
}

// Options represents cucumber settings
type Options struct {
	Config string `yaml:"config,omitempty" json:"config"`
	// Name is a regular expression for selecting scenario names.
	Name              string            `yaml:"name,omitempty" json:"name"`
	Paths             []string          `yaml:"paths,omitempty" json:"paths"`
	ExcludedTestFiles []string          `yaml:"excludedTestFiles,omitempty" json:"excludedTestFiles"`
	Backtrace         bool              `yaml:"backtrace,omitempty" json:"backtrace"`
	Require           []string          `yaml:"require,omitempty" json:"require"`
	Import            []string          `yaml:"import,omitempty" json:"import"`
	Tags              []string          `yaml:"tags,omitempty" json:"tags"`
	Format            []string          `yaml:"format,omitempty" json:"format"`
	FormatOptions     map[string]string `yaml:"formatOptions,omitempty" json:"formatOptions"`
	Parallel          int               `yaml:"parallel,omitempty" json:"parallel"`
}

// FromFile creates a new cucumber project based on the filepath.
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

	if p.Defaults.Timeout < 0 {
		p.Defaults.Timeout = 0
	}

	// Default rootDir to .
	if p.RootDir == "" {
		p.RootDir = "."
		msg.LogRootDirWarning()
	}

	p.Sauce.Tunnel.SetDefaults()
	p.Sauce.Metadata.SetDefaultBuild()
	p.Npm.SetDefaults(p.Kind, p.Playwright.Version)

	for k := range p.Suites {
		suite := &p.Suites[k]

		if suite.BrowserName == "" {
			suite.BrowserName = "chromium"
		}
		if suite.PlatformName == "" {
			suite.PlatformName = "Windows 11"

			if strings.ToLower(suite.BrowserName) == "safari" {
				suite.PlatformName = "macOS 12"
			}
		}
		if suite.PassThreshold < 1 {
			suite.PassThreshold = 1
		}
	}

	// Apply global env vars onto every suite.
	for k, v := range p.Env {
		for ks := range p.Suites {
			s := &p.Suites[ks]
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = v
		}
	}
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p *Project) error {
	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New(msg.MissingRegion)
	}

	if ok := config.ValidateVisibility(p.Sauce.Visibility); !ok {
		return fmt.Errorf(msg.InvalidVisibility, p.Sauce.Visibility, strings.Join(config.ValidVisibilityValues, ","))
	}

	err := config.ValidateRegistries(p.Npm.Registries)
	if err != nil {
		return err
	}

	p.Playwright.Version = config.StandardizeVersionFormat(p.Playwright.Version)
	if p.Playwright.Version == "" {
		return errors.New(msg.MissingFrameworkVersionConfig)
	}

	if p.Sauce.LaunchOrder != "" && p.Sauce.LaunchOrder != config.LaunchOrderFailRate {
		return fmt.Errorf(msg.InvalidLaunchingOption, p.Sauce.LaunchOrder, string(config.LaunchOrderFailRate))
	}

	for i, v := range p.Suites {
		files, err := fpath.FindFiles(p.RootDir, v.Options.Paths, fpath.FindByShellPattern)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			msg.SuiteSplitNoMatch(v.Name, p.RootDir, v.Options.Paths)
			return fmt.Errorf("suite '%s' test patterns have no matching files", v.Name)
		}
		excludedFiles, err := fpath.FindFiles(p.RootDir, v.Options.ExcludedTestFiles, fpath.FindByShellPattern)
		if err != nil {
			return err
		}

		p.Suites[i].Options.Paths = fpath.ExcludeFiles(files, excludedFiles)

		if p.Sauce.Retries < v.PassThreshold-1 {
			return fmt.Errorf(msg.InvalidPassThreshold)
		}
	}
	if p.Sauce.Retries < 0 {
		log.Warn().Int("retries", p.Sauce.Retries).Msg(msg.InvalidReries)
	}

	p.Suites, err = shardSuites(p.RootDir, p.Suites, p.Sauce.Concurrency)

	return err
}

// shardSuites divides suites into shards based on the pattern.
func shardSuites(rootDir string, suites []Suite, ccy int) ([]Suite, error) {
	var shardedSuites []Suite

	for _, s := range suites {
		if s.Shard != "spec" && s.Shard != "concurrency" {
			shardedSuites = append(shardedSuites, s)
			continue
		}
		files, err := fpath.FindFiles(rootDir, s.Options.Paths, fpath.FindByShellPattern)
		if err != nil {
			return []Suite{}, err
		}
		if len(files) == 0 {
			msg.SuiteSplitNoMatch(s.Name, rootDir, s.Options.Paths)
			return []Suite{}, fmt.Errorf("suite '%s' patterns have no matching files", s.Name)
		}
		excludedFiles, err := fpath.FindFiles(rootDir, s.Options.ExcludedTestFiles, fpath.FindByShellPattern)
		if err != nil {
			return []Suite{}, err
		}

		testFiles := fpath.ExcludeFiles(files, excludedFiles)

		if s.Shard == "spec" {
			for _, f := range testFiles {
				replica := s
				replica.Name = fmt.Sprintf("%s - %s", s.Name, f)
				replica.Options.Paths = []string{f}
				shardedSuites = append(shardedSuites, replica)
			}
		}
		if s.Shard == "concurrency" {
			groups := concurrency.BinPack(testFiles, ccy)
			for i, group := range groups {
				replica := s
				replica.Name = fmt.Sprintf("%s - %d/%d", s.Name, i+1, len(groups))
				replica.Options.Paths = group
				shardedSuites = append(shardedSuites, replica)
			}
		}
	}

	return shardedSuites, nil
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

// IsSharded checks if the suite is sharded
func IsSharded(suites []Suite) bool {
	for _, s := range suites {
		if s.Shard != "" {
			return true
		}
	}
	return false
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

// FilterFailedTests takes the failed specs in the report and sets them as a test filter in the suite.
// The test filter remains unchanged if the report does not contain any failed tests.
func (p *Project) FilterFailedTests(suiteName string, report saucereport.SauceReport) error {
	specs, err := getFailedSpecFiles(report)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return nil
	}

	var found bool
	for i, s := range p.Suites {
		if s.Name != suiteName {
			continue
		}
		found = true
		p.Suites[i].Options.Paths = specs
	}

	if !found {
		return fmt.Errorf("suite(%s) not found", suiteName)
	}
	return nil
}

func getFailedSpecFiles(report saucereport.SauceReport) ([]string, error) {
	var failedSpecs []string
	if report.Status != saucereport.StatusFailed {
		return failedSpecs, nil
	}
	for _, s := range report.Suites {
		re, err := regexp.Compile(".*.feature$")
		if err != nil {
			return failedSpecs, err
		}
		if s.Status == saucereport.StatusFailed && re.MatchString(s.Name) {
			failedSpecs = append(failedSpecs, filepath.Clean(s.Name))
		}
	}

	return failedSpecs, nil
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
