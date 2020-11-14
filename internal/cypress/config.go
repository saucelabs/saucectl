package cypress

import (
	"fmt"
	"github.com/saucelabs/saucectl/cli/config"
	"gopkg.in/yaml.v2"
	"os"
)

type Project struct {
	Sauce      config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Cypress    Cypress            `yaml:"cypress,omitempty" json:"cypress"`
	Suites     []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker     Docker             `yaml:"docker,omitempty" json:"docker"`
}

type Suite struct {
	Name           string      `yaml:"name,omitempty" json:"name"`
	Browser        string      `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion string      `yaml:"browserVersion,omitempty" json:"browserVersion"`
	Config         SuiteConfig `yaml:"config,omitempty" json:"config"`
}

type SuiteConfig struct {
	TestFiles []string          `yaml:"testFiles,omitempty" json:"testFiles"`
	Env       map[string]string `yaml:"env,omitempty" json:"env"`
}

type Cypress struct {
	// ConfigFile is the path to "cypress.json".
	ConfigFile string `yaml:"configFile,omitempty" json:"configFile"`
}

// DockerImage describes the docker configuration.
type Docker struct {
	Image Image `yaml:"image,omitempty" json:"image"`
}

type Image struct {
	Name string `yaml:"name,omitempty" json:"name"`
	Tag  string `yaml:"tag,omitempty" json:"tag"`
}

func FromFile(cfgPath string) (Project, error) {
	var p Project

	f, err := os.Open(cfgPath)
	if err != nil {
		return Project{}, fmt.Errorf("failed to locate project config: %v", err)
	}

	if err = yaml.NewDecoder(f).Decode(&p); err != nil {
		return Project{}, fmt.Errorf("failed to parse project config: %v", err)
	}

	return p, nil
}
