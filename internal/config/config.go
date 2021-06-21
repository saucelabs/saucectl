package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// ErrUnknownCfg is thrown when the provided config doesn't match anything known, be it in "kind" or the version of it.
var ErrUnknownCfg = errors.New("unknown framework configuration")

// Metadata describes job metadata
type Metadata struct {
	Tags  []string `yaml:"tags" json:"tags,omitempty"`
	Build string   `yaml:"build" json:"build"`
}

// SauceConfig represents sauce labs related settings.
type SauceConfig struct {
	Region      string            `yaml:"region,omitempty" json:"region"`
	Metadata    Metadata          `yaml:"metadata,omitempty" json:"metadata"`
	Tunnel      Tunnel            `yaml:"tunnel,omitempty" json:"tunnel,omitempty"`
	Concurrency int               `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
	Sauceignore string            `yaml:"sauceignore,omitempty" json:"sauceignore,omitempty"`
	Experiments map[string]string `yaml:"experiments,omitempty" json:"experiments,omitempty"`
}

// DeviceOptions represents the devices capabilities required from a real device.
type DeviceOptions struct {
	CarrierConnectivity bool   `yaml:"carrierConnectivity,omitempty" json:"carrierConnectivity"`
	DeviceType          string `yaml:"deviceType,omitempty" json:"deviceType,omitempty"`
	Private             bool   `yaml:"private,omitempty" json:"private,omitempty"`
}

// Device represents the RDC device configuration.
type Device struct {
	ID              string        `yaml:"id,omitempty" json:"id"`
	Name            string        `yaml:"name,omitempty" json:"name"`
	PlatformName    string        `yaml:"platformName,omitempty" json:"platformName"`
	PlatformVersion string        `yaml:"platformVersion,omitempty" json:"platformVersion"`
	Options         DeviceOptions `yaml:"options,omitempty" json:"options,omitempty"`
}

// Emulator represents the emulator configuration.
type Emulator struct {
	Name             string   `yaml:"name,omitempty" json:"name,omitempty"`
	PlatformName     string   `yaml:"platformName,omitempty" json:"platformName"`
	Orientation      string   `yaml:"orientation,omitempty" json:"orientation,omitempty"`
	PlatformVersions []string `yaml:"platformVersions,omitempty" json:"platformVersions,omitempty"`
}

// When represents a conditional status for when artifacts should be downloaded.
type When string

// These conditions indicate when artifacts are to be downloaded.
const (
	WhenFail   When = "fail"
	WhenPass   When = "pass"
	WhenNever  When = "never"
	WhenAlways When = "always"
)

// ArtifactDownload represents the test artifacts configuration.
type ArtifactDownload struct {
	Match     []string `yaml:"match,omitempty" json:"match"`
	When      When     `yaml:"when,omitempty" json:"when"`
	Directory string   `yaml:"directory,omitempty" json:"directory"`
}

// Artifacts represents the test artifacts configuration.
type Artifacts struct {
	Download ArtifactDownload `yaml:"download,omitempty" json:"download"`
}

// Tunnel represents a sauce labs tunnel.
type Tunnel struct {
	ID     string `yaml:"id,omitempty" json:"id"`
	Parent string `yaml:"parent,omitempty" json:"parent,omitempty"`
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
	Registry  string            `yaml:"registry,omitempty" json:"registry,omitempty"`
	Packages  map[string]string `yaml:"packages,omitempty" json:"packages"`
	StrictSSL *bool             `yaml:"strictSSL,omitempty" json:"strictSSL"`
}

// Defaults represents default suite settings.
type Defaults struct {
	Mode string `yaml:"mode,omitempty" json:"mode"`
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

	return os.ReadFile(fp)
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

	// Sanity check.
	if d.APIVersion == "" {
		return TypeDef{}, errors.New("invalid sauce config, which is either malformed or corrupt, please refer to https://docs.saucelabs.com/testrunner-toolkit/configuration for creating a valid config")
	}

	// Normalize certain values for ease of use.
	d.Kind = strings.ToLower(d.Kind)

	return d, nil
}

// ExpandEnv expands environment variables inside metadata fields.
func (m *Metadata) ExpandEnv() {
	m.Build = os.ExpandEnv(m.Build)
	for i, v := range m.Tags {
		m.Tags[i] = os.ExpandEnv(v)
	}

}

// StandardizeVersionFormat remove the leading v in version to ensure reliable comparisons.
func StandardizeVersionFormat(version string) string {
	if strings.HasPrefix(version, "v") {
		return version[1:]
	}
	return version
}
