package credentials

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/yaml"
	yamlbase "gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

// Credentials contains a set of Username + AccessKey for SauceLabs.
type Credentials struct {
	Username  string `yaml:"username"`
	AccessKey string `yaml:"accessKey"`
	Source    string
}

// Get returns the currently configured credentials (env is prioritary vs. file).
func Get() *Credentials {
	if envCredentials := FromEnv(); envCredentials != nil {
		return envCredentials
	}
	return FromFile()
}

// FromEnv reads the credentials from the user environment.
func FromEnv() *Credentials {
	username, usernamePresence := os.LookupEnv("SAUCE_USERNAME")
	accessKey, accessKeyPresence := os.LookupEnv("SAUCE_ACCESS_KEY")

	if usernamePresence && accessKeyPresence && len(username) > 0 && len(accessKey) > 0 {
		return &Credentials{
			Username:  username,
			AccessKey: accessKey,
			Source: "environment variables",
		}
	}
	return nil
}

// FromFile reads the credentials from the user credentials file.
func FromFile() *Credentials {
	var c *Credentials

	folderPath, err := getCredentialsFolderPath()
	if err != nil {
		return nil
	}
	filePath, err := getCredentialsFilePath()
	if err != nil {
		return nil
	}

	if _, err := os.Stat(folderPath); err != nil {
		log.Debug().Msgf("%s: config folder does not exists: %v", filePath, err)
		return nil
	}

	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Info().Msgf("failed to read credentials: %v", err)
		}
		return nil
	}

	if err = yamlbase.Unmarshal(yamlFile, &c); err != nil {
		log.Info().Msgf("failed to parse credentials: %v", err)
		return nil
	}
	c.Source, err = getCredentialsFilePath()
	return c
}

func getCredentialsFolderPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".sauce"), nil
}

func getCredentialsFilePath() (string, error) {
	credentialsDir, err := getCredentialsFolderPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(credentialsDir, "credentials.yml"), nil
}

// Store stores the provided credentials into the user config.
func (c *Credentials) Store() error {
	folderPath, err := getCredentialsFolderPath()
	if err != nil {
		return nil
	}
	filePath, err := getCredentialsFilePath()
	if err != nil {
		return nil
	}

	err = os.MkdirAll(folderPath, 0700)
	if err != nil {
		return fmt.Errorf("unable to create configuration folder")
	}
	return yaml.WriteFile(filePath, c, 0600)
}

// IsEmpty ensure credentials are not set
func (c *Credentials) IsEmpty() bool {
	return c.AccessKey == "" || c.Username == ""
}

// IsValid validates that the credentials are valid.
func (c *Credentials) IsValid() bool {
	return !c.IsEmpty()
	// FIXME this is wrong, since credentials are region specific
	//httpClient := http.Client{}
	//ctx, _ := context.WithTimeout(context.Background(), 5 * time.Second)
	//req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://saucelabs.com/rest/v1/users/" + c.Username, nil)
	//if err != nil {
	//	log.Error().Msgf("unable to check c")
	//	return false
	//}
	//req.SetBasicAuth(c.Username, c.AccessKey)
	//resp, err := httpClient.Do(req)
	//if err != nil {
	//	log.Error().Msgf("unable to check c")
	//	return false
	//}
	//return resp.StatusCode == 200
}