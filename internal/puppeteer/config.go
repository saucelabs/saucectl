package puppeteer

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
)

// Project represents the puppeteer project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ShowConsoleLog bool
	DryRun         bool               `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Puppeteer      Puppeteer          `yaml:"puppeteer,omitempty" json:"puppeteer"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
	RootDir        string             `yaml:"rootDir,omitempty" json:"rootDir"`
}

// Suite represents the puppeteer test suite configuration.
type Suite struct {
	Name      string            `yaml:"name,omitempty" json:"name"`
	Browser   string            `yaml:"browser,omitempty" json:"browser"`
	TestMatch []string          `yaml:"testMatch,omitempty" json:"testMatch"`
	Env       map[string]string `yaml:"env,omitempty" json:"env"`
}

// Puppeteer represents the configuration for puppeteer.
type Puppeteer struct {
	// Version represents the puppeteer framework version.
	Version string `yaml:"version,omitempty" json:"version"`
}

// FromFile creates a new puppeteer project based on the filepath.
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

	if p.RootDir == "" {
		return p, fmt.Errorf("could not find 'rootDir' in config yml, 'rootDir' must be set to specify project files")
	}

	p.Puppeteer.Version = config.StandardizeVersionFormat(p.Puppeteer.Version)
	if p.Puppeteer.Version == "" {
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
		// Default concurrency is 2
		p.Sauce.Concurrency = 2
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
