package sauceignore

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// ExcludeSauceIgnorePatterns excludes any match with sauceignore content.
// If loading and parsing the sauceignore content fails, no filtering is applied.
func ExcludeSauceIgnorePatterns(files []string, sauceignoreFile string) []string {
	fmt.Println("files: ", files)
	matcher, err := NewMatcherFromFile(sauceignoreFile)
	if err != nil {
		log.Warn().Err(err).Msgf("An error occurred when filtering specs with %s. No filter will be applied", sauceignoreFile)
		return files
	}

	var selectedFiles []string
	for _, filename := range files {
		fmt.Println("checking matched file: ", strings.Split(filename, string(filepath.Separator)))
		if !matcher.Match(strings.Split(filename, "/"), false) {
			fmt.Printf("file %s should not be ignored \n", strings.Split(filename, string(filepath.Separator)))
			selectedFiles = append(selectedFiles, filename)
		}
	}
	return selectedFiles
}
