package node

import (
	"encoding/json"
	"os"
)

type Package struct {
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}

func PackageFromFile(filename string) (Package, error) {
	var p Package

	fd, err := os.Open(filename)
	if err != nil {
		return p, err 
	}

	err = json.NewDecoder(fd).Decode(&p)
	return p, err
}
