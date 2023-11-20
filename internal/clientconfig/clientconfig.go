package clientconfig

import (
	"os"
	"path/filepath"

	"github.com/saucelabs/saucectl/internal/iam"
	"gopkg.in/yaml.v2"
)

// just breaking the dependendency cycle, must be better ways
type RegionConf struct {
	Name             string `yaml:"name"`
	APIBaseURL       string `yaml:"apiBaseURL,omitempty"`
	AppBaseURL       string `yaml:"appBaseURL,omitempty"`
	WebDriverBaseURL string `yaml:"webdriverBaseURL,omitempty"`
}

type ClientConfig struct {
	Regions     []RegionConf               `yaml:"regions,omitempty"`
	Credentials map[string]iam.Credentials `yaml:"credentials,omitempty"`
}

func FromFile(path string) (*ClientConfig, error) {
	// read the contents of the YAML file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// unmarshal the YAML data into a struct
	var config ClientConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

var config *ClientConfig

func Get() (*ClientConfig, error) {
	if config != nil {
		return config, nil
	}
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".sauce", "saucectl.yaml")
	// if file config file does not exist, return nil
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}
	_config, err := FromFile(configPath)
	if err != nil {
		return nil, err
	}
	config = _config
	return config, nil
}
