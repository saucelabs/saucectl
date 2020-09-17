package yaml

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

// WriteFile serializes v to a file with the given name.
func WriteFile(name string, v interface{}) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(name, b, os.ModePerm)
}
