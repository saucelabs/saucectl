package puppeteer

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/region"
	"os"

	"github.com/saucelabs/saucectl/internal/config"
	"gopkg.in/yaml.v2"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "puppeteer"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the puppeteer project configuration.
type Project struct {
	config.TypeDef `yaml:",inline"`
	ShowConsoleLog bool
	ConfigFilePath string             `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	BeforeExec     []string           `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker         config.Docker      `yaml:"docker,omitempty" json:"docker"`
	Puppeteer      Puppeteer          `yaml:"puppeteer,omitempty" json:"puppeteer"`
	Npm            config.Npm         `yaml:"npm,omitempty" json:"npm"`
	RootDir        string             `yaml:"rootDir,omitempty" json:"rootDir"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
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

	if cfgPath == "" {
		return Project{}, nil
	}

	f, err := os.Open(cfgPath)
	if err != nil {
		return p, fmt.Errorf("failed to locate project config: %v", err)
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&p); err != nil {
		return Project{}, fmt.Errorf("failed to parse project config: %v", err)
	}
	p.ConfigFilePath = cfgPath

	if p.Kind != Kind && p.APIVersion != APIVersion {
		return p, config.ErrUnknownCfg
	}

	for _, s := range p.Suites {
		for kk, v := range s.Env {
			s.Env[kk] = os.ExpandEnv(v)
		}
	}

	return p, nil
}

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

	// Set default docker file transfer to mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}
}

func Validate(p *Project) error {
	p.Puppeteer.Version = config.StandardizeVersionFormat(p.Puppeteer.Version)
	if p.Puppeteer.Version == "" {
		return errors.New("missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
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

	return nil
}
