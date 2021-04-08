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
	Source    string `yaml:"-"`
}

// Get returns the configured credentials.
//
// The lookup order is:
//  1. Environment variables
//  2. Credentials file
func Get() Credentials {
	if c := FromEnv(); c.IsValid() {
		return c
	}
	return FromFile()
}

// FromEnv reads the credentials from the user environment.
func FromEnv() Credentials {
	return Credentials{
		Username:  os.Getenv("SAUCE_USERNAME"),
		AccessKey: os.Getenv("SAUCE_ACCESS_KEY"),
		Source:    "environment variables",
	}
}

// FromFile reads the credentials from the user credentials file.
func FromFile() Credentials {
	filePath, err := getFilepath()
	if err != nil {
		return Credentials{}
	}

	if _, err := os.Stat(filepath.Dir(filePath)); err != nil {
		log.Debug().Msgf("%s: config folder does not exists: %v", filepath.Dir(filePath), err)
		return Credentials{}
	}

	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error().Msgf("failed to read credentials: %v", err)
			return Credentials{}
		}
		return Credentials{}
	}

	var c Credentials
	if err = yamlbase.Unmarshal(yamlFile, &c); err != nil {
		log.Error().Msgf("failed to parse credentials: %v", err)
		return Credentials{}
	}
	c.Source = filePath

	return c
}

func getFilepath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".sauce", "credentials.yml"), nil
}

// Store stores the provided credentials into the user config.
func (c *Credentials) Store() error {
	filePath, err := getFilepath()
	if err != nil {
		return nil
	}

	err = os.MkdirAll(filepath.Dir(filePath), 0700)
	if err != nil {
		return fmt.Errorf("unable to create configuration folder")
	}
	return yaml.WriteFile(filePath, c, 0600)
}

// IsEmpty checks whether the credentials, i.e. username and access key are not empty.
// Returns false if even one of the credentials is empty.
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
