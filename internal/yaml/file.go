package yaml

import (
	"os"

	"gopkg.in/yaml.v2"
)

// WriteFile serializes v to a file with the given name.
func WriteFile(name string, v interface{}, mode os.FileMode) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}

	return os.WriteFile(name, b, mode)
}
