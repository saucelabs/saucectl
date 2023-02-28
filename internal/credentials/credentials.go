package credentials

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/yaml"
	yamlbase "gopkg.in/yaml.v2"
)

// Get returns the configured credentials.
// Effectively a covenience wrapper around FromEnv, followed by a call to FromFile.
//
// The lookup order is:
//  1. Environment variables (see FromEnv)
//  2. Credentials file (see FromFile)
func Get() iam.Credentials {
	if c := FromEnv(); c.IsSet() {
		return c
	}

	return FromFile()
}

// FromEnv reads the credentials from the user environment.
func FromEnv() iam.Credentials {
	return iam.Credentials{
		Username:  os.Getenv("SAUCE_USERNAME"),
		AccessKey: os.Getenv("SAUCE_ACCESS_KEY"),
	}
}

// FromFile reads the credentials that stored in the default file location.
func FromFile() iam.Credentials {
	return fromFile(defaultFilepath())
}

// fromFile reads the credentials from path.
func fromFile(path string) iam.Credentials {
	yamlFile, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// not a real error but a valid usecase when credentials have not been persisted yet
			return iam.Credentials{}
		}

		log.Error().Msgf("failed to read credentials: %v", err)
		return iam.Credentials{}
	}
	defer yamlFile.Close()

	var c iam.Credentials
	if err = yamlbase.NewDecoder(yamlFile).Decode(&c); err != nil {
		log.Error().Msgf("failed to parse credentials: %v", err)
		return iam.Credentials{}
	}

	return c
}

// ToFile stores the provided credentials in the default file location.
func ToFile(c iam.Credentials) error {
	return toFile(c, defaultFilepath())
}

// toFile stores the provided credentials into the file at path.
func toFile(c iam.Credentials, path string) error {
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
