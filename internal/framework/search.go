package framework

import "context"

type MetadataSearchStrategy interface {
	Find(ctx context.Context, svc MetadataService, frameworkName string, searchValue string) (Metadata, error)
}

type ExactStrategy struct {
}

type PackageStrategy struct {
	packageJsonFilePath string
}

func (s ExactStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, searchValue string) (Metadata, error) {
	return svc.Search(ctx, SearchOptions{
		Name:             frameworkName,
		FrameworkVersion: searchValue,
	})
}

func (s PackageStrategy) Find(ctx context.Context, svc MetadataService, frameworkName string, searchValue string) (Metadata, error) {
// 	p, err := node.PackageFromFile(s.packageJsonFilePath)
// 	if err != nil {
// 		// TODO: Handle unreadable package.json
// 	}
// 
// 	var ver string
// 	var ok bool
// 	ver, ok = p.DevDependencies["cypress"]
// 	if ok {
// 		// p.Cypress.Version = ver
// 	}
// 
// 		svc.Versions()
	return Metadata{}, nil
}

func NewSearchStrategy(version string) (MetadataSearchStrategy) {
	if version == "package.json" {
		return PackageStrategy{}
	} else {
		return ExactStrategy{}
	}
}
