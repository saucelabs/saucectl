package zip

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/human"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/node"
	"github.com/saucelabs/saucectl/internal/sauceignore"
)

// ArchiveFileCountSoftLimit is the threshold count of files added to the archive
// before a warning is printed.
// The value here (2^15) is somewhat arbitrary. In testing, ~32K files in the archive
// resulted in about 30s for download and extraction.
const ArchiveFileCountSoftLimit = 32768

// BaseFilepathLength represents the path length where project will be unpacked.
// Example: "D:\sauce-playwright-runner\1.12.0\bundle\__project__\"
const BaseFilepathLength = 53

// MaxFilepathLength represents the maximum path length acceptable.
const MaxFilepathLength = 255

// ArchiveRunnerConfig compresses runner config into `config.zip`.
func ArchiveRunnerConfig(project interface{}, tempDir string) (string, error) {
	zipName := filepath.Join(tempDir, "config.zip")
	z, err := zip.NewFileWriter(zipName, sauceignore.NewMatcher([]sauceignore.Pattern{}))
	if err != nil {
		return "", err
	}
	defer z.Close()

	rcPath := filepath.Join(tempDir, "sauce-runner.json")
	if err := jsonio.WriteFile(rcPath, project); err != nil {
		return "", err
	}

	_, _, err = z.Add(rcPath, "")
	if err != nil {
		return "", err
	}
	return zipName, nil
}

// ArchiveFiles walks through sourceDir, collects specified files, compresses them into target dir and returns the zip file path.
func ArchiveFiles(targetFileName string, targetDir string, sourceDir string, files []string, matcher sauceignore.Matcher) (string, error) {
	start := time.Now()

	zipName := filepath.Join(targetDir, targetFileName+".zip")
	z, err := zip.NewFileWriter(zipName, matcher)
	if err != nil {
		return "", err
	}
	defer z.Close()

	totalFileCount := 0
	longestPathLength := 0

	// Keep file order stable for consistent zip archives
	sort.Strings(files)
	for _, f := range files {
		rel, err := filepath.Rel(sourceDir, filepath.Dir(f))
		if err != nil {
			return "", err
		}
		fileCount, length, err := z.Add(f, rel)
		if err != nil {
			return "", err
		}
		totalFileCount += fileCount
		if length > longestPathLength {
			longestPathLength = length
		}
	}

	err = z.Close()
	if err != nil {
		return "", err
	}

	f, err := os.Stat(zipName)
	if err != nil {
		return "", err
	}

	log.Info().
		Str("duration", time.Since(start).Round(time.Second).String()).
		Str("size", human.Bytes(f.Size())).
		Int("fileCount", totalFileCount).
		Int("longestPathLength", longestPathLength).
		Msg("Archive created.")

	if totalFileCount >= ArchiveFileCountSoftLimit {
		msg.LogArchiveSizeWarning()
	}

	if longestPathLength+BaseFilepathLength > MaxFilepathLength {
		msg.LogArchivePathLengthWarning(MaxFilepathLength - BaseFilepathLength)
	}

	return zipName, nil
}

// ArchiveNodeModules collects npm dependencies from sourceDir and compresses them into targetDir.
func ArchiveNodeModules(targetDir string, sourceDir string, matcher sauceignore.Matcher, dependencies []string) (string, error) {
	dependencies, err := ExpandDependencies(sourceDir, dependencies)
	if err != nil {
		return "", err
	}

	var files []string
	wantMods := len(dependencies) > 0
	// does the user only want a subset of dependencies?
	if wantMods {
		reqs := node.Requirements(filepath.Join(sourceDir, "node_modules"), dependencies...)
		if len(reqs) == 0 {
			return "", fmt.Errorf("unable to find required dependencies; please check 'node_modules' folder and make sure the dependencies exist")
		}
		log.Info().Msgf("Found a total of %d related npm dependencies", len(reqs))
		for _, v := range reqs {
			files = append(files, filepath.Join(sourceDir, "node_modules", v))
		}
	}

	// node_modules exists, has not been ignored and a subset has not been specified, so include the entire folder.
	// This is the legacy behavior (backwards compatible) of saucectl.
	if !wantMods {
		log.Warn().Msg("Adding the entire node_modules folder to the payload. " +
			"This behavior is deprecated, not recommended and will be removed in the future. " +
			"Please address your dependency needs via https://docs.saucelabs.com/dev/cli/saucectl/usage/use-cases/#including-node-dependencies")
		files = append(files, filepath.Join(sourceDir, "node_modules"))
	}

	return ArchiveFiles("node_modules", targetDir, sourceDir, files, matcher)
}

// ExpandDependencies looks for "package.json" files inside dependencies and
// expands them into a list of dependencies.
func ExpandDependencies(sourceDir string, dependencies []string) ([]string, error) {
	var expanded []string
	for _, dep := range dependencies {
		if strings.HasSuffix(dep, "package.json") {
			p, err := node.PackageFromFile(filepath.Join(sourceDir, dep))
			if err != nil {
				return nil, fmt.Errorf("failed to read dependencies from %s: %w", dep, err)
			}

			for k := range p.Dependencies {
				expanded = append(expanded, k)
			}
			for k := range p.DevDependencies {
				expanded = append(expanded, k)
			}
			continue
		}

		expanded = append(expanded, dep)
	}

	return expanded, nil
}
