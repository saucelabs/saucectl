package cypress

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/saucelabs/saucectl/internal/msg"
)

// Config represents the cypress.json native configuration file.
type Config struct {
	// Path is the location of the config file itself.
	Path string `yaml:"-" json:"-"`

	FixturesFolder    interface{} `json:"fixturesFolder,omitempty"`
	IntegrationFolder string      `json:"integrationFolder,omitempty"`
	PluginsFile       string      `json:"pluginsFile,omitempty"`
	SupportFile       string      `json:"supportFile,omitempty"`
}

// AbsIntegrationFolder returns the absolute path to Config.IntegrationFolder.
func (c Config) AbsIntegrationFolder() string {
	return filepath.Join(filepath.Join(filepath.Dir(c.Path), c.IntegrationFolder))
}

// configFromFile loads cypress configuration into Config structure.
func configFromFile(fileName string) (Config, error) {
	var c Config

	fd, err := os.Open(fileName)
	if err != nil {
		return c, fmt.Errorf(msg.UnableToLocateCypressCfg, fileName)
	}
	err = json.NewDecoder(fd).Decode(&c)
	c.Path = fileName
	return c, err
}
