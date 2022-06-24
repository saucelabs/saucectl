package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/viper"
	"gopkg.in/yaml.v2"
)

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
	Retries     int               `yaml:"retries,omitempty" json:"-"`
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

// Simulator represents the simulator configuration.
type Simulator Emulator

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

// Notifications represents the test notifications configuration.
type Notifications struct {
	Slack Slack `yaml:"slack,omitempty" json:"slack"`
}

// Slack represents slack configuration.
type Slack struct {
	Channels []string `yaml:"channels,omitempty" json:"channels"`
	Send     When     `yaml:"send,omitempty" json:"send"`
}

// Artifacts represents the test artifacts configuration.
type Artifacts struct {
	Download ArtifactDownload `yaml:"download,omitempty" json:"download"`
	Cleanup  bool             `yaml:"cleanup,omitempty" json:"cleanup"`
}

// Reporters represents the reporter configuration.
type Reporters struct {
	JUnit struct {
		Enabled  bool   `yaml:"enabled"`
		Filename string `yaml:"filename"`
	} `yaml:"junit"`

	JSON struct {
		Enabled    bool   `yaml:"enabled"`
		WebhookURL string `yaml:"webhookURL"`
		Filename   string `yaml:"filename"`
	} `yaml:"json"`
}

// Tunnel represents a sauce labs tunnel.
type Tunnel struct {
	// ID represents the tunnel identifier (aka tunnel name).
	// Deprecated. Use Name instead.
	ID   string `yaml:"id,omitempty" json:"id,omitempty"`
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// Parent represents the tunnel owner.
	// Deprecated. Use Owner instead.
	Parent string `yaml:"parent,omitempty" json:"parent,omitempty"`
	Owner  string `yaml:"owner,omitempty" json:"owner,omitempty"`
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
	FileTransfer DockerFileMode `yaml:"fileTransfer,omitempty" json:"fileTransfer,omitempty"`
	Image        string         `yaml:"image,omitempty" json:"image,omitempty"`
}

// Npm represents the npm settings
type Npm struct {
	Registry     string            `yaml:"registry,omitempty" json:"registry,omitempty"`
	Packages     map[string]string `yaml:"packages,omitempty" json:"packages"`
	Dependencies []string          `yaml:"dependencies,omitempty" json:"dependencies"`
	StrictSSL    bool              `yaml:"strictSSL,omitempty" json:"strictSSL"`
}

// Defaults represents default suite settings.
type Defaults struct {
	Mode    string        `yaml:"mode,omitempty" json:"mode"`
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout"`
}

// AppSettings represents override settings.
type AppSettings struct {
	AudioCapture    bool            `yaml:"audioCapture,omitempty" json:"audioCapture"`
	Instrumentation Instrumentation `yaml:"instrumentation,omitempty" json:"instrumentation"`
}

// Instrumentation represents Instrumentation settings for real devices.
type Instrumentation struct {
	NetworkCapture bool `yaml:"networkCapture,omitempty" json:"networkCapture"`
}

func readYaml(cfgFilePath string) ([]byte, error) {
	if cfgFilePath == "" {
		return nil, errors.New(msg.MissingConfigFile)
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

	if cfgPath == "" {
		return TypeDef{}, nil
	}

	yamlFile, err := readYaml(cfgPath)
	if err != nil {
		return TypeDef{}, fmt.Errorf("failed to locate project configuration: %v", err)
	}

	if err = yaml.Unmarshal(yamlFile, &d); err != nil {
		return TypeDef{}, fmt.Errorf("failed to parse project configuration: %v", err)
	}

	// Sanity check.
	if d.APIVersion == "" {
		return TypeDef{}, errors.New(msg.InvalidSauceConfig)
	}

	// Normalize certain values for ease of use.
	d.Kind = strings.ToLower(d.Kind)

	return d, nil
}

// SetDefaultBuild sets default build if it's empty
func (m *Metadata) SetDefaultBuild() {
	if m.Build == "" {
		now := time.Now()
		m.Build = fmt.Sprintf("build-%s", now.Format(time.RFC3339))
	}
}

// StandardizeVersionFormat remove the leading v in version to ensure reliable comparisons.
func StandardizeVersionFormat(version string) string {
	if strings.HasPrefix(version, "v") {
		return version[1:]
	}
	return version
}

// SupportedDeviceTypes contains the list of supported device types.
var SupportedDeviceTypes = []string{"ANY", "PHONE", "TABLET"}

// IsSupportedDeviceType check that the specified deviceType is valid.
func IsSupportedDeviceType(deviceType string) bool {
	for _, dt := range SupportedDeviceTypes {
		if dt == deviceType {
			return true
		}
	}
	return false
}

// CleanNpmPackages removes any packages in denyList from packages
func CleanNpmPackages(packages map[string]string, denyList []string) map[string]string {
	for _, p := range denyList {
		_, exists := packages[p]
		if exists {
			delete(packages, p)
		}
	}
	return packages
}

// Unmarshal parses the file cfgPath into the given project struct.
func Unmarshal(cfgPath string, project interface{}) error {
	if cfgPath != "" {
		name := strings.TrimSuffix(filepath.Base(cfgPath), filepath.Ext(cfgPath)) // config name without extension
		viper.SetConfigName(name)
		viper.AddConfigPath(filepath.Dir(cfgPath))
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to locate project config: %v", err)
		}
	}

	return viper.Unmarshal(&project, func(decodeCfg *mapstructure.DecoderConfig) {
		decodeCfg.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			func(in reflect.Kind, out reflect.Kind, v interface{}) (interface{}, error) {
				return expandEnv(v), nil
			},
		)
	})
}

func expandEnv(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.String:
		return os.ExpandEnv(v.(string))
	case reflect.Slice:
		if val, ok := v.([]string); ok {
			var strs []string
			for _, item := range val {
				strs = append(strs, os.ExpandEnv(item))
			}
			return strs
		}
		if val, ok := v.([]interface{}); ok {
			var items []interface{}
			for _, item := range val {
				items = append(items, expandEnv(item))
			}
			return items
		}
	case reflect.Map:
		if mp, ok := v.(map[string]string); ok {
			for key, val := range mp {
				mp[key] = os.ExpandEnv(val)
			}
			return mp
		}
		if mp, ok := v.(map[string]interface{}); ok {
			for key, val := range mp {
				mp[key] = expandEnv(val)
			}
			return mp
		}
		if mp, ok := v.(map[interface{}]interface{}); ok {
			for key, val := range mp {
				mp[key] = expandEnv(val)
			}
			return mp
		}
	}
	return v
}

// SetDefaults updates tunnel default values
func (t *Tunnel) SetDefaults() {
	if t.ID != "" {
		log.Warn().Msg("tunnel.id has been deprecated, please use tunnel.name instead")
		t.Name = t.ID
	}
	if t.Parent != "" {
		log.Warn().Msg("tunnel.parent has been deprecated, please use tunnel.owner instead")
		t.Owner = t.Parent
	}
}

// ShouldDownloadArtifact returns true if it should download artifacts, otherwise false
func ShouldDownloadArtifact(jobID string, passed, timedOut, async bool, cfg ArtifactDownload) bool {
	if jobID == "" || timedOut || async {
		return false
	}
	if cfg.When == WhenAlways {
		return true
	}
	if cfg.When == WhenFail && !passed {
		return true
	}
	if cfg.When == WhenPass && passed {
		return true
	}

	return false
}

// GetSuiteArtifactFolder returns a target folder that's based on a combination of suiteName and the configured artifact
// download folder.
// The suiteName is sanitized by undergoing character replacements that are safe to be used as a directory name.
// If the determined target directory already exists, a running number is added as a suffix.
func GetSuiteArtifactFolder(suiteName string, cfg ArtifactDownload) (string, error) {
	suiteName = strings.NewReplacer("/", "-", "\\", "-", ".", "-", " ", "_").Replace(suiteName)
	// If targetDir doesn't exist, no need to find maxVersion and return
	targetDir := filepath.Join(cfg.Directory, suiteName)
	if _, err := os.Open(targetDir); os.IsNotExist(err) {
		return targetDir, nil
	}
	// Find the maxVersion of downloaded artifacts in artifacts dir
	f, err := os.Open(cfg.Directory)
	if err != nil {
		return "", nil
	}
	files, err := f.ReadDir(0)
	if err != nil {
		return "", err
	}
	maxVersion := 0
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		fileName := strings.Split(file.Name(), ".")
		if len(fileName) != 2 || fileName[0] != suiteName {
			continue
		}

		version, err := strconv.Atoi(fileName[1])
		if err != nil {
			return "", err
		}
		if version > maxVersion {
			maxVersion = version
		}
	}
	suiteName = fmt.Sprintf("%s.%d", suiteName, maxVersion+1)

	return filepath.Join(cfg.Directory, suiteName), nil
}
