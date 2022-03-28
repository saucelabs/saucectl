package framework

import (
	"context"
	"errors"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/saucelabs/saucectl/internal/node"
)

type MetadataSearchStrategy interface {
	Find(ctx context.Context, svc MetadataService, frameworkName string, searchValue string) (Metadata, error)
}

type ExactStrategy struct {
}

type PackageStrategy struct {
}

func (s ExactStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, frameworkVersion string) (Metadata, error) {
	return svc.Search(ctx, SearchOptions{
		Name:             frameworkName,
		FrameworkVersion: frameworkVersion,
	})
}

func (s PackageStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, packageJsonPath string) (Metadata, error) {
	p, err := node.PackageFromFile(packageJsonPath)

	if err != nil {
		// TODO: Handle unreadable package.json
	}

	var ver string
	var ok bool
	ver, ok = p.DevDependencies["cypress"]
	if !ok {
		ver, ok = p.Dependencies["cypress"]
		if !ok {
			// TODO: Cypress not defined anywheref
		}
	}

	allVersions, err := svc.Versions(context.Background(), frameworkName)
	if err != nil {
		// TODO: Handle error fetching from service
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

	return Metadata{}, errors.New("unsupported version")
}

func NewSearchStrategy(version string) MetadataSearchStrategy {
	if strings.Contains(version, "package.json") {
		return PackageStrategy{}
	} else {
		return ExactStrategy{}
	}
}
