package framework

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/saucelabs/saucectl/internal/node"
)

// UnavailableError is an error type returned if the requested framework version is unavailable.
type UnavailableError struct {
	Name    string
	Version string
}

// Error returns the error string
func (e *UnavailableError) Error() string {
	s := fmt.Sprintf("version %s for %s is not available", e.Version, e.Name)
	return s
}

// MetadataSearchStrategy is a generic strategy for determining if the requested framework version is supported
type MetadataSearchStrategy interface {
	Find(ctx context.Context, svc MetadataService, frameworkName string, searchValue string) (Metadata, error)
}

// ExactStrategy searches for the metadata of a framework by its exact version
type ExactStrategy struct {
}

// PackageStrategy searches for metadata of a framework from a package.json file.
// If the requested version is a range (e.g. ~9.0.0), it matches the most recent version that satisfies the constraint.
type PackageStrategy struct {
	packageJSONFilePath string
}

func (s ExactStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, frameworkVersion string) (Metadata, error) {
	if frameworkVersion == "" {
		return Metadata{}, errors.New("framework version not defined")
	}

	versions, err := svc.Versions(ctx, frameworkName)
	if err != nil {
		return Metadata{}, fmt.Errorf("unable to determine available versions for framework: %w", err)
	}

	for _, version := range versions {
		// Not looking for any specific version, so pick the latest == first one.
		if frameworkVersion == "latest" {
			return version, nil
		}

		if version.FrameworkVersion == frameworkVersion {
			return version, nil
		}
	}

	return Metadata{}, &UnavailableError{
		Name:    frameworkName,
		Version: frameworkVersion,
	}
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

// Monkey patch functions to allow mocking during unit testing
var (
	PackageFromFile = node.PackageFromFile
	NewConstraint   = semver.NewConstraint
)

func (s PackageStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, _ string) (Metadata, error) {
	p, err := PackageFromFile(s.packageJSONFilePath)

	if err != nil {
		return Metadata{}, fmt.Errorf("error reading package.json: %w", err)
	}

	var ver string
	var ok bool
	packageName := toNpmPackageName(frameworkName)
	ver, ok = p.DevDependencies[packageName]
	if !ok {
		ver, ok = p.Dependencies[packageName]
		if !ok {
			return Metadata{}, fmt.Errorf("unable to determine dependencies for package: %s", packageName)
		}
	}

	allVersions, err := svc.Versions(ctx, frameworkName)
	if err != nil {
		return Metadata{}, fmt.Errorf("unable to determine framework versions: %w", err)
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

	c, err := NewConstraint(ver)
	if err != nil {
		return Metadata{}, fmt.Errorf("unable to parse package version (%s): %w", ver, err)
	}

	for _, v := range allVersions {
		sv, err := semver.NewVersion(v.FrameworkVersion)
		if err != nil {
			continue
		}

		if c.Check(sv) {
			return v, nil
		}
	}

	return Metadata{}, &UnavailableError{
		Name:    frameworkName,
		Version: ver,
	}
}

// NewSearchStrategy returns a concrete MetadataSearchStrategy
func NewSearchStrategy(version string, rootDir string) MetadataSearchStrategy {
	if strings.Contains(version, "package.json") {
		return PackageStrategy{
			packageJSONFilePath: filepath.Join(rootDir, version),
		}
	}

	return ExactStrategy{}
}
