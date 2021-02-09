package testcafe

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
)

// Project represents the testcafe project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ShowConsoleLog bool
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Testcafe       Testcafe           `yaml:"testcafe,omitempty" json:"testcafe"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
}

// Suite represents the testcafe test suite configuration.
type Suite struct {
	Name             string            `yaml:"name,omitempty" json:"name"`
	Browser          string            `yaml:"browser,omitempty" json:"browser"`
	BrowserVersion   string            `yaml:"browserVersion,omitempty" json:"browserVersion"`
	Src              []string          `yaml:"src,omitempty" json:"src"`
	Screenshots      Screenshots       `yaml:"screenshots,omitempty" json:"screenshots"`
	PlatformName     string            `yaml:"platformName,omitempty" json:"platformName"`
	ScreenResolution string            `yaml:"screenResolution,omitempty" json:"screenResolution"`
	Env              map[string]string `yaml:"env,omitempty" json:"env"`
}

// Screenshots represents screenshots configuration.
type Screenshots struct {
	TakeOnFails bool `yaml:"takeOnFails,omitempty" json:"takeOnFails"`
}

// Testcafe represents the configuration for testcafe.
type Testcafe struct {
	// ProjectPath is the path for testing project.
	ProjectPath string `yaml:"projectPath,omitempty" json:"projectPath"`

	// Version represents the testcafe framework version.
	Version string `yaml:"version,omitempty" json:"version"`
}

// FromFile creates a new testcafe project based on the filepath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	f, err := os.Open(cfgPath)
	defer f.Close()
	if err != nil {
		return p, fmt.Errorf("failed to locate project config: %v", err)
	}
	if err = yaml.NewDecoder(f).Decode(&p); err != nil {
		return p, fmt.Errorf("failed to parse project config: %v", err)
	}

	if p.Testcafe.ProjectPath == "" {
		return p, fmt.Errorf("no project folder defined")
	}

	p.Testcafe.Version = config.StandardizeVersionFormat(p.Testcafe.Version)
	if p.Testcafe.Version == "" {
		return p, errors.New("missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}

	// Set default docker file transfer to mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}

	if p.Docker.Image != "" {
		log.Info().Msgf(
			"Ignoring framework version for Docker, using provided image %s (only applicable to docker mode)",
			p.Docker.Image)
	}

	if p.Sauce.Concurrency < 1 {
		p.Sauce.Concurrency = 1
	}

	for i, s := range p.Suites {
		env := map[string]string{}
		for k, v := range s.Env {
			env[k] = os.ExpandEnv(v)
		}
		p.Suites[i].Env = env
	}

	return p, nil
}
