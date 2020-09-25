package utils

import (
	"errors"
	"os"
	"path/filepath"
        "bufio"
	"strings"
)

// SplitLines convert string into []string based on line-breaks
func SplitLines(s string) ([]string) {
	var l []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		l = append(l, sc.Text())
	}
	return l
}
// ValidateOutputPath validates the output paths of the `export` and `save` commands.
func ValidateOutputPath(path string) error {
	dir := filepath.Dir(filepath.Clean(path))
	if dir != "" && dir != "." {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return errors.New("invalid output path: directory " + dir + " does not exist")
		}
	}
	// check whether `path` points to a regular file
	// (if the path exists and doesn't point to a directory)
	if fileInfo, err := os.Stat(path); !os.IsNotExist(err) {
		if err != nil {
			return err
		}

		if fileInfo.Mode().IsDir() || fileInfo.Mode().IsRegular() {
			return nil
		}

		if err := ValidateOutputPathFileMode(fileInfo.Mode()); err != nil {
			return errors.New("invalid output path: " + path + " must be a directory or a regular file")
		}
	}
	return nil
}

// ValidateOutputPathFileMode validates the output paths of the `cp` command and serves as a
// helper to `ValidateOutputPath`
func ValidateOutputPathFileMode(fileMode os.FileMode) error {
	switch {
	case fileMode&os.ModeDevice != 0:
		return errors.New("got a device")
	case fileMode&os.ModeIrregular != 0:
		return errors.New("got an irregular file")
	}
	return nil
}
