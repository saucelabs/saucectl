package puppeteer

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/spf13/viper"
	"os"
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
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ShowConsoleLog bool
	ConfigFilePath string             `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	// Suite is only used as a workaround to parse adhoc suites that are created via CLI args.
	Suite      Suite             `yaml:"suite,omitempty" json:"-"`
	Suites     []Suite           `yaml:"suites,omitempty" json:"suites"`
	BeforeExec []string          `yaml:"beforeExec,omitempty" json:"beforeExec"`
	Docker     config.Docker     `yaml:"docker,omitempty" json:"docker"`
	Puppeteer  Puppeteer         `yaml:"puppeteer,omitempty" json:"puppeteer"`
	Npm        config.Npm        `yaml:"npm,omitempty" json:"npm"`
	RootDir    string            `yaml:"rootDir,omitempty" json:"rootDir"`
	Artifacts  config.Artifacts  `yaml:"artifacts,omitempty" json:"artifacts"`
	Env        map[string]string `yaml:"env,omitempty" json:"env"`
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

	if err := viper.Unmarshal(&p); err != nil {
		return p, err
	}

	p.ConfigFilePath = cfgPath

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

	// Set default docker file transfer to mount
	if p.Docker.FileTransfer == "" {
		p.Docker.FileTransfer = config.DockerFileMount
	}

	// Apply global env vars onto every suite.
	for k, v := range p.Env {
		for ks := range p.Suites {
			s := &p.Suites[ks]
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = os.ExpandEnv(v)
		}
	}
}

// Validate validates basic configuration of the project and returns an error if any of the settings contain illegal
// values. This is not an exhaustive operation and further validation should be performed both in the client and/or
// server side depending on the workflow that is executed.
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
