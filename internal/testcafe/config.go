package testcafe

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/node"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/saucereport"
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
	DryRun         bool                   `yaml:"-" json:"-"`
	ShowConsoleLog bool                   `yaml:"showConsoleLog" json:"-"`
	ConfigFilePath string                 `yaml:"-" json:"-"`
	CLIFlags       map[string]interface{} `yaml:"-" json:"-"`
	Sauce          config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite                `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite              `yaml:"suites,omitempty" json:"suites"`
	BeforeExec    []string             `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Testcafe      Testcafe             `yaml:"testcafe,omitempty" json:"testcafe"`
	Npm           config.Npm           `yaml:"npm,omitempty" json:"npm"`
	RootDir       string               `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string               `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Defaults      config.Defaults      `yaml:"defaults,omitempty" json:"defaults"`
	Env           map[string]string    `yaml:"env,omitempty" json:"env"`
	EnvFlag       map[string]string    `yaml:"-" json:"-"`
	Notifications config.Notifications `yaml:"notifications,omitempty" json:"-"`
	NodeVersion   string               `yaml:"nodeVersion,omitempty" json:"nodeVersion,omitempty"`
}

// Filter represents the testcafe filters configuration
type Filter struct {
	Test        string            `yaml:"test,omitempty" json:"test,omitempty"`
	TestGrep    string            `yaml:"testGrep,omitempty" json:"testGrep,omitempty"`
	Fixture     string            `yaml:"fixture,omitempty" json:"fixture,omitempty"`
	FixtureGrep string            `yaml:"fixtureGrep,omitempty" json:"fixtureGrep,omitempty"`
	TestMeta    map[string]string `yaml:"testMeta,omitempty" json:"testMeta,omitempty"`
	FixtureMeta map[string]string `yaml:"fixtureMeta,omitempty" json:"fixtureMeta,omitempty"`
}

// CompilerOptions represents the compiler options.
type CompilerOptions struct {
	TypeScript TypescriptCompilerOptions `yaml:"typescript,omitempty" json:"typescript,omitempty"`
}

// TypescriptCompilerOptions represents the typescript compiler options.
type TypescriptCompilerOptions struct {
	ConfigPath               string            `yaml:"configPath,omitempty" json:"configPath,omitempty"`
	CustomCompilerModulePath string            `yaml:"customCompilerModulePath,omitempty" json:"customCompilerModulePath,omitempty"`
	Options                  map[string]string `yaml:"options,omitempty" json:"options,omitempty"`
}

// Suite represents the testcafe test suite configuration.
type Suite struct {
	Name              string            `yaml:"name,omitempty" json:"name"`
	BrowserName       string            `yaml:"browserName,omitempty" json:"browserName"`
	BrowserVersion    string            `yaml:"browserVersion,omitempty" json:"browserVersion"`
	BrowserArgs       []string          `yaml:"browserArgs,omitempty" json:"browserArgs"`
	Src               []string          `yaml:"src,omitempty" json:"src"`
	Screenshots       Screenshots       `yaml:"screenshots,omitempty" json:"screenshots"`
	PlatformName      string            `yaml:"platformName,omitempty" json:"platformName"`
	ScreenResolution  string            `yaml:"screenResolution,omitempty" json:"screenResolution"`
	Env               map[string]string `yaml:"env,omitempty" json:"env"`
	Timeout           time.Duration     `yaml:"timeout,omitempty" json:"timeout"`
	PreExec           []string          `yaml:"preExec,omitempty" json:"preExec"`
	ExcludedTestFiles []string          `yaml:"excludedTestFiles,omitempty" json:"-"`
	// Deprecated as of TestCafe v1.10.0 https://testcafe.io/documentation/402638/reference/configuration-file#tsconfigpath
	TsConfigPath         string                 `yaml:"tsConfigPath,omitempty" json:"tsConfigPath"`
	ClientScripts        []string               `yaml:"clientScripts,omitempty" json:"clientScripts,omitempty"`
	SkipJsErrors         bool                   `yaml:"skipJsErrors,omitempty" json:"skipJsErrors"`
	QuarantineMode       map[string]interface{} `yaml:"quarantineMode,omitempty" json:"quarantineMode,omitempty"`
	SkipUncaughtErrors   bool                   `yaml:"skipUncaughtErrors,omitempty" json:"skipUncaughtErrors"`
	SelectorTimeout      int                    `yaml:"selectorTimeout,omitempty" json:"selectorTimeout"`
	AssertionTimeout     int                    `yaml:"assertionTimeout,omitempty" json:"assertionTimeout"`
	PageLoadTimeout      int                    `yaml:"pageLoadTimeout,omitempty" json:"pageLoadTimeout"`
	AjaxRequestTimeout   int                    `yaml:"ajaxRequestTimeout,omitempty" json:"ajaxRequestTimeout"`
	PageRequestTimeout   int                    `yaml:"pageRequestTimeout,omitempty" json:"pageRequestTimeout"`
	BrowserInitTimeout   int                    `yaml:"browserInitTimeout,omitempty" json:"browserInitTimeout"`
	TestExecutionTimeout int                    `yaml:"testExecutionTimeout,omitempty" json:"testExecutionTimeout"`
	RunExecutionTimeout  int                    `yaml:"runExecutionTimeout,omitempty" json:"runExecutionTimeout"`
	Speed                float64                `yaml:"speed,omitempty" json:"speed"`
	StopOnFirstFail      bool                   `yaml:"stopOnFirstFail,omitempty" json:"stopOnFirstFail"`
	DisablePageCaching   bool                   `yaml:"disablePageCaching,omitempty" json:"disablePageCaching"`
	DisableScreenshots   bool                   `yaml:"disableScreenshots,omitempty" json:"disableScreenshots"`
	Filter               Filter                 `yaml:"filter,omitempty" json:"filter,omitempty"`
	DisableVideo         bool                   `yaml:"disableVideo,omitempty" json:"disableVideo"` // This field is for sauce, not for native testcafe config.
	Mode                 string                 `yaml:"mode,omitempty" json:"-"`
	Shard                string                 `yaml:"shard,omitempty" json:"-"`
	Headless             bool                   `yaml:"headless,omitempty" json:"headless"`
	TimeZone             string                 `yaml:"timeZone,omitempty" json:"timeZone"`
	PassThreshold        int                    `yaml:"passThreshold,omitempty" json:"-"`
	SmartRetry           config.SmartRetry      `yaml:"smartRetry,omitempty" json:"-"`
	ESM                  bool                   `yaml:"esm,omitempty" json:"esm"`
	// TypeScript compiling options
	CompilerOptions CompilerOptions `yaml:"compilerOptions,omitempty" json:"compilerOptions"`
	// Deprecated. Reserved for future use for actual devices.
	Devices    []config.Simulator `yaml:"devices,omitempty" json:"devices"`
	Simulators []config.Simulator `yaml:"simulators,omitempty" json:"simulators"`
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
	// ConfigFile represents the testcafe config file
	ConfigFile string `yaml:"configFile,omitempty" json:"configFile"`
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
	p.Npm.SetDefaults()

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

		if suite.Timeout <= 0 {
			suite.Timeout = p.Defaults.Timeout
		}
		if suite.PassThreshold < 1 {
			suite.PassThreshold = 1
		}

		// If this suite is targeting devices, then the platformName on the device takes precedence and we can skip the
		// defaults on the suite level.
		if suite.PlatformName == "" && len(suite.Simulators) == 0 {
			suite.PlatformName = "Windows 10"
			if strings.ToLower(suite.BrowserName) == "safari" {
				suite.PlatformName = "macOS 11.00"
			}
			log.Info().Msgf(msg.InfoUsingDefaultPlatform, suite.PlatformName, suite.Name)
		}

		for j := range suite.Simulators {
			sim := &suite.Simulators[j]
			if sim.PlatformName == "" && appleDeviceRegex.MatchString(sim.Name) {
				sim.PlatformName = "iOS"
			}
		}
	}

	// Apply global env vars onto every suite.
	// Precedence: --env flag > root-level env vars > suite-level env vars.
	for _, env := range []map[string]string{p.Env, p.EnvFlag} {
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

	if err := config.ValidateArtifacts(p.Artifacts); err != nil {
		return err
	}

	if p.Npm.UsePackageLock {
		if err := config.ValidatePackageLock(); err != nil {
			return fmt.Errorf("invalid use of usePackageLock: %w", err)
		}
		packages, err := node.PackageFromFile("package.json")
		if err != nil {
			return fmt.Errorf("invalid use of usePackageLock. Failed to read package.json: %w", err)
		}
		if err := config.ValidatePackage(packages, "testcafe", p.Testcafe.Version); err != nil {
			return fmt.Errorf("invalid use of usePackageLock: %w", err)
		}
	}

	p.Testcafe.Version = config.StandardizeVersionFormat(p.Testcafe.Version)
	if p.Testcafe.Version == "" {
		return errors.New(msg.MissingFrameworkVersionConfig)
	}

	if p.Sauce.LaunchOrder != "" && p.Sauce.LaunchOrder != config.LaunchOrderFailRate {
		return fmt.Errorf(msg.InvalidLaunchingOption, p.Sauce.LaunchOrder, string(config.LaunchOrderFailRate))
	}

	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
	}
	suiteNames := make(map[string]bool)
	for i, v := range p.Suites {
		if _, seen := suiteNames[v.Name]; seen {
			return fmt.Errorf(msg.DuplicateSuiteName, v.Name)
		}
		suiteNames[v.Name] = true

		if len(v.Name) == 0 {
			return fmt.Errorf(msg.MissingSuiteName, i)
		}

		for _, c := range v.Name {
			if unicode.IsSymbol(c) {
				return fmt.Errorf(msg.IllegalSymbol, c, v.Name)
			}
		}

		// Force the user to migrate.
		if len(v.Devices) != 0 {
			return errors.New(msg.InvalidTestCafeDeviceSetting)
		}
		if len(v.ExcludedTestFiles) != 0 {
			files, err := fpath.FindFiles(p.RootDir, v.Src, fpath.FindByShellPattern)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				msg.SuiteSplitNoMatch(v.Name, p.RootDir, v.Src)
				return fmt.Errorf("suite '%s' test patterns have no matching files", v.Name)
			}
			excludedFiles, err := fpath.FindFiles(p.RootDir, v.ExcludedTestFiles, fpath.FindByShellPattern)
			if err != nil {
				return err
			}

			p.Suites[i].Src = fpath.ExcludeFiles(files, excludedFiles)
		}

		if len(v.Simulators) == 0 && v.BrowserName == "" {
			return fmt.Errorf(msg.MissingBrowserInSuite, v.Name)
		}
		if p.Sauce.Retries < v.PassThreshold-1 {
			return fmt.Errorf(msg.InvalidPassThreshold)
		}
	}
	if p.Sauce.Retries < 0 {
		log.Warn().Int("retries", p.Sauce.Retries).Msg(msg.InvalidReries)
	}

	p.Suites, err = shardSuites(p.RootDir, p.Suites, p.Sauce.Concurrency, p.Sauce.Sauceignore)

	return err
}

// shardSuites divides suites into shards based on the pattern.
func shardSuites(rootDir string, suites []Suite, ccy int, sauceignoreFile string) ([]Suite, error) {
	var shardedSuites []Suite

	for _, s := range suites {
		if s.Shard != "spec" && s.Shard != "concurrency" {
			shardedSuites = append(shardedSuites, s)
			continue
		}
		files, err := fpath.FindFiles(rootDir, s.Src, fpath.FindByShellPattern)
		if err != nil {
			return []Suite{}, err
		}
		if len(files) == 0 {
			msg.SuiteSplitNoMatch(s.Name, rootDir, s.Src)
			return []Suite{}, fmt.Errorf("suite '%s' patterns have no matching files", s.Name)
		}
		excludedFiles, err := fpath.FindFiles(rootDir, s.ExcludedTestFiles, fpath.FindByShellPattern)
		if err != nil {
			return []Suite{}, err
		}

		files = sauceignore.ExcludeSauceIgnorePatterns(files, sauceignoreFile)
		testFiles := fpath.ExcludeFiles(files, excludedFiles)

		if s.Shard == "spec" {
			for _, f := range testFiles {
				replica := s
				replica.Name = fmt.Sprintf("%s - %s", s.Name, f)
				replica.Src = []string{f}
				shardedSuites = append(shardedSuites, replica)
			}
		}
		if s.Shard == "concurrency" {
			groups := concurrency.BinPack(testFiles, ccy)
			for i, group := range groups {
				replica := s
				replica.Name = fmt.Sprintf("%s - %d/%d", s.Name, i+1, len(groups))
				replica.Src = group
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
	// if no failed tests found, just keep the original settings
	if len(failedTests) == 0 {
		return nil
	}
	var found bool
	for i, s := range p.Suites {
		if s.Name != suiteName {
			continue
		}
		p.Suites[i].Filter.TestGrep = strings.Join(failedTests, "|")
		found = true
		break
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
