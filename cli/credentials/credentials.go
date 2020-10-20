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

	configDir := getCredentialsFolderPath()
	err := os.MkdirAll(configDir, 0700)
	if err != nil {
		log.Warn().Msgf("Unable to create configuration folder")
		return Credentials{}
	}
	return GetCredentialsFromConfig()
}

// StoreCredentials stores the provided credentials into the user config.
func StoreCredentials(credentials Credentials) error {
	err := os.MkdirAll(getCredentialsFolderPath(), 0700)
	if err != nil {
		return fmt.Errorf("unable to create configuration folder")
	}
	fmt.Printf("%s / %s\n", credentials.Username, credentials.AccessKey)
	return yaml.WriteFile(getCredentialsFilePath(), credentials)
}

// GetCredentialsFromConfig reads the credentials from the user config.
func GetCredentialsFromConfig() Credentials {
	var c Credentials

	yamlFile, err := ioutil.ReadFile(getCredentialsFilePath())
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

func getCredentialsFolderPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/"
		log.Warn().Msgf("unable to locate home folder")
	}
	return homeDir
}

func getCredentialsFilePath() string {
	homeDir := getCredentialsFolderPath()
	return filepath.Join(homeDir, ".sauce", "credentials.yml")
}