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
	"github.com/saucelabs/saucectl/internal/cypress/suite"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
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
	Docker        config.Docker        `yaml:"docker,omitempty" json:"docker"`
	Npm           config.Npm           `yaml:"npm,omitempty" json:"npm"`
	RootDir       string               `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string               `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts     `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters     config.Reporters     `yaml:"reporters,omitempty" json:"-"`
	Env           map[string]string    `yaml:"env,omitempty" json:"env"`
	Notifications config.Notifications `yaml:"notifications,omitempty" json:"-"`
}

// Suite represents the cypress test suite configuration.
type Suite struct {
	Name             string        `yaml:"name,omitempty" json:"name"`
	Browser          string        `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion   string        `yaml:"browserVersion,omitempty" json:"browserVersion"`
	PlatformName     string        `yaml:"platformName,omitempty" json:"platformName"`
	Config           SuiteConfig   `yaml:"config,omitempty" json:"config"`
	ScreenResolution string        `yaml:"screenResolution,omitempty" json:"screenResolution"`
	Mode             string        `yaml:"mode,omitempty" json:"-"`
	Timeout          time.Duration `yaml:"timeout,omitempty" json:"timeout"`
	Shard            string        `yaml:"shard,omitempty" json:"-"`
	Headless         bool          `yaml:"headless,omitempty" json:"headless"`
	PreExec          []string      `yaml:"preExec,omitempty" json:"preExec"`
	TimeZone         string        `yaml:"timeZone,omitempty" json:"timeZone"`
}

// SuiteConfig represents the cypress config overrides.
type SuiteConfig struct {
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

	// Default mode to Mount
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
	p.Sauce.Metadata.SetDefaultBuild()

	for k := range p.Suites {
		s := &p.Suites[k]
		if s.PlatformName == "" {
			s.PlatformName = "Windows 10"
		}

		if s.Timeout <= 0 {
			s.Timeout = p.Defaults.Timeout
		}

		if s.Config.Env == nil {
			s.Config.Env = map[string]string{}
		}

		// Apply global env vars onto suite.
		for k, v := range p.Env {
			s.Config.Env[k] = v
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

	// Validate docker.
	if p.Docker.FileTransfer != config.DockerFileMount && p.Docker.FileTransfer != config.DockerFileCopy {
		return fmt.Errorf(msg.InvalidDockerFileTransferType,
			p.Docker.FileTransfer,
			strings.Join([]string{string(config.DockerFileMount), string(config.DockerFileCopy)}, "|"))
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

	// Validate suites.
	if len(p.Suites) == 0 {
		return errors.New(msg.EmptySuite)
	}
	suiteNames := make(map[string]bool)
	for _, s := range p.Suites {
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

		if len(s.Config.SpecPattern) == 0 {
			return fmt.Errorf(msg.MissingTestFiles, s.Name)
		}
	}

	var err error
	p.Suites, err = shardSuites(p.RootDir, p.Suites, p.Sauce.Concurrency)
	return err
}

func shardSuites(rootDir string, suites []Suite, ccy int) ([]Suite, error) {
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
		if len(files) == 0 {
			msg.SuiteSplitNoMatch(s.Name, rootDir, s.Config.SpecPattern)
			return []Suite{}, fmt.Errorf("suite '%s' patterns have no matching files", s.Name)
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
			fileGroups := concurrency.SplitTestFiles(testFiles, ccy)
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
		if s.Shard != "" {
			return true
		}
	}
	return false
}

func (p *Project) CleanPackages() {
	// Don't allow framework installation, it is provided by the runner
	version, hasFramework := p.Npm.Packages["cypress"]
	if hasFramework {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("cypress", p.Cypress.Version, []string{fmt.Sprintf("cypress@%s", version)}))
		p.Npm.Packages = config.CleanNpmPackages(p.Npm.Packages, []string{"cypress"})
	}
}

func (p *Project) GetSuiteCount() int {
	if p == nil {
		return 0
	}
	return len(p.Suites)
}

func (p *Project) GetVersion() string {
	return p.Cypress.Version
}

func (p *Project) GetRunnerVersion() string {
	return p.RunnerVersion
}

func (p *Project) SetVersion(version string) {
	p.Cypress.Version = version
}

func (p *Project) SetRunnerVersion(version string) {
	p.RunnerVersion = version
}

func (p *Project) GetSauceCfg() config.SauceConfig {
	return p.Sauce
}

func (p *Project) IsDryRun() bool {
	return p.DryRun
}

func (p *Project) GetRootDir() string {
	return p.RootDir
}

func (p *Project) GetSuiteNames() string {
	var names []string
	for _, s := range p.Suites {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}

func (p *Project) GetCfgPath() string {
	return p.ConfigFilePath
}

func (p *Project) GetCLIFlags() map[string]interface{} {
	return p.CLIFlags
}

func (p *Project) GetArtifactsCfg() config.Artifacts {
	return p.Artifacts
}

func (p *Project) IsShowConsoleLog() bool {
	return p.ShowConsoleLog
}

func (p *Project) GetDocker() config.Docker {
	return p.Docker
}

func (p *Project) GetBeforeExec() []string {
	return p.BeforeExec
}

func (p *Project) GetReporter() config.Reporters {
	return p.Reporters
}

func (p *Project) GetNotifications() config.Notifications {
	return p.Notifications
}

func (p *Project) GetNpm() config.Npm {
	return p.Npm
}

func (p *Project) SetCLIFlags(flags map[string]interface{}) {
	p.CLIFlags = flags
}

func (p *Project) GetSuites() []suite.Suite {
	suites := []suite.Suite{}
	for _, s := range p.Suites {
		suites = append(suites, suite.Suite{
			Name:             s.Name,
			Browser:          s.Browser,
			BrowserVersion:   s.BrowserVersion,
			PlatformName:     s.PlatformName,
			ScreenResolution: s.ScreenResolution,
			Mode:             s.Mode,
			Timeout:          s.Timeout,
			Shard:            s.Shard,
			Headless:         s.Headless,
			PreExec:          s.PreExec,
			TimeZone:         s.TimeZone,
			Env:              s.Config.Env,
		})
	}
	return suites
}

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

func (p *Project) AppendTags(tags []string) {
	p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, tags...)
}

func (p *Project) GetSuite() suite.Suite {
	s := p.Suite
	return suite.Suite{
		Name:             s.Name,
		Browser:          s.Browser,
		BrowserVersion:   s.BrowserVersion,
		PlatformName:     s.PlatformName,
		ScreenResolution: s.ScreenResolution,
		Mode:             s.Mode,
		Timeout:          s.Timeout,
		Shard:            s.Shard,
		Headless:         s.Headless,
		PreExec:          s.PreExec,
		TimeZone:         s.TimeZone,
		Env:              s.Config.Env,
	}
}

func (p *Project) IsSharded() bool {
	for _, s := range p.Suites {
		if s.Shard != "" {
			return true
		}
	}
	return false
}

func (p *Project) GetAPIVersion() string {
	return p.APIVersion
}
