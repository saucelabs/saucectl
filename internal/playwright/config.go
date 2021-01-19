package playwright

import (
	"fmt"
	"github.com/saucelabs/saucectl/cli/config"
	"gopkg.in/yaml.v2"
	"os"
)

// Project represents the playwright project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Playwright     Playwright         `yaml:"playwright,omitempty" json:"playwright"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
}

// Playwright represents crucial playwright configuration that is required for setting up a project.
type Playwright struct {
	ProjectPath string      `yaml:"projectPath,omitempty" json:"projectPath,omitempty"`
	Version     string      `yaml:"version,omitempty" json:"version,omitempty"`
	Params      SuiteConfig `yaml:"params,omitempty" json:"params,omitempty"`
}

// Suite represents the playwright test suite configuration.
type Suite struct {
	Name              string      `yaml:"name,omitempty" json:"name"`
	PlaywrightVersion string      `yaml:"playwrightVersion,omitempty" json:"playwrightVersion,omitempty"`
	TestMatch         string      `yaml:"testMatch,omitempty" json:"testMatch,omitempty"`
	PlatformName      string      `yaml:"platformName,omitempty" json:"platformName"`
	Params            SuiteConfig `yaml:"params,omitempty" json:"params,omitempty"`
}

// SuiteConfig represents the configuration specific to a suite
type SuiteConfig struct {
	BrowserName         string `yaml:"browserName,omitempty" json:"browserName,omitempty"`
	HeadFull            bool   `yaml:"headful,omitempty" json:"headful,omitempty"`
	ScreenshotOnFailure bool   `yaml:"screenshotOnFailure,omitempty" json:"screenshotOnFailure,omitempty"`
	SlowMo              uint   `yaml:"slowMo,omitempty" json:"slowMo,omitempty"`
	Video               bool   `yaml:"video,omitempty" json:"video,omitempty"`
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

	// Default project path
	if p.Playwright.ProjectPath == "" {
		p.Playwright.ProjectPath = "./tests/"
	}

	// Default mode to Mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}
	return p, nil
}
