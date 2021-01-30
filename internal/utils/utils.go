package utils

import (
	"errors"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
)

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

// GetProjectDir gets the home directory.
func GetProjectDir() string {
	if (os.Getenv("SAUCE_ROOT_DIR") != "") {
		return os.Getenv("SAUCE_ROOT_DIR")
	} else if (os.Getenv("SAUCE_VM") == "") {
		return "/home/seluser"
	}
	workingDir, err := os.Getwd()
	if (err != nil) {
		log.Warn().Msg("Could not get current working directory. Defaulting to /home/seluser")
		return "/home/seluser"
	}
	return workingDir
}

