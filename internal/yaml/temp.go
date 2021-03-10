package yaml

import (
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

// TempFile serializes v to a file with the given name and stores it in a temp directory.
// Returns the path to the temp file if successful.
func TempFile(name string, v interface{}) (string, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}

	d, err := os.MkdirTemp("", "tempy")
	if err != nil {
		return "", err
	}

	tpath := filepath.Join(d, name)
	if err := os.WriteFile(tpath, b, os.ModePerm); err != nil {
		return "", err
	}

	return tpath, nil
}
