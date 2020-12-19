package playwright

import (
	"fmt"
	"github.com/saucelabs/saucectl/cli/config"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

// Project represents the cypress project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Playwright     Playwright         `yaml:"cypress,omitempty" json:"cypress"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
}

// Suite represents the cypress test suite configuration.
type Suite struct {
	Name           string      `yaml:"name,omitempty" json:"name"`
	Browser        string      `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion string      `yaml:"browserVersion,omitempty" json:"browserVersion"`
	PlatformName   string      `yaml:"platformName,omitempty" json:"platformName"`
	Config         SuiteConfig `yaml:"config,omitempty" json:"config"`
}

// SuiteConfig represents the cypress config overrides.
type SuiteConfig struct {
	TestFiles []string          `yaml:"testFiles,omitempty" json:"testFiles"`
	Env       map[string]string `yaml:"env,omitempty" json:"env"`
}

// Playwright represents crucial playwright configuration that is required for setting up a project.
type Playwright struct {
	ConfigFile  string `yaml:"configFile,omitempty"`
	EnvFile     string `yaml:"envFile,omitempty"`
	ProjectPath string `yaml:"projectPath,omitempty"`
}

// FromFile creates a new playwright Project based on the filepath cfgPath.
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

	if _, err := os.Stat(p.Playwright.ConfigFile); err != nil {
		return p, fmt.Errorf("unable to locate %s", p.Playwright.ConfigFile)
	}
	configDir := filepath.Dir(p.Playwright.ConfigFile)

	// We must locate the cypress folder.
	cPath := filepath.Join(configDir, "cypress")
	if _, err := os.Stat(cPath); err != nil {
		return p, fmt.Errorf("unable to locate the cypress folder in %s", configDir)
	}
	p.Playwright.ProjectPath = cPath

	// Optionally include the env file if it exists.
	envFile := filepath.Join(configDir, "cypress.env.json")
	if _, err := os.Stat(envFile); err == nil {
		p.Playwright.EnvFile = envFile
	}

	// Default mode to Mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}
	return p, nil
}
