package yaml

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

// WriteFile serializes v to a file with the given name.
func WriteFile(name string, v interface{}) error {
	return WriteFileWithFileMode(name, v, os.ModePerm)
}

// WriteFileWithFileMode serializes v to a file with the given name with specified FileMode.
func WriteFileWithFileMode(name string, v interface{}, mode os.FileMode) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(name, b, mode)
}
