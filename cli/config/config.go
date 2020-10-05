package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Metadata describes job metadata
type Metadata struct {
	Name  string   `yaml:"name"`
	Tags  []string `yaml:"tags"`
	Build string   `yaml:"build"`
}

// Timeouts describes WebDriver timeouts
type Timeouts struct {
	Script   int `yaml:"script"`
	PageLoad int `yaml:"pageLoad"`
	Implicit int `yaml:"implicit"`
}

// Settings describes job settings
type Settings struct {
	BrowserName               string   `yaml:"browserName"`
	BrowserVersion            string   `yaml:"browserVersion"`
	PlatformName              string   `yaml:"platformName"`
	AcceptInsecureCerts       bool     `yaml:"acceptInsecureCerts"`
	PageLoadStrategy          bool     `yaml:"pageLoadStrategy"`
	SetWindowRect             bool     `yaml:"setWindowRect"`
	Timeouts                  Timeouts `yaml:"timeouts"`
	StrictFileInteractability bool     `yaml:"strictFileInteractability"`
	UnhandledPromptBehavior   string   `yaml:"unhandledPromptBehavior"`
}

// ImageDefinition describe configuration to the testrunner image
type ImageDefinition struct {
	Base    string                 `yaml:"base"`
	Version string                 `yaml:"version"`
	Exec    string                 `yaml:"exec"`
	Options map[string]interface{} `yaml:"options"`
}

// Project represents the project configuration.
type Project struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   Metadata          `yaml:"metadata"`
	Suites     []Suite           `yaml:"suites"`
	Files      []string          `yaml:"files"`
	Image      ImageDefinition   `yaml:"image"`
	BeforeExec []string          `yaml:"beforeExec"`
	Timeout    int               `yaml:"timeout"`
	Sauce      SauceConfig       `yaml:"sauce"`
	Env        map[string]string `yaml:"env"`
	Parallel   bool              `yaml:"parallel"`
}

// Suite represents the test suite configuration.
type Suite struct {
	Name         string   `yaml:"name"`
	Capabilities Settings `yaml:"capabilities"`
	Settings     Settings `yaml:"settings"`
	Match        string   `yaml:"match"`
}

// SauceConfig represents sauce labs related settings.
type SauceConfig struct {
	Region string `yaml:"region"`
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

	return c, nil
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
