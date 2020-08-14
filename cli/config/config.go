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

// Capabilities describes job capabilies
type Capabilities struct {
	BrowserName               string                 `yaml:"browserName"`
	BrowserVersion            string                 `yaml:"browserVersion"`
	PlatformName              string                 `yaml:"platformName"`
	AcceptInsecureCerts       bool                   `yaml:"acceptInsecureCerts"`
	PageLoadStrategy          bool                   `yaml:"pageLoadStrategy"`
	Proxy                     map[string]interface{} `yaml:"proxy"`
	SetWindowRect             bool                   `yaml:"setWindowRect"`
	Timeouts                  Timeouts               `yaml:"timeouts"`
	StrictFileInteractability bool                   `yaml:"strictFileInteractability"`
	UnhandledPromptBehavior   string                 `yaml:"unhandledPromptBehavior"`
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
	APIVersion   string            `yaml:"apiVersion"`
	Kind         string            `yaml:"kind"`
	Metadata     Metadata          `yaml:"metadata"`
	Capabilities []Capabilities    `yaml:"capabilities"`
	Files        []string          `yaml:"files"`
	Image        ImageDefinition   `yaml:"image"`
	Timeout      int               `yaml:"timeout"`
	Sauce        SauceConfig       `yaml:"sauce"`
	Env          map[string]string `yaml:"env"`
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
