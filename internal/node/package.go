package node

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
)

type Package struct {
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	DevDependencies  map[string]string `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
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

// Requirements returns a list of all root dependencies for the given packages, including themselves.
func Requirements(root string, pkgs ...string) []string {
	rootDeps := make(map[string]struct{})

	log.Info().Msgf("Getting requirements for npm dependencies %s", pkgs)
	for _, pkg := range pkgs {
		requirements(rootDeps, root, pkg, true)
	}

	var deps []string
	for k := range rootDeps {
		deps = append(deps, k)
	}
	return deps
}

// requirements recursively traverses the dependency tree for the given package, adding all root (shared) dependencies
// to the rootDeps map.
func requirements(rootDeps map[string]struct{}, root, pkg string, rootDep bool) {
	if _, ok := rootDeps[pkg]; ok {
		return
	}

	log.Debug().Msgf("Getting requirements for %s", pkg)
	p, err := PackageFromFile(filepath.Join(root, pkg, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			// likely an unmet peer dependency, not necessarily an issue
			log.Debug().Msgf("Unmet requirement %s", pkg)
			return
		}
		log.Err(err).Msgf("Failed to get requirements for %s", pkg)
		return
	}
	if rootDep {
		rootDeps[pkg] = struct{}{}
	}

	var requiredDeps []string
	for k := range p.Dependencies {
		requiredDeps = append(requiredDeps, k)
	}
	for k := range p.PeerDependencies {
		requiredDeps = append(requiredDeps, k)
	}

	for _, v := range requiredDeps {
		// check for nested dependencies (deps that conflict with requirements, hence embedded in a nested node_modules)
		submodule := filepath.Join(pkg, "node_modules", v)
		_, err = os.Stat(filepath.Join(root, submodule))
		if os.IsNotExist(err) {
			// doesn't exist, so must be a root dependency
			requirements(rootDeps, root, v, true)
			continue
		}

		if err != nil {
			log.Err(err).Msgf("Failed to inspect dependencies for %s", pkg)
			continue
		}

		// Dependency exists in a nested node_modules. We don't need to add it to the list of requirements, but we do
		// need to recurse into the node_modules to get its dependencies, since those might refer to other root deps.
		requirements(rootDeps, root, submodule, false)
	}
}
