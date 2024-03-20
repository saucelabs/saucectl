package playwright

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright/grep"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/saucereport"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "playwright"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

var supportedBrowsers = []string{"chromium", "firefox", "webkit", "chrome"}

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
	Npm           config.Npm           `yaml:"npm,omitempty" json:"npm"`
	RootDir       string               `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string               `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Defaults      config.Defaults      `yaml:"defaults,omitempty" json:"defaults"`
	Env           map[string]string    `yaml:"env,omitempty" json:"env"`
	EnvFlag       map[string]string    `yaml:"-" json:"-"`
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
	ExcludedTestFiles []string          `yaml:"excludedTestFiles,omitempty" json:"testIgnore"`
	PlatformName      string            `yaml:"platformName,omitempty" json:"platformName,omitempty"`
	Params            SuiteConfig       `yaml:"params,omitempty" json:"param,omitempty"`
	ScreenResolution  string            `yaml:"screenResolution,omitempty" json:"screenResolution,omitempty"`
	Env               map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	NumShards         int               `yaml:"numShards,omitempty" json:"-"`
	Shard             string            `yaml:"shard,omitempty" json:"-"`
	PreExec           []string          `yaml:"preExec,omitempty" json:"preExec"`
	TimeZone          string            `yaml:"timeZone,omitempty" json:"timeZone"`
	PassThreshold     int               `yaml:"passThreshold,omitempty" json:"-"`
	SmartRetry        config.SmartRetry `yaml:"smartRetry,omitempty" json:"-"`
	ShardGrepEnabled  bool              `yaml:"shardGrepEnabled,omitempty" json:"-"`
}

// SuiteConfig represents the configuration specific to a suite
type SuiteConfig struct {
	BrowserName string `yaml:"browserName,omitempty" json:"browserName,omitempty"`
	// BrowserVersion for playwright is not specified by the user, but determined by Test-Composer
	BrowserVersion string `yaml:"-" json:"-"`

	// Fields appeared in v1.12+
	Headless        bool   `yaml:"headless,omitempty" json:"headless,omitempty"`
	GlobalTimeout   int    `yaml:"globalTimeout,omitempty" json:"globalTimeout,omitempty"`
	Timeout         int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Grep            string `yaml:"grep,omitempty" json:"grep,omitempty"`
	GrepInvert      string `yaml:"grepInvert,omitempty" json:"grepInvert,omitempty"`
	RepeatEach      int    `yaml:"repeatEach,omitempty" json:"repeatEach,omitempty"`
	Retries         int    `yaml:"retries,omitempty" json:"retries,omitempty"`
	MaxFailures     int    `yaml:"maxFailures,omitempty" json:"maxFailures,omitempty"`
	Project         string `yaml:"project" json:"project,omitempty"`
	UpdateSnapshots bool   `yaml:"updateSnapshots,omitempty" json:"updateSnapshots"`
	Workers         int    `yaml:"workers,omitempty" json:"workers,omitempty"`

	// Shard is set by saucectl (not user) based on Suite.NumShards.
	Shard string `yaml:"-" json:"shard,omitempty"`
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

	// Default rootDir to .
	if p.RootDir == "" {
		p.RootDir = "."
		msg.LogRootDirWarning()
	}

	if p.Defaults.Timeout < 0 {
		p.Defaults.Timeout = 0
	}

	p.Sauce.Tunnel.SetDefaults()
	p.Sauce.Metadata.SetDefaultBuild()
	p.Npm.SetDefaults(p.Kind, p.Playwright.Version)

	for k := range p.Suites {
		s := &p.Suites[k]
		if s.PlatformName == "" {
			s.PlatformName = "Windows 10"
			log.Info().Msgf(msg.InfoUsingDefaultPlatform, s.PlatformName, s.Name)
		}

		if s.Timeout <= 0 {
			s.Timeout = p.Defaults.Timeout
		}

		if s.Params.Workers <= 0 {
			s.Params.Workers = 1
		}
		if s.PassThreshold < 1 {
			s.PassThreshold = 1
		}
	}

	// Apply global env vars onto every suite.
	// Precedence: --env flag > root-level env vars > suite-level env vars.
	for _, env := range []map[string]string{p.EnvFlag, p.Env} {
		for k, v := range env {
			for ks := range p.Suites {
				s := &p.Suites[ks]
				if s.Env == nil {
					s.Env = map[string]string{}
				}
				s.Env[k] = v
			}
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
	shardedSuites, err := shardInSuites(p.RootDir, p.Suites, p.Sauce.Concurrency, p.Sauce.Sauceignore)
	if err != nil {
		return err
	}
	p.Suites = shardedSuites

	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
	}
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
func shardInSuites(rootDir string, suites []Suite, ccy int, sauceignoreFile string) ([]Suite, error) {
	var shardedSuites []Suite

	for _, s := range suites {
		if s.Shard != "spec" && s.Shard != "concurrency" {
			shardedSuites = append(shardedSuites, s)
			continue
		}
		files, err := fpath.FindFiles(rootDir, s.TestMatch, fpath.FindByRegex)
		if err != nil {
			return []Suite{}, err
		}
		if len(files) == 0 {
			msg.SuiteSplitNoMatch(s.Name, rootDir, s.TestMatch)
			return []Suite{}, fmt.Errorf("suite '%s' patterns have no matching files", s.Name)
		}
		excludedFiles, err := fpath.FindFiles(rootDir, s.ExcludedTestFiles, fpath.FindByRegex)
		if err != nil {
			return []Suite{}, err
		}

		files = sauceignore.ExcludeSauceIgnorePatterns(files, sauceignoreFile)
		testFiles := fpath.ExcludeFiles(files, excludedFiles)

		if s.ShardGrepEnabled && (s.Params.Grep != "" || s.Params.GrepInvert != "") {
			var unmatched []string
			testFiles, unmatched = grep.MatchFiles(os.DirFS(rootDir), testFiles, s.Params.Grep, s.Params.GrepInvert)
			if len(testFiles) == 0 {
				log.Error().Str("suiteName", s.Name).Str("grep", s.Params.Grep).Msg("No files match the configured grep expressions")
				return []Suite{}, errors.New(msg.ShardingConfigurationNoMatchingTests)
			} else if len(unmatched) > 0 {
				log.Info().Str("suiteName", s.Name).Str("grep", s.Params.Grep).Msgf("Files filtered out by grep: %q", unmatched)
			}
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
			groups := concurrency.BinPack(testFiles, ccy)
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

	if ok := config.ValidateVisibility(p.Sauce.Visibility); !ok {
		return fmt.Errorf(msg.InvalidVisibility, p.Sauce.Visibility, strings.Join(config.ValidVisibilityValues, ","))
	}

	err := config.ValidateRegistries(p.Npm.Registries)
	if err != nil {
		return err
	}

	if err := config.ValidateArtifacts(p.Artifacts); err != nil {
		return err
	}

	if p.Sauce.LaunchOrder != "" && p.Sauce.LaunchOrder != config.LaunchOrderFailRate {
		return fmt.Errorf(msg.InvalidLaunchingOption, p.Sauce.LaunchOrder, string(config.LaunchOrderFailRate))
	}

	suiteNames := make(map[string]bool)
	for idx, s := range p.Suites {
		if len(s.Name) == 0 {
			return fmt.Errorf(msg.MissingSuiteName, idx)
		}

		if _, seen := suiteNames[s.Name]; seen {
			return fmt.Errorf(msg.DuplicateSuiteName, s.Name)
		}
		suiteNames[s.Name] = true

		for _, c := range s.Name {
			if unicode.IsSymbol(c) {
				return fmt.Errorf(msg.IllegalSymbol, c, s.Name)
			}
		}
		if p.Sauce.Retries < s.PassThreshold-1 {
			return fmt.Errorf(msg.InvalidPassThreshold)
		}
	}

	if p.Sauce.Retries < 0 {
		log.Warn().Int("retries", p.Sauce.Retries).Msg(msg.InvalidReries)
	}

	return nil
}

func checkSupportedBrowsers(p *Project) error {
	for _, suite := range p.Suites {
		if suite.Params.BrowserName == "" || !isSupportedBrowser(suite.Params.BrowserName) {
			return fmt.Errorf(msg.UnsupportedBrowser, suite.Params.BrowserName, strings.Join(supportedBrowsers, ", "))
		}
	}

	return nil
}

func isSupportedBrowser(browser string) bool {
	for _, supportedBr := range supportedBrowsers {
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

// FilterFailedTests takes the failed tests in the report and sets them as a test filter in the suite.
// The test filter remains unchanged if the report does not contain any failed tests.
func (p *Project) FilterFailedTests(suiteName string, report saucereport.SauceReport) error {
	failedTests := saucereport.GetFailedTests(report)
	if len(failedTests) == 0 {
		return nil
	}

	var found bool
	for i, s := range p.Suites {
		if s.Name != suiteName {
			continue
		}
		found = true
		p.Suites[i].Params.Grep = strings.Join(failedTests, "|")
	}
	if !found {
		return fmt.Errorf("suite(%s) not found", suiteName)
	}
	return nil
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
