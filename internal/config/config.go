package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// Metadata describes job metadata
type Metadata struct {
	Name  string   `yaml:"name" json:"name"`
	Tags  []string `yaml:"tags" json:"tags,omitempty"`
	Build string   `yaml:"build" json:"build"`
}

// Timeouts describes WebDriver timeouts
type Timeouts struct {
	Script   int `yaml:"script,omitempty"`
	PageLoad int `yaml:"pageLoad,omitempty"`
	Implicit int `yaml:"implicit,omitempty"`
}

// Settings describes job settings
type Settings struct {
	BrowserName               string   `yaml:"browserName,omitempty"`
	BrowserVersion            string   `yaml:"browserVersion,omitempty"`
	PlatformName              string   `yaml:"platformName,omitempty"`
	AcceptInsecureCerts       bool     `yaml:"acceptInsecureCerts,omitempty"`
	PageLoadStrategy          bool     `yaml:"pageLoadStrategy,omitempty"`
	SetWindowRect             bool     `yaml:"setWindowRect,omitempty"`
	Timeouts                  Timeouts `yaml:"timeouts,omitempty"`
	StrictFileInteractability bool     `yaml:"strictFileInteractability,omitempty"`
	UnhandledPromptBehavior   string   `yaml:"unhandledPromptBehavior,omitempty"`
}

// ImageDefinition describe configuration to the testrunner image
type ImageDefinition struct {
	Base    string                 `yaml:"base,omitempty"`
	Version string                 `yaml:"version,omitempty"`
	Exec    string                 `yaml:"exec,omitempty"`
	Options map[string]interface{} `yaml:"options,omitempty"`
}

// Project represents the project configuration.
type Project struct {
	TypeDef      `yaml:",inline"`
	Metadata     Metadata          `yaml:"metadata,omitempty"`
	Suites       []Suite           `yaml:"suites,omitempty"`
	Files        []string          `yaml:"files,omitempty"`
	FileTransfer DockerFileMode    `yaml:"fileTransfer,omitempty"`
	Image        ImageDefinition   `yaml:"image,omitempty"`
	BeforeExec   []string          `yaml:"beforeExec,omitempty"`
	Timeout      int               `yaml:"timeout,omitempty"`
	Sauce        SauceConfig       `yaml:"sauce,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"`
	Parallel     bool              `yaml:"parallel,omitempty"`
	Npm          Npm               `yaml:"npm,omitempty"`
}

// Suite represents the test suite configuration.
type Suite struct {
	Name         string   `yaml:"name,omitempty"`
	Capabilities Settings `yaml:"capabilities,omitempty"`
	Settings     Settings `yaml:"settings,omitempty"`
	Match        string   `yaml:"match,omitempty"`
}

// SauceConfig represents sauce labs related settings.
type SauceConfig struct {
	Region      string   `yaml:"region,omitempty" json:"region"`
	Metadata    Metadata `yaml:"metadata,omitempty" json:"metadata"`
	Tunnel      Tunnel   `yaml:"tunnel,omitempty" json:"tunnel,omitempty"`
	Concurrency int      `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
}

// Tunnel represents a sauce labs tunnel.
type Tunnel struct {
	ID     string `yaml:"id,omitempty" json:"id"`
	Parent string `yaml:"parent,omitempty" json:"parent,omitempty"`
}

// RunnerConfiguration describes configurations for the testrunner
type RunnerConfiguration struct {
	RootDir     string   `yaml:"rootDir"`
	TargetDir   string   `yaml:"targetDir"`
	ExecCommand []string `yaml:"execCommand"`
}

// Run represents the configuration for a particular test run. This information is communicated to the test framework.
type Run struct {
	Match       []string `yaml:"match"`
	ProjectPath string   `yaml:"projectPath"`
}

// TypeDef represents the type definition of the config.
type TypeDef struct {
	APIVersion string `yaml:"apiVersion,omitempty"`
	Kind       string `yaml:"kind,omitempty"`
}

// DockerFileMode represent the file providing method
type DockerFileMode string

// DockerFile* represent the different modes
const (
	DockerFileMount DockerFileMode = "mount"
	DockerFileCopy  DockerFileMode = "copy"
)

// Docker represents docker settings.
type Docker struct {
	FileTransfer DockerFileMode `yaml:"fileTransfer,omitempty" json:"fileTransfer"`
	Image        string         `yaml:"image,omitempty" json:"image"`
}

// Npm represents the npm settings
type Npm struct {
	Packages map[string]string `yaml:"packages,omitempty" json:"packages"`
}

// Version* contains referenced config version
const (
	VersionV1Alpha = "v1alpha"
)

// Kind* contains referenced config kinds
const (
	KindCypress    = "cypress"
	KindPlaywright = "playwright"
	KindTestcafe   = "testcafe"
)

func readYaml(cfgFilePath string) ([]byte, error) {
	if cfgFilePath == "" {
		return nil, errors.New("no config file was provided")
	}

	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	fp := cfgFilePath
	if !filepath.IsAbs(fp) {
		fp = filepath.Join(pwd, cfgFilePath)
	}

	return ioutil.ReadFile(fp)
}

// NewRunnerConfiguration reads yaml file for runner configurations
func NewRunnerConfiguration(cfgFilePath string) (RunnerConfiguration, error) {
	var obj RunnerConfiguration

	yamlFile, err := readYaml(cfgFilePath)
	if err != nil {
		return RunnerConfiguration{}, fmt.Errorf("failed to locate runner configuration: %v", err)
	}

	if err = yaml.Unmarshal(yamlFile, &obj); err != nil {
		return RunnerConfiguration{}, fmt.Errorf("failed to parse runner configuration: %v", err)
	}

	return obj, nil
}

// NewJobConfiguration creates a new job configuration based on a config file
func NewJobConfiguration(cfgFilePath string) (Project, error) {
	var c Project

	yamlFile, err := readYaml(cfgFilePath)
	if err != nil {
		return Project{}, fmt.Errorf("failed to locate job configuration: %v", err)
	}

	if err = yaml.Unmarshal(yamlFile, &c); err != nil {
		return Project{}, fmt.Errorf("failed to parse job configuration: %v", err)
	}

	// go-yaml doesn't have the ability to define default values, so we have to do it here
	if c.Env == nil {
		c.Env = make(map[string]string)
	}

	c.SyncCapabilities()

	// Default mode to Mount
	if c.FileTransfer == "" {
		c.FileTransfer = DockerFileMount
	}

	return c, nil
}

// Describe returns a description of the given config that is cfgPath.
func Describe(cfgPath string) (TypeDef, error) {
	var d TypeDef

	yamlFile, err := readYaml(cfgPath)
	if err != nil {
		return TypeDef{}, fmt.Errorf("failed to locate project configuration: %v", err)
	}

	if err = yaml.Unmarshal(yamlFile, &d); err != nil {
		return TypeDef{}, fmt.Errorf("failed to parse project configuration: %v", err)
	}

	// Normalize certain values for ease of use.
	d.Kind = strings.ToLower(d.Kind)

	return d, nil
}

// ExpandEnv expands environment variables inside metadata fields.
func (m *Metadata) ExpandEnv() {
	m.Name = os.ExpandEnv(m.Name)
	m.Build = os.ExpandEnv(m.Build)
	for i, v := range m.Tags {
		m.Tags[i] = os.ExpandEnv(v)
	}

}

// SyncCapabilities uses the project capabilities if no settings have been defined in the suites.
func (p *Project) SyncCapabilities() {
	empty := Settings{}
	for i, s := range p.Suites {
		if s.Settings == empty && s.Capabilities != empty {
			s.Settings = Settings{
				BrowserName:               s.Capabilities.BrowserName,
				BrowserVersion:            s.Capabilities.BrowserVersion,
				PlatformName:              s.Capabilities.PlatformName,
				AcceptInsecureCerts:       s.Capabilities.AcceptInsecureCerts,
				PageLoadStrategy:          s.Capabilities.PageLoadStrategy,
				SetWindowRect:             s.Capabilities.SetWindowRect,
				Timeouts:                  s.Capabilities.Timeouts,
				StrictFileInteractability: s.Capabilities.StrictFileInteractability,
				UnhandledPromptBehavior:   s.Capabilities.UnhandledPromptBehavior,
			}
		}
		p.Suites[i] = s
	}
}

// StandardizeVersionFormat remove the leading v in version to ensure reliable comparisons.
func StandardizeVersionFormat(version string) string {
	if strings.HasPrefix(version, "v") {
		return version[1:]
	}
	return version
}
