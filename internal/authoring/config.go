package authoring

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
)

// Config descriptors.
var (
	// Kind is the `kind` value that selects this runner.
	Kind = "authoring"
	// APIVersion is the supported configuration version.
	APIVersion = "v1alpha"
)

// Limits and defaults the service and the runner impose.
const (
	// maxBuildNameLength is the per-case run endpoint's cap. The suite-run
	// endpoint allows 255, but the runner never uses it.
	maxBuildNameLength = 100
	// DefaultSuiteTimeout bounds a suite's wait when the configuration sets
	// none. Nothing waits indefinitely (Constitution VIII); an authored run
	// typically finishes within a minute, so this is generous.
	DefaultSuiteTimeout = 30 * time.Minute
	// defaultConcurrency matches the other kinds' default.
	defaultConcurrency = 2
	// defaultTunnelTimeout matches the --tunnel-timeout flag default.
	defaultTunnelTimeout = 30 * time.Second
)

// Project is the `kind: authoring` configuration.
type Project struct {
	config.TypeDef `yaml:",inline" mapstructure:",squash"`
	ConfigFilePath string             `yaml:"-" json:"-"`
	DryRun         bool               `yaml:"-" json:"-"`
	Sauce          config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	Defaults       config.Defaults    `yaml:"defaults,omitempty" json:"defaults"`
	Suites         []Suite            `yaml:"suites,omitempty" json:"suites"`
	Artifacts      config.Artifacts   `yaml:"artifacts,omitempty" json:"artifacts"`
	Reporters      config.Reporters   `yaml:"reporters,omitempty" json:"-"`

	// Settings that the shared `run` flags or a copied configuration can
	// populate but this kind cannot honour. They are decoded only so that
	// Validate can warn about them instead of dropping them silently
	// (FR-013). EnvFlag is what --env binds to; Env is the YAML form.
	Env            map[string]string `yaml:"env,omitempty" json:"-"`
	EnvFlag        map[string]string `yaml:"-" json:"-"`
	ShowConsoleLog bool              `yaml:"showConsoleLog,omitempty" json:"-"`
	LiveLogs       bool              `yaml:"liveLogs,omitempty" json:"-"`
}

// Suite selects a set of authored test cases to run and where to run them.
// Exactly one of TestSuiteID, TestSuiteName and TestCases must be set.
type Suite struct {
	Name string `yaml:"name,omitempty" json:"name"`
	// TestSuiteID is a suite's identifier (dashless 32-hex).
	TestSuiteID string `yaml:"testSuiteId,omitempty" json:"testSuiteId,omitempty"`
	// TestSuiteName is a suite's exact name, resolved at run time. The
	// service's search is a substring match, so the exact comparison is made
	// client-side and an ambiguous match is an error.
	TestSuiteName string `yaml:"testSuiteName,omitempty" json:"testSuiteName,omitempty"`
	// TestCases is an explicit list of test case identifiers (24-hex).
	TestCases []string `yaml:"testCases,omitempty" json:"testCases,omitempty"`
	// Tags narrows a referenced suite's cases to those carrying any of the
	// tags. Ignored when TestCases is given.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	// Targets overrides each case's stored run targets. When omitted the
	// stored targets apply.
	Targets []Target `yaml:"targets,omitempty" json:"targets,omitempty"`
	// Timeout bounds how long the runner waits for each run in this suite.
	// Defaults to defaults.timeout, then DefaultSuiteTimeout.
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout"`
}

// FromFile creates a Project from the configuration file at cfgPath.
func FromFile(cfgPath string) (Project, error) {
	var p Project
	if err := config.Unmarshal(cfgPath, &p); err != nil {
		return p, err
	}
	p.ConfigFilePath = cfgPath
	return p, nil
}

// SetDefaults fills in what the user left blank and normalises what the YAML
// decoder produced: nested capability maps arrive as map[interface{}]
// interface{}, which encoding/json cannot marshal, so they are converted to
// map[string]any here rather than at request time.
func SetDefaults(p *Project) {
	if p.Kind == "" {
		p.Kind = Kind
	}
	if p.APIVersion == "" {
		p.APIVersion = APIVersion
	}
	if p.Sauce.Concurrency < 1 {
		p.Sauce.Concurrency = defaultConcurrency
	}
	p.Sauce.Tunnel.SetDefaults()
	// The other kinds inherit this from the --tunnel-timeout flag default;
	// applying it here too keeps a YAML-only configuration valid.
	if p.Sauce.Tunnel.Name != "" && p.Sauce.Tunnel.Timeout <= 0 {
		p.Sauce.Tunnel.Timeout = defaultTunnelTimeout
	}
	p.Sauce.Metadata.SetDefaultBuild()
	// The service's cap is on characters, so count runes: slicing bytes
	// splits a multi-byte rune and puts invalid UTF-8 in the run request,
	// while also dropping characters that were within the limit.
	if utf8.RuneCountInString(p.Sauce.Metadata.Build) > maxBuildNameLength {
		log.Warn().Msgf("Build name exceeds %d characters and will be truncated for AI authoring runs.", maxBuildNameLength)
		p.Sauce.Metadata.Build = string([]rune(p.Sauce.Metadata.Build)[:maxBuildNameLength])
	}

	for i := range p.Suites {
		s := &p.Suites[i]
		if s.Timeout <= 0 {
			s.Timeout = p.Defaults.Timeout
		}
		if s.Timeout <= 0 {
			s.Timeout = DefaultSuiteTimeout
		}
		for j := range s.Targets {
			s.Targets[j].Capabilities = normalizeCapabilities(s.Targets[j].Capabilities)
		}
	}
}

// normalizeCapabilities converts YAML-decoded nested maps into map[string]any
// all the way down, so the capabilities marshal to JSON unchanged.
func normalizeCapabilities(caps map[string]any) map[string]any {
	if caps == nil {
		return nil
	}
	out := make(map[string]any, len(caps))
	for k, v := range caps {
		out[k] = normalizeValue(v)
	}
	return out
}

// normalizeValue is the recursive step of normalizeCapabilities.
func normalizeValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return normalizeCapabilities(t)
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = normalizeValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeValue(val)
		}
		return out
	default:
		return v
	}
}

// Validate enforces the configuration contract. It also warns about settings
// this kind cannot honour (FR-013) — from here rather than from the command,
// so the warnings fire whether the value came from YAML or from a flag.
func Validate(p Project) error {
	if region.FromString(p.Sauce.Region) == region.None {
		return errors.New(msg.MissingRegion)
	}
	if len(p.Suites) == 0 {
		return errors.New("no suites configured: add at least one entry under 'suites'")
	}

	seen := map[string]bool{}
	for _, s := range p.Suites {
		if err := validateSuite(s); err != nil {
			return err
		}
		if seen[s.Name] {
			return fmt.Errorf("suite name %q is used more than once", s.Name)
		}
		seen[s.Name] = true
	}

	if p.Sauce.Retries > 0 {
		log.Warn().Msg("sauce.retries is not supported for kind: authoring and will be ignored.")
	}
	if len(p.Sauce.Metadata.Tags) > 0 {
		log.Warn().Msg("sauce.metadata.tags is not supported for kind: authoring and will be ignored.")
	}
	if p.Sauce.Tunnel.Owner != "" {
		log.Warn().Msg("sauce.tunnel.owner is not supported for kind: authoring; runs use the tunnel by name only.")
	}
	if len(p.Env) > 0 || len(p.EnvFlag) > 0 {
		log.Warn().Msg("env / --env is not supported for kind: authoring and will be ignored; use 'saucectl authoring variables' to pass values to authored tests.")
	}
	if p.ShowConsoleLog {
		log.Warn().Msg("showConsoleLog / --show-console-log is not supported for kind: authoring and will be ignored.")
	}
	if p.LiveLogs {
		log.Warn().Msg("--live-logs is not supported for kind: authoring and will be ignored.")
	}
	if p.Sauce.LaunchOrder != "" {
		log.Warn().Msg("sauce.launchOrder / --launch-order is not supported for kind: authoring and will be ignored.")
	}
	if p.Sauce.Visibility != "" {
		log.Warn().Msg("sauce.visibility is not supported for kind: authoring and will be ignored.")
	}
	if len(p.Sauce.Experiments) > 0 {
		log.Warn().Msg("sauce.experiments / --experiment is not supported for kind: authoring and will be ignored.")
	}
	// --root-dir and --sauceignore carry non-empty defaults, so a set value
	// cannot be told from the default here; they are irrelevant to this kind
	// and are not warned about.

	return nil
}

// validateSuite checks one suite entry.
func validateSuite(s Suite) error {
	if s.Name == "" {
		return errors.New("every suite needs a name")
	}

	refs := 0
	if s.TestSuiteID != "" {
		refs++
	}
	if s.TestSuiteName != "" {
		refs++
	}
	if len(s.TestCases) > 0 {
		refs++
	}
	if refs != 1 {
		return fmt.Errorf("suite %q must set exactly one of testSuiteId, testSuiteName or testCases", s.Name)
	}
	for _, id := range s.TestCases {
		if id == "" {
			return fmt.Errorf("suite %q has an empty test case id", s.Name)
		}
	}
	if len(s.TestCases) > 0 && len(s.Tags) > 0 {
		log.Warn().Msgf("Suite %q: tags are ignored when testCases is set.", s.Name)
	}
	for i, t := range s.Targets {
		if len(t.Capabilities) == 0 {
			return fmt.Errorf("suite %q target %d has no capabilities", s.Name, i+1)
		}
	}
	if s.Timeout < 0 {
		return fmt.Errorf("suite %q has a negative timeout", s.Name)
	}
	return nil
}

// FilterSuites narrows the project to the named suite, for --select-suite.
func FilterSuites(p *Project, suiteName string) error {
	for _, s := range p.Suites {
		if s.Name == suiteName {
			p.Suites = []Suite{s}
			return nil
		}
	}
	return fmt.Errorf(msg.SuiteNameNotFound, suiteName)
}
