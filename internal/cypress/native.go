package cypress

import (
	"encoding/json"
	"os"
)

// Config represents the cypress.json native configuration file.
type Config struct {
	FixturesFolder    string `json:"fixturesFolder,omitempty"`
	IntegrationFolder string `json:"integrationFolder,omitempty"`
	PluginsFile       string `json:"pluginsFile,omitempty"`
	SupportFile       string `json:"supportFile,omitempty"`
}

// ConfigFromFile loads cypress configuration into Config structure.
func ConfigFromFile(fileName string) (Config, error) {
	var c Config

	fd, err := os.Open(fileName)
	if err != nil {
		return c, err
	}
	err = json.NewDecoder(fd).Decode(&c)
	return c, err
}
