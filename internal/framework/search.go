package framework

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/saucelabs/saucectl/internal/node"
)

// FrameworkUnavailableError is an error type returned if the requested framework version is unavailable.
type FrameworkUnavailableError struct {
	Name    string
	Version string
}

// Error returns the error string
func (e *FrameworkUnavailableError) Error() string {
	s := fmt.Sprintf("Version %s for %s is not available", e.Version, e.Name)
	return s
}

// Misc errors
var (
	ErrServerError      = errors.New("Unable to check framework version availability")
	ErrVersionUndefined = errors.New("Framework version is not defined")
)

// MetadataSearchStrategy is a generic strategy for determining if the requested framework version is supported
type MetadataSearchStrategy interface {
	Find(ctx context.Context, svc MetadataService, frameworkName string, searchValue string) (Metadata, error)
}

// ExactStrategy searches for the metadata of a framework by its exact version
type ExactStrategy struct {
}

// PackageStrategy searches for metadata of a framework from a package.json file. If the requested version is a range (e.g. ~9.0.0), it matches the most recent version that satifies the constraint.
type PackageStrategy struct {
}

func (s ExactStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, frameworkVersion string) (Metadata, error) {
	if frameworkVersion == "" {
		return Metadata{}, ErrVersionUndefined
	}

	m, err := svc.Search(ctx, SearchOptions{
		Name:             frameworkName,
		FrameworkVersion: frameworkVersion,
	})

	if err != nil && strings.Contains(err.Error(), "Bad Request: unsupported version") {
		return Metadata{}, &FrameworkUnavailableError{
			Name:    frameworkName,
			Version: frameworkVersion,
		}
	}

	if err != nil {
		return Metadata{}, ErrServerError
	}

	return m, nil
}

func toNpmPackageName(frameworkName string) string {
	mapping := map[string]string{
		"cypress":    "cypress",
		"playwright": "@playwright/test",
		"testcafe":   "testcafe",
	}

	p, ok := mapping[frameworkName]
	if !ok {
		return frameworkName
	}

	return p
}

func (s PackageStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, packageJsonPath string) (Metadata, error) {
	p, err := node.PackageFromFile(packageJsonPath)

	if err != nil {
		return Metadata{}, fmt.Errorf("Error reading package.json: %w", err)
	}

	var ver string
	var ok bool
	packageName := toNpmPackageName(frameworkName)
	ver, ok = p.DevDependencies[packageName]
	if !ok {
		ver, ok = p.Dependencies[packageName]
		if !ok {
			return Metadata{}, ErrVersionUndefined
		}
	}

	allVersions, err := svc.Versions(context.Background(), frameworkName)
	if err != nil {
		return Metadata{}, ErrServerError
	}

	sort.Slice(allVersions, func(i, j int) bool {
		vi, err := semver.NewVersion(allVersions[i].FrameworkVersion)
		if err != nil {
			return false
		}

		vj, err := semver.NewVersion(allVersions[j].FrameworkVersion)
		if err != nil {
			return false
		}

		// sort in descending order
		return vi.GreaterThan(vj)
	})

	c, err := semver.NewConstraint(ver)
	for _, v := range allVersions {
		sv, err := semver.NewVersion(v.FrameworkVersion)
		if err != nil {
			continue
		}

		if c.Check(sv) {
			return v, nil
		}
	}

	return Metadata{}, &FrameworkUnavailableError{
		Name:    frameworkName,
		Version: ver,
	}
}

// NewSearchStrategy returns a concrete MetadataSearchStrategy
func NewSearchStrategy(version string) MetadataSearchStrategy {
	if strings.Contains(version, "package.json") {
		return PackageStrategy{}
	} else {
		return ExactStrategy{}
	}
}
