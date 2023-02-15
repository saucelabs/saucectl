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
	Env            map[string]string  `yaml:"env,omitempty"`
}

// Suite represents the apitest suite configuration.
type Suite struct {
	Timeout        time.Duration     `yaml:"timeout,omitempty"`
	Name           string            `yaml:"name,omitempty"`
	ProjectName    string            `yaml:"projectName,omitempty"`
	UseRemoteTests bool              `yaml:"useRemoteTests,omitempty"`
	Tests          []string          `yaml:"tests,omitempty"`
	Tags           []string          `yaml:"tags,omitempty"`
	TestMatch      []string          `yaml:"testMatch,omitempty"`
	Env            map[string]string `yaml:"env,omitempty"`

	// HookID is a technical ID unique to a project that's required by the APIs that execute API tests.
	// The HookID is retrieved dynamically before calling those endpoints.
	HookID string `yaml:"-"`
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

	if p.RootDir == "" {
		p.RootDir = "."
	}

	// Apply global env var onto every suite.
	for k, v := range p.Env {
		for ks := range p.Suites {
			s := &p.Suites[ks]
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			s.Env[k] = v
		}
	}
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
	if suite.ProjectName == "" {
		return errors.New(msg.NoProjectName)
	}
	return nil
}
