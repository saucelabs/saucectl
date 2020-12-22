package playwright

import (
	"fmt"
	"github.com/saucelabs/saucectl/cli/config"
	"gopkg.in/yaml.v2"
	"os"
)

// Project represents the cypress project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Playwright     Playwright         `yaml:"playwright,omitempty" json:"playwright"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
}

type SuiteConfig struct {
	Env       map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	TestFiles map[string]string `yaml:"testFiles,omitempty" json:"testFiles,omitempty"`
}

// Suite represents the cypress test suite configuration.
type Suite struct {
	Name           string                 `yaml:"name,omitempty" json:"name"`
	Browser        string                 `yaml:"browserName,omitempty" json:"browserName"`
	BrowserVersion string                 `yaml:"browserVersion,omitempty" json:"browserVersion"`
	PlatformName   string                 `yaml:"platformName,omitempty" json:"platformName"`
	Param          map[string]interface{} `yaml:"param,omitempty" json:"param,omitempty"`
	Config         SuiteConfig            `yaml:"config" json:"config"`
}

// Playwright represents crucial playwright configuration that is required for setting up a project.
type Playwright struct {
	ProjectPath string                 `yaml:"projectPath,omitempty"`
	Param       map[string]interface{} `yaml:"param,omitempty"`
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

	// Default mode to Mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}
	return p, nil
}
