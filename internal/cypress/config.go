package cypress

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/saucelabs/saucectl/internal/config"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "cypress"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the cypress project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	Defaults       config.Defaults `yaml:"defaults" json:"defaults"`
	DryRun         bool            `yaml:"-" json:"-"`
	ShowConsoleLog bool
	ConfigFilePath string                 `yaml:"-" json:"-"`
	CLIFlags       map[string]interface{} `yaml:"-" json:"-"`
	Sauce          config.SauceConfig     `yaml:"sauce,omitempty" json:"sauce"`
	Cypress        Cypress                `yaml:"cypress,omitempty" json:"cypress"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite         Suite             `yaml:"suite,omitempty" json:"-"`
	Suites        []Suite           `yaml:"suites,omitempty" json:"suites"`
	BeforeExec    []string          `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker        config.Docker     `yaml:"docker,omitempty" json:"docker"`
	Npm           config.Npm        `yaml:"npm,omitempty" json:"npm"`
	RootDir       string            `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion string            `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
	Artifacts     config.Artifacts  `yaml:"artifacts,omitempty" json:"artifacts"`
	Env           map[string]string `yaml:"env,omitempty" json:"env"`
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
}

// SuiteConfig represents the cypress config overrides.
type SuiteConfig struct {
	TestFiles []string          `yaml:"testFiles,omitempty" json:"testFiles"`
	Env       map[string]string `yaml:"env,omitempty" json:"env"`
}

// Reporter represents a cypress report configuration.
type Reporter struct {
	Name string `yaml:"name" json:"name"`
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
func FromFile(cfgPath string) (Project, error) {
	var p Project

	if err := config.Unmarshal(cfgPath, &p); err != nil {
		return p, err
	}

	p.ConfigFilePath = cfgPath

	p.Cypress.Key = os.ExpandEnv(p.Cypress.Key)

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

		// Expand env vars.
		for envK, envV := range s.Config.Env {
			s.Config.Env[envK] = os.ExpandEnv(envV)

			// Add an entry without CYPRESS_ prefix as we directly pass it in Cypress.
			if strings.HasPrefix(envK, "CYPRESS_") {
				newKey := strings.TrimPrefix(envK, "CYPRESS_")
				s.Config.Env[newKey] = s.Config.Env[envK]
			}
		}
	}
}

func checkAvailability(path string, mustBeDirectory bool) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if mustBeDirectory && !st.IsDir() {
		return fmt.Errorf("%s: not a folder", path)
	}
	return nil
}

// ValidateCypressConfiguration validates that Cypress config has required folders.
func ValidateCypressConfiguration(configFile string) error {
	configDir := filepath.Dir(configFile)
	cfg, err := ConfigFromFile(configFile)
	if err != nil {
		return err
	}

	integrationFolder := "cypress/integration"
	if cfg.IntegrationFolder != "" {
		integrationFolder = cfg.IntegrationFolder
	}
	if err = checkAvailability(filepath.Join(configDir, integrationFolder), true); err != nil {
		return err
	}

	if cfg.FixturesFolder != "" {
		if err = checkAvailability(filepath.Join(configDir, cfg.FixturesFolder), true); err != nil {
			return err
		}
	}

	if cfg.SupportFile != "" {
		if err = checkAvailability(filepath.Join(configDir, cfg.SupportFile), false); err != nil {
			return err
		}
	}

	if cfg.PluginsFile != "" {
		if err = checkAvailability(filepath.Join(configDir, cfg.PluginsFile), false); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p *Project) error {
	p.Cypress.Version = config.StandardizeVersionFormat(p.Cypress.Version)

	if p.Cypress.Version == "" {
		return errors.New("missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}

	cypressConfigFileCompletePath := filepath.Join(p.RootDir, p.Cypress.ConfigFile)
	if _, err := os.Stat(cypressConfigFileCompletePath); err != nil {
		return fmt.Errorf("unable to locate the cypress config file at %s", cypressConfigFileCompletePath)
	}

	// Validate cypress configuration
	if err := ValidateCypressConfiguration(cypressConfigFileCompletePath); err != nil {
		return err
	}

	// Validate docker.
	if p.Docker.FileTransfer != config.DockerFileMount && p.Docker.FileTransfer != config.DockerFileCopy {
		return fmt.Errorf("illegal file transfer type '%s', must be one of '%s'",
			p.Docker.FileTransfer,
			strings.Join([]string{string(config.DockerFileMount), string(config.DockerFileCopy)}, "|"))
	}

	// Check rootDir exists.
	if p.RootDir != "" {
		if _, err := os.Stat(p.RootDir); err != nil {
			return fmt.Errorf("unable to locate the rootDir folder %s", p.RootDir)
		}
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New("no sauce region set")
	}

	// Validate suites.
	if len(p.Suites) == 0 {
		return errors.New("no suites defined")
	}
	suiteNames := make(map[string]bool)
	for _, s := range p.Suites {
		if _, seen := suiteNames[s.Name]; seen {
			return fmt.Errorf("suite names must be unique, but found duplicate for '%s'", s.Name)
		}
		suiteNames[s.Name] = true

		for _, c := range s.Name {
			if unicode.IsSymbol(c) {
				return fmt.Errorf("illegal symbol '%c' in suite name: '%s'", c, s.Name)
			}
		}

		if s.Browser == "" {
			return fmt.Errorf("no browser specified in suite '%s'", s.Name)
		}

		if len(s.Config.TestFiles) == 0 {
			return fmt.Errorf("no config.testFiles specified in suite '%s'", s.Name)
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
