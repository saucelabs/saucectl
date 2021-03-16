package cypress

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"

	"github.com/saucelabs/saucectl/internal/config"
)

// Project represents the cypress project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ShowConsoleLog bool
	DryRun         bool               `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Cypress        Cypress            `yaml:"cypress,omitempty" json:"cypress"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
	RootDir        string             `yaml:"rootDir,omitempty" json:"rootDir"`
	RunnerVersion  string             `yaml:"runnerVersion,omitempty" json:"runnerVersion"`
}

// Suite represents the cypress test suite configuration.
type Suite struct {
	Name             string      `yaml:"name,omitempty" json:"name"`
	Browser          string      `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion   string      `yaml:"browserVersion,omitempty" json:"browserVersion"`
	PlatformName     string      `yaml:"platformName,omitempty" json:"platformName"`
	Config           SuiteConfig `yaml:"config,omitempty" json:"config"`
	ScreenResolution string      `yaml:"screenResolution,omitempty" json:"screenResolution"`
}

// SuiteConfig represents the cypress config overrides.
type SuiteConfig struct {
	TestFiles []string          `yaml:"testFiles,omitempty" json:"testFiles"`
	Env       map[string]string `yaml:"env,omitempty" json:"env"`
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

	// ProjectPath is the path to the cypress directory itself. Not set by the user, but is instead based on the
	// location of ConfigFile.
	ProjectPath string `yaml:"-" json:"-"`

	// EnvFile is the path to cypress.env.json. Not set by the user, but is instead based on the location of ConfigFile.
	EnvFile string `yaml:"-" json:"-"`
}

// FromFile creates a new cypress Project based on the filepath cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	f, err := os.Open(cfgPath)
	defer f.Close()
	if err != nil {
		return Project{}, fmt.Errorf("failed to locate project config: %v", err)
	}

	if err = yaml.NewDecoder(f).Decode(&p); err != nil {
		return Project{}, fmt.Errorf("failed to parse project config: %v", err)
	}

	p.Cypress.Key = os.ExpandEnv(p.Cypress.Key)

	p.Cypress.Version = config.StandardizeVersionFormat(p.Cypress.Version)

	if p.Cypress.Version == "" {
		return p, errors.New("missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}

	if _, err := os.Stat(p.Cypress.ConfigFile); err != nil {
		return p, fmt.Errorf("unable to locate %s", p.Cypress.ConfigFile)
	}
	configDir := filepath.Dir(p.Cypress.ConfigFile)

	// We must locate the cypress folder.
	cPath := filepath.Join(configDir, "cypress")
	if _, err := os.Stat(cPath); err != nil {
		return p, fmt.Errorf("unable to locate the cypress folder in %s", configDir)
	}
	p.Cypress.ProjectPath = cPath

	// Check rootDir if it is set.
	if p.RootDir != "" {
		if _, err := os.Stat(p.RootDir); err != nil {
			return p, fmt.Errorf("unable to locate the rootDir folder %s", p.RootDir)
		}
	}

	// Optionally include the env file if it exists.
	envFile := filepath.Join(configDir, "cypress.env.json")
	if _, err := os.Stat(envFile); err == nil {
		p.Cypress.EnvFile = envFile
	}

	// Default mode to Mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}

	if p.Docker.Image != "" {
		log.Info().Msgf(
			"Ignoring framework version for Docker, using provided image %s (only applicable to docker mode)",
			p.Docker.Image)
	}

	if p.Sauce.Concurrency < 1 {
		// Default concurrency is 2
		p.Sauce.Concurrency = 2
	}

	for i, s := range p.Suites {
		env := map[string]string{}
		for k, v := range s.Config.Env {
			env[k] = os.ExpandEnv(v)
		}
		p.Suites[i].Config.Env = env
	}

	return p, nil
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
func Validate(p Project) error {
	if len(p.Suites) == 0 {
		return errors.New("no suites defined")
	}

	// Validate docker.
	if p.Docker.FileTransfer != config.DockerFileMount && p.Docker.FileTransfer != config.DockerFileCopy {
		return fmt.Errorf("illegal file transfer type '%s', must be one of '%s'",
			p.Docker.FileTransfer,
			strings.Join([]string{string(config.DockerFileMount), string(config.DockerFileCopy)}, "|"))
	}

	// Validate suites.
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
			return fmt.Errorf("no config.testFiles specified in suite '%s", s.Name)
		}
	}

	return nil
}
