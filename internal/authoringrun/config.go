// Package authoringrun implements the `kind: authoring` runner for
// `saucectl run`: it executes AI-authored test suites (created via
// `saucectl author` / `saucectl author sync`) rather than a local test
// framework. See docs/authoring-run-integration-rfc.md for the design.
package authoringrun

import (
	"errors"
	"fmt"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/region"
)

// Config descriptors.
var (
	// Kind represents the type definition of this config.
	Kind = "authoring"

	// APIVersion represents the supported config version.
	APIVersion = "v1alpha"
)

// Project represents the authoring project configuration -- a set of
// already-authored test suites to execute, as opposed to local test files to
// upload.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	DryRun         bool               `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty"`
	Suites         []Suite            `yaml:"suites,omitempty"`
	Reporters      config.Reporters   `yaml:"reporters,omitempty" json:"-"`
}

// Suite represents a single authored test suite to run.
type Suite struct {
	Name string `yaml:"name,omitempty"`

	// TestSuiteID is the Sauce Labs test suite ID (see `saucectl author` /
	// `saucectl author sync`, which create and populate this suite). This is
	// the only thing needed to run it -- the runner asks Sauce for the
	// suite's current test case membership at run time rather than trusting
	// anything cached locally.
	TestSuiteID string `yaml:"testSuiteId,omitempty"`

	// Tags, if set, are applied to each test case run triggered for this
	// suite (in addition to whatever tags the test case already has).
	Tags []string `yaml:"tags,omitempty"`

	// Timeout bounds how long to wait for any single test case's job(s) to
	// finish before treating it as timed out.
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// FromFile creates a new authoring Project based on the filepath cfgPath.
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

	for i := range p.Suites {
		if p.Suites[i].Timeout == 0 {
			p.Suites[i].Timeout = 30 * time.Minute
		}
	}
}

// Validate checks the project for the bare minimum needed to run it.
func Validate(p Project) error {
	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		return errors.New("no sauce region set")
	}

	if len(p.Suites) == 0 {
		return errors.New("no suites defined")
	}

	for _, s := range p.Suites {
		if err := validateSuite(s); err != nil {
			return err
		}
	}

	return nil
}

func validateSuite(suite Suite) error {
	if suite.Name == "" {
		return errors.New("suite is missing a name")
	}
	if suite.TestSuiteID == "" {
		return fmt.Errorf("suite %q is missing testSuiteId", suite.Name)
	}
	return nil
}

// FilterSuites filters out suites in the project that don't match the given
// suite name (supports `saucectl run --select-suite`).
func FilterSuites(p *Project, suiteName string) error {
	for _, s := range p.Suites {
		if s.Name == suiteName {
			p.Suites = []Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite named %q not found", suiteName)
}
