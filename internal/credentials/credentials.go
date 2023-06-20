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

func init() {
	homeDir, _ := os.UserHomeDir()
	DefaultCredsPath = filepath.Join(homeDir, ".sauce", "credentials.yml")
}

// DefaultCredsPath returns the default location of the credentials file.
// It's under the user's home directory, if defined, otherwise under the current working directory.
var DefaultCredsPath = ""

// EnvSource indicates the credentials are retrieved from environment variables.
const EnvSource = "Environment variables"

// ConfigFileSource indicates the credentials are retrieved from configuration file.
const ConfigFileSource = "Configuration file"

// Get returns the configured credentials.
// Effectively a convenience wrapper around FromEnv, followed by a call to FromFile.
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
		Source:    fmt.Sprintf("%s(%s)", EnvSource, "$SAUCE_USERNAME, $SAUCE_ACCESS_KEY"),
	}
}

// FromFile reads the credentials that stored in the default file location.
func FromFile() iam.Credentials {
	return fromFile(DefaultCredsPath)
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

	c.Source = fmt.Sprintf("%s(%s)", ConfigFileSource, path)

	return c
}

// ToFile stores the provided credentials in the default file location.
func ToFile(c iam.Credentials) error {
	return toFile(c, DefaultCredsPath)
}

// toFile stores the provided credentials into the file at path.
func toFile(c iam.Credentials, path string) error {
	if os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return fmt.Errorf("unable to create configuration folder")
	}
	return yaml.WriteFile(path, c, 0600)
}
