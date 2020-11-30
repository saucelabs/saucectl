package cypress

import (
	"fmt"
	"github.com/saucelabs/saucectl/cli/config"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

// Project represents the cypress project configuration.
type Project struct {
	Sauce      config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Cypress    Cypress            `yaml:"cypress,omitempty" json:"cypress"`
	Suites     []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker     config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Npm        config.Npm         `yaml:"npm,omitempty" json:"npm"`
}

// Suite represents the cypress test suite configuration.
type Suite struct {
	Name           string      `yaml:"name,omitempty" json:"name"`
	Browser        string      `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion string      `yaml:"browserVersion,omitempty" json:"browserVersion"`
	Config         SuiteConfig `yaml:"config,omitempty" json:"config"`
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

	// ProjectPath is the path to the cypress directory itself. Not set by the user, but is instead based on the
	// location of ConfigFile.
	ProjectPath string `json:"-"`

	// EnvFile is the path to cypress.env.json. Not set by the user, but is instead based on the location of ConfigFile.
	EnvFile string `json:"-"`
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

	// Optionally include the env file if it exists.
	envFile := filepath.Join(configDir, "cypress.env.json")
	if _, err := os.Stat(envFile); err == nil {
		p.Cypress.EnvFile = envFile
	}

	// Default mode to Mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}

	return p, nil
}
