package v1

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
	"github.com/saucelabs/saucectl/internal/cypress/grep"
	"github.com/saucelabs/saucectl/internal/cypress/suite"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/saucereport"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "cypress"

	// APIVersion represents the supported config version.
	APIVersion = "v1"
)

// Project represents the cypress project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	Defaults       config.Defaults        `yaml:"defaults" json:"defaults"`
	DryRun         bool                   `yaml:"-" json:"-"`
	ShowConsoleLog bool                   `yaml:"showConsoleLog" json:"-"`
	ConfigFilePath string                 `yaml:"-" json:"-"`
	CLIFlags       map[string]interface{} `yaml:"-" json:"-"`
	Sauce          config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	Cypress        Cypress                `yaml:"cypress,omitempty" json:"cypress"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite                `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite              `yaml:"suites,omitempty" json:"suites"`
	BeforeExec    []string             `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Npm           config.Npm           `yaml:"npm,omitempty" json:"npm"`
	RootDir       string               `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string               `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Env           map[string]string    `yaml:"env,omitempty" json:"env"`
	EnvFlag       map[string]string    `yaml:"-" json:"-"`
	Notifications config.Notifications `yaml:"notifications,omitempty" json:"-"`
	NodeVersion   string               `yaml:"nodeVersion,omitempty" json:"nodeVersion,omitempty"`
}

// Suite represents the cypress test suite configuration.
type Suite struct {
	Name             string            `yaml:"name,omitempty" json:"name"`
	Browser          string            `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion   string            `yaml:"browserVersion,omitempty" json:"browserVersion"`
	PlatformName     string            `yaml:"platformName,omitempty" json:"platformName"`
	Config           SuiteConfig       `yaml:"config,omitempty" json:"config"`
	ScreenResolution string            `yaml:"screenResolution,omitempty" json:"screenResolution"`
	Timeout          time.Duration     `yaml:"timeout,omitempty" json:"timeout"`
	Shard            string            `yaml:"shard,omitempty" json:"-"`
	ShardGrepEnabled bool              `yaml:"shardGrepEnabled,omitempty" json:"-"`
	Headless         bool              `yaml:"headless,omitempty" json:"headless"`
	PreExec          []string          `yaml:"preExec,omitempty" json:"preExec"`
	TimeZone         string            `yaml:"timeZone,omitempty" json:"timeZone"`
	PassThreshold    int               `yaml:"passThreshold,omitempty" json:"-"`
	SmartRetry       config.SmartRetry `yaml:"smartRetry,omitempty" json:"-"`
}

// SuiteConfig represents the cypress config overrides.
type SuiteConfig struct {
	TestingType        string            `yaml:"testingType,omitempty" json:"testingType"`
	SpecPattern        []string          `yaml:"specPattern,omitempty" json:"specPattern"`
	ExcludeSpecPattern []string          `yaml:"excludeSpecPattern,omitempty" json:"excludeSpecPattern,omitempty"`
	Env                map[string]string `yaml:"env,omitempty" json:"env"`
}

// Reporter represents a cypress report configuration.
type Reporter struct {
	Name    string                 `yaml:"name" json:"name"`
	Options map[string]interface{} `yaml:"options" json:"options"`
}

// Cypress represents crucial cypress configuration that is required for setting up a project.
type Cypress struct {
	// ConfigFile is the path to "cypress.json".
	ConfigFile string `yaml:"configFile,omitempty" json:"configFile"`

	// Version represents the cypress framework version.
	Version string `yaml:"version" json:"version"`

	// Record represents the cypress framework record flag.
	Record bool `yaml:"record" json:"record"`

	// Key represents the cypress framework key flag.
	Key string `yaml:"key" json:"key"`

	// Reporters represents the customer reporters.
	Reporters []Reporter `yaml:"reporters" json:"reporters"`
}

// FromFile creates a new cypress Project based on the filepath cfgPath.
func FromFile(cfgPath string) (*Project, error) {
	var p *Project

	if err := config.Unmarshal(cfgPath, &p); err != nil {
		return p, err
	}

	p.ConfigFilePath = cfgPath

	return p, nil
}

// SetDefaults applies config defaults in case the user has left them blank.
func (p *Project) SetDefaults() {
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
	p.Npm.SetDefaults(p.Kind, p.Cypress.Version)

	for k := range p.Suites {
		s := &p.Suites[k]
		if s.PlatformName == "" {
			s.PlatformName = "Windows 10"
			log.Info().Msgf(msg.InfoUsingDefaultPlatform, s.PlatformName, s.Name)
		}

		if s.Timeout <= 0 {
			s.Timeout = p.Defaults.Timeout
		}

		if s.Config.Env == nil {
			s.Config.Env = map[string]string{}
		}

		// Apply global env vars onto suite.
		// Precedence: --env flag > root-level env vars > suite-level env vars.
		for _, env := range []map[string]string{p.Env, p.EnvFlag} {
			for k, v := range env {
				s.Config.Env[k] = v
			}
		}

		if s.Config.TestingType == "" {
			s.Config.TestingType = "e2e"
		}
		if s.PassThreshold < 1 {
			s.PassThreshold = 1
		}

		// Update cypress related env vars.
		for envK := range s.Config.Env {
			// Add an entry without CYPRESS_ prefix as we directly pass it in Cypress.
			if strings.HasPrefix(envK, "CYPRESS_") {
				newKey := strings.TrimPrefix(envK, "CYPRESS_")
				s.Config.Env[newKey] = s.Config.Env[envK]
			}
		}
	}
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func (p *Project) Validate() error {
	p.Cypress.Version = config.StandardizeVersionFormat(p.Cypress.Version)

	if p.Cypress.Version == "" {
		return errors.New(msg.MissingCypressVersion)
	}

	// Check rootDir exists.
	if p.RootDir != "" {
		if _, err := os.Stat(p.RootDir); err != nil {
			return fmt.Errorf(msg.UnableToLocateRootDir, p.RootDir)
		}
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

	if len(p.Cypress.Reporters) > 0 {
		log.Warn().Msg("cypress.reporters has been deprecated. Migrate your reporting configuration to your cypress config file.")
	}

	// Validate suites.
	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
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

		if s.Browser == "" {
			return fmt.Errorf(msg.MissingBrowserInSuite, s.Name)
		}

		if s.PlatformName == "" {
			return fmt.Errorf(msg.MissingPlatformName)
		}

		if s.Config.TestingType != "e2e" && s.Config.TestingType != "component" {
			return fmt.Errorf(msg.InvalidCypressTestingType, s.Name)
		}

		if len(s.Config.SpecPattern) == 0 {
			return fmt.Errorf(msg.MissingTestFiles, s.Name)
		}
		if p.Sauce.Retries < s.PassThreshold-1 {
			return fmt.Errorf(msg.InvalidPassThreshold)
		}
	}
	if p.Sauce.Retries < 0 {
		log.Warn().Int("retries", p.Sauce.Retries).Msg(msg.InvalidReries)
	}

	if p.Suites, err = shardSuites(p.RootDir, p.Suites, p.Sauce.Concurrency, p.Sauce.Sauceignore); err != nil {
		return err
	}
	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
	}
	return nil
}

func shardSuites(rootDir string, suites []Suite, ccy int, sauceignoreFile string) ([]Suite, error) {
	var shardedSuites []Suite
	for _, s := range suites {
		// Use the original suite if there is nothing to shard.
		if s.Shard != "spec" && s.Shard != "concurrency" {
			shardedSuites = append(shardedSuites, s)
			continue
		}
		files, err := fpath.FindFiles(rootDir, s.Config.SpecPattern, fpath.FindByShellPattern)
		if err != nil {
			return shardedSuites, err
		}

		files = sauceignore.ExcludeSauceIgnorePatterns(files, sauceignoreFile)

		if len(files) == 0 {
			msg.SuiteSplitNoMatch(s.Name, rootDir, s.Config.SpecPattern)
			return []Suite{}, fmt.Errorf("suite '%s' patterns have no matching files", s.Name)
		}

		if s.ShardGrepEnabled {
			grepExp, grepExists := s.Config.Env["grep"]
			grepTagsExp, grepTagsExists := s.Config.Env["grepTags"]

			if grepExists || grepTagsExists {
				var unmatched []string
				files, unmatched = grep.MatchFiles(os.DirFS(rootDir), files, grepExp, grepTagsExp)

				if len(files) == 0 {
					log.Error().Str("suiteName", s.Name).Str("grep", grepExp).Str("grepTags", grepTagsExp).Msg("No files match the configured grep and grepTags expressions")
					return []Suite{}, errors.New(msg.ShardingConfigurationNoMatchingTests)
				} else if len(unmatched) > 0 {
					log.Info().Str("suiteName", s.Name).Str("grep", grepExp).Str("grepTags", grepTagsExp).Msgf("Files filtered out by grep and grepTags: [%s]", unmatched)
				}
			}
		}

		excludedFiles, err := fpath.FindFiles(rootDir, s.Config.ExcludeSpecPattern, fpath.FindByShellPattern)
		if err != nil {
			return shardedSuites, err
		}

		testFiles := fpath.ExcludeFiles(files, excludedFiles)

		if s.Shard == "spec" {
			for _, f := range testFiles {
				replica := s
				replica.Name = fmt.Sprintf("%s - %s", s.Name, f)
				replica.Config.SpecPattern = []string{f}
				shardedSuites = append(shardedSuites, replica)
			}
		}
		if s.Shard == "concurrency" {
			fileGroups := concurrency.BinPack(testFiles, ccy)
			for i, group := range fileGroups {
				replica := s
				replica.Name = fmt.Sprintf("%s - %d/%d", s.Name, i+1, len(fileGroups))
				replica.Config.SpecPattern = group
				shardedSuites = append(shardedSuites, replica)
			}
		}
	}

	return shardedSuites, nil
}

func (p *Project) CleanPackages() {
	// Don't allow framework installation, it is provided by the runner
	version, hasFramework := p.Npm.Packages["cypress"]
	if hasFramework {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("cypress", p.Cypress.Version, []string{fmt.Sprintf("cypress@%s", version)}))
		p.Npm.Packages = config.CleanNpmPackages(p.Npm.Packages, []string{"cypress"})
	}
}

// GetSuiteCount returns the amount of suites
func (p *Project) GetSuiteCount() int {
	if p == nil {
		return 0
	}
	return len(p.Suites)
}

// GetVersion returns cypress version
func (p *Project) GetVersion() string {
	return p.Cypress.Version
}

// GetRunnerVersion returns RunnerVersion
func (p *Project) GetRunnerVersion() string {
	return p.RunnerVersion
}

// SetVersion sets cypress version
func (p *Project) SetVersion(version string) {
	p.Cypress.Version = version
}

// SetRunnerVersion sets runner version
func (p *Project) SetRunnerVersion(version string) {
	p.RunnerVersion = version
}

// GetSauceCfg returns sauce related config
func (p *Project) GetSauceCfg() config.SauceConfig {
	return p.Sauce
}

// IsDryRun returns DryRun
func (p *Project) IsDryRun() bool {
	return p.DryRun
}

// GetRootDir returns RootDir
func (p *Project) GetRootDir() string {
	return p.RootDir
}

// GetSuiteNames returns combined suite names
func (p *Project) GetSuiteNames() string {
	var names []string
	for _, s := range p.Suites {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}

// GetCfgPath returns ConfigFilePath
func (p *Project) GetCfgPath() string {
	return p.ConfigFilePath
}

// GetCLIFlags returns CLIFlags
func (p *Project) GetCLIFlags() map[string]interface{} {
	return p.CLIFlags
}

// GetArtifactsCfg returns config.Artifacts
func (p *Project) GetArtifactsCfg() config.Artifacts {
	return p.Artifacts
}

// IsShowConsoleLog returns ShowConsoleLog
func (p *Project) IsShowConsoleLog() bool {
	return p.ShowConsoleLog
}

// GetBeforeExec returns BeforeExec
func (p *Project) GetBeforeExec() []string {
	return p.BeforeExec
}

// GetReporter returns config.Reporters
func (p *Project) GetReporters() config.Reporters {
	return p.Reporters
}

// GetNotifications returns config.Notifications
func (p *Project) GetNotifications() config.Notifications {
	return p.Notifications
}

// GetNpm returns config.Npm
func (p *Project) GetNpm() config.Npm {
	return p.Npm
}

func (p *Project) SetNpmStrictSSL(strictSSL *bool) {
	p.Npm.StrictSSL = strictSSL
}

// SetCLIFlags sets cli flags
func (p *Project) SetCLIFlags(flags map[string]interface{}) {
	p.CLIFlags = flags
}

// GetSuites returns suites
func (p *Project) GetSuites() []suite.Suite {
	suites := []suite.Suite{}
	for _, s := range p.Suites {
		suites = append(suites, suite.Suite{
			Name:             s.Name,
			Browser:          s.Browser,
			BrowserVersion:   s.BrowserVersion,
			PlatformName:     s.PlatformName,
			ScreenResolution: s.ScreenResolution,
			Timeout:          s.Timeout,
			Shard:            s.Shard,
			Headless:         s.Headless,
			PreExec:          s.PreExec,
			TimeZone:         s.TimeZone,
			Env:              s.Config.Env,
			PassThreshold:    s.PassThreshold,
		})
	}
	return suites
}

// GetKind returns Kind
func (p *Project) GetKind() string {
	return p.Kind
}

// FilterSuites filters out suites in the project that don't match the given suite name.
func (p *Project) FilterSuites(suiteName string) error {
	for _, s := range p.Suites {
		if s.Name == suiteName {
			p.Suites = []Suite{s}
			return nil
		}
	}
	return fmt.Errorf(msg.SuiteNameNotFound, suiteName)
}

// ApplyFlags applys cli flags on cypress project
func (p *Project) ApplyFlags(selectedSuite string) error {
	if selectedSuite != "" {
		if err := p.FilterSuites(selectedSuite); err != nil {
			return err
		}
	}

	// Create an adhoc suite if "--name" is provided
	if p.Suite.Name != "" {
		p.Suites = []Suite{p.Suite}
	}

	return nil
}

// AppendTags adds tags
func (p *Project) AppendTags(tags []string) {
	p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, tags...)
}

// GetSuite returns suite
func (p *Project) GetSuite() suite.Suite {
	s := p.Suite
	return suite.Suite{
		Name:             s.Name,
		Browser:          s.Browser,
		BrowserVersion:   s.BrowserVersion,
		PlatformName:     s.PlatformName,
		ScreenResolution: s.ScreenResolution,
		Timeout:          s.Timeout,
		Shard:            s.Shard,
		Headless:         s.Headless,
		PreExec:          s.PreExec,
		TimeZone:         s.TimeZone,
		Env:              s.Config.Env,
		PassThreshold:    s.PassThreshold,
	}
}

// GetShardTypes returns the shard types in a project.
func (p *Project) GetShardTypes() []string {
	var set = map[string]bool{}
	for _, s := range p.Suites {
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

// GetShardOpts returns additional shard options.
func (p *Project) GetShardOpts() map[string]bool {
	var opts = map[string]bool{}
	for _, s := range p.Suites {
		if s.ShardGrepEnabled {
			opts["shard_by_grep"] = true
		}
	}
	return opts
}

// GetAPIVersion returns APIVersion
func (p *Project) GetAPIVersion() string {
	return p.APIVersion
}

// GetSmartRetry returns the smartRetry config for the given suite.
// Returns an empty config if the suite could not be found.
func (p *Project) GetSmartRetry(suiteName string) config.SmartRetry {
	for _, s := range p.Suites {
		if s.Name == suiteName {
			return s.SmartRetry
		}
	}
	return config.SmartRetry{}
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
		if p.Suites[i].Config.Env == nil {
			p.Suites[i].Config.Env = map[string]string{}
		}
		p.Suites[i].Config.Env["grep"] = strings.Join(failedTests, ";")
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

func (p *Project) GetNodeVersion() string {
	return p.NodeVersion
}

func (p *Project) SetNodeVersion(version string) {
	p.NodeVersion = version
}
