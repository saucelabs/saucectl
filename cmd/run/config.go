package run

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

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

// Capabilties describes job capabilies
type Capabilties struct {
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
	Options map[string]interface{} `yaml:"options"`
}

// Configuration describes testrunner config format
type Configuration struct {
	APIVersion  string          `yaml:"apiVersion"`
	Kind        string          `yaml:"kind"`
	Metadata    Metadata        `yaml:"metadata"`
	Capabilties []Capabilties   `yaml:"capabilties"`
	Files       []string        `yaml:"files"`
	Image       ImageDefinition `yaml:"image"`
}

// Run runs the command
func (c *Configuration) readFromFilePath(cfgFilePath string) (Configuration, error) {
	var config Configuration

	if len(cfgFilePath) == 0 {
		return config, fmt.Errorf("No config file was provided")
	}

	pwd, err := os.Getwd()
	if err != nil {
		return config, err
	}

	yamlFile, err := ioutil.ReadFile(path.Join(pwd, cfgFilePath))
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}
