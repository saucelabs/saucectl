package apitest

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "apitest"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the apitest project configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	Suites         []Suite            `yaml:"suites,omitempty"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty"`
	RootDir        string             `yaml:"rootDir,omitempty"`
}

// Suite represents the apitest suite configuration.
type Suite struct {
	Name    string        `yaml:"name,omitempty"`
	HookID  string        `yaml:"hookId,omitempty"`
	Tags    []string      `yaml:"tags,omitempty"`
	Tests   []string      `yaml:"tests,omitempty"`
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// FromFile creates a new apitest Project based on the filepath cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project

	if err := config.Unmarshal(cfgPath, &p); err != nil {
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

	p.Sauce.Tunnel.SetDefaults()
}

func Validate(p Project) error {
	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New(msg.MissingRegion)
	}

	for _, s := range p.Suites {
		if err := validateSuite(s); err != nil {
			return err
		}
	}

	return nil
}

func validateSuite(suite Suite) error {
	if suite.HookID == "" {
		return errors.New("suites must have a hookId defined")
	}
	return nil
}
