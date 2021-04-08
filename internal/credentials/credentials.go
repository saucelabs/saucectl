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
// Effectively a covenience wrapper around FromEnv, followed by a call to FromFile.
//
// The lookup order is:
//  1. Environment variables (see FromEnv)
//  2. Credentials file (see FromFile)
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

// FromFile reads the credentials that stored in the default file location.
func FromFile() Credentials {
	return fromFile(defaultFilepath())
}

// fromFile reads the credentials from path.
func fromFile(path string) Credentials {
	yamlFile, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// not a real error but a valid usecase when credentials have not been persisted yet
			return Credentials{}
		}

		log.Error().Msgf("failed to read credentials: %v", err)
		return Credentials{}
	}
	defer yamlFile.Close()

	var c Credentials
	if err = yamlbase.NewDecoder(yamlFile).Decode(&c); err != nil {
		log.Error().Msgf("failed to parse credentials: %v", err)
		return Credentials{}
	}

	return c
}

// ToFile stores the provided credentials in the default file location.
func ToFile(c Credentials) error {
	return toFile(c, defaultFilepath())
}

// toFile stores the provided credentials into the file at path.
func toFile(c Credentials, path string) error {
	if os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return fmt.Errorf("unable to create configuration folder")
	}
	return yaml.WriteFile(path, c, 0600)
}

// defaultFilepath returns the default location of the credentials file.
// It will be based on the user home directory, if defined, or under the current working directory otherwise.
func defaultFilepath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".sauce", "credentials.yml")
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
	// FIXME nor should a simple struct be calling out to a webservice
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
