package framework

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/saucelabs/saucectl/internal/node"
)

type FrameworkUnavailableError struct {
	Name    string
	Version string
}

func (e *FrameworkUnavailableError) Error() string {
	s := fmt.Sprintf("Version %s for %s is not available", e.Version, e.Name)
	return s
}

var (
	ErrServerError         = errors.New("Unable to check framework version availability")
	ErrPackageJsonNotFound = errors.New("Could not read package.json")
	ErrVersionUndefined    = errors.New("Framework version is not defined")
)

type MetadataSearchStrategy interface {
	Find(ctx context.Context, svc MetadataService, frameworkName string, searchValue string) (Metadata, error)
}

type ExactStrategy struct {
}

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

	if strings.Contains(err.Error(), "Bad Request: unsupported version") {
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

func (s PackageStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, packageJsonPath string) (Metadata, error) {
	p, err := node.PackageFromFile(packageJsonPath)

	if err != nil {
		return Metadata{}, ErrPackageJsonNotFound
	}

	var ver string
	var ok bool
	ver, ok = p.DevDependencies["cypress"]
	if !ok {
		ver, ok = p.Dependencies["cypress"]
		if !ok {
			return Metadata{}, ErrVersionUndefined
		}
	}

	allVersions, err := svc.Versions(context.Background(), frameworkName)
	if err != nil {
		return Metadata{}, ErrServerError
	}

	// TODO: Need to sort allVersions

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

func NewSearchStrategy(version string) MetadataSearchStrategy {
	if strings.Contains(version, "package.json") {
		return PackageStrategy{}
	} else {
		return ExactStrategy{}
	}
}
