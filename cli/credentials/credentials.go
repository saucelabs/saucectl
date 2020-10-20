package credentials

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/yaml"
	yamlbase "gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Credentials contains a set of Username + AccessKey for SauceLabs.
type Credentials struct {
	Username  string `yaml:"username"`
	AccessKey string `yaml:"accessKey"`
}

// GetCredentials returns the currently configured credentials (env is prioritary vs. file).
func GetCredentials() Credentials {
	if os.Getenv("SAUCE_USERNAME") != "" && os.Getenv("SAUCE_ACCESS_KEY") != "" {
		return Credentials{
			Username:  os.Getenv("SAUCE_USERNAME"),
			AccessKey: os.Getenv("SAUCE_ACCESS_KEY"),
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Warn().Msgf("Unable to locate configuration folder")
		return Credentials{}
	}

	err = os.MkdirAll(filepath.Join(homeDir, ".sauce"), 0700)
	if err != nil {
		log.Warn().Msgf("Unable to create configuration folder")
		return Credentials{}
	}

	return credentialsFromFile(filepath.Join(homeDir, ".sauce", "config.yml"))
}

// StoreCredentials stores the provided credentials into the user config.
func StoreCredentials(credentials Credentials) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("unable to locate home folder")
	}

	err = os.MkdirAll(filepath.Join(homeDir, ".sauce"), 0700)
	if err != nil {
		return fmt.Errorf("unable to create configuration folder")
	}
	fmt.Printf("%s / %s\n", credentials.Username, credentials.AccessKey)
	return yaml.WriteFile(filepath.Join(homeDir, ".sauce", "config.yml"), credentials)
}

func credentialsFromFile(credentialsFilePath string) Credentials {
	var c Credentials

	yamlFile, err := ioutil.ReadFile(credentialsFilePath)
	if err != nil {
		log.Info().Msgf("failed to locate credentials: %v", err)
		return Credentials{}
	}

	if err = yamlbase.Unmarshal(yamlFile, &c); err != nil {
		log.Info().Msgf("failed to parse credentials: %v", err)
		return Credentials{}
	}
	return c
}
