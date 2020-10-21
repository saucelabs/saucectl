package credentials

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/yaml"
	"golang.org/x/net/context"
	yamlbase "gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Credentials contains a set of Username + AccessKey for SauceLabs.
type Credentials struct {
	Username  string `yaml:"username"`
	AccessKey string `yaml:"accessKey"`
}

// GetCredentials returns the currently configured credentials (env is prioritary vs. file).
func GetCredentials() Credentials {
	envCredentials := GetCredentialsFromEnv()
	if !envCredentials.IsEmpty() {
		return envCredentials
	}

	configDir := getCredentialsFolderPath()
	err := os.MkdirAll(configDir, 0700)
	if err != nil {
		log.Warn().Msgf("Unable to create configuration folder")
		return Credentials{}
	}
	return GetCredentialsFromFile()
}


// GetCredentialsFromEnv reads the credentials from the user environment.
func GetCredentialsFromEnv() Credentials {
	return Credentials{
		Username:  os.Getenv("SAUCE_USERNAME"),
		AccessKey: os.Getenv("SAUCE_ACCESS_KEY"),
	}
}

// GetCredentialsFromFile reads the credentials from the user credentials file.
func GetCredentialsFromFile() Credentials {
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

// Store stores the provided credentials into the user config.
func (credentials *Credentials) Store() error {
	err := os.MkdirAll(getCredentialsFolderPath(), 0700)
	if err != nil {
		return fmt.Errorf("unable to create configuration folder")
	}
	return yaml.WriteFileWithFileMode(getCredentialsFilePath(), credentials, 0600)
}

// IsEmpty ensure credentials are not set
func (credentials *Credentials) IsEmpty() bool {
	return credentials.AccessKey == "" || credentials.Username == ""
}

// IsValid validates that the credentials are valid.
func (credentials *Credentials) IsValid() bool {
	if  credentials.IsEmpty() {
		return false
	}
	httpClient := http.Client{}
	ctx, _ := context.WithTimeout(context.Background(), 5 * time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://saucelabs.com/rest/v1/users/" + credentials.Username, nil)
	if err != nil {
		log.Error().Msgf("unable to check credentials")
		return false
	}
	req.SetBasicAuth(credentials.Username, credentials.AccessKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error().Msgf("unable to check credentials")
		return false
	}
	return resp.StatusCode == 200
}