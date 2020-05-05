package utils

import (
	"errors"
	"os"
	"time"
	"fmt"
	"sync"
	"path/filepath"

	"github.com/briandowns/spinner"
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

type SpinnerSingleton struct {
	Spinner *spinner.Spinner
	Speed time.Duration
	Animation []string
}

var lock = &sync.Mutex{}
var spinnerInstance *SpinnerSingleton

func NewSpinner() (*SpinnerSingleton) {
	// Using a singleton so we can stop previous messages
	// otherwise multiple message show in stdout
	if spinnerInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if spinnerInstance == nil {
			spinnerInstance = &SpinnerSingleton{}
			spinnerInstance.Speed = 300*time.Millisecond
			spinnerInstance.Animation = []string{"⠋ ", "⠙ ", "⠹ ","⠸ ","⠼ ","⠴ ", "⠦ ", "⠧ ", "⠇ ", "⠏ "}
			spinnerInstance.Spinner = spinner.New(spinnerInstance.Animation, spinnerInstance.Speed)
		}
	}
	return spinnerInstance
}

func Spinner(text string, args ...interface{}) *spinner.Spinner {
	message := fmt.Sprintf(text, args...)
	spinner := NewSpinner()
	spinner.Spinner.Suffix = message
	spinner.Spinner.Stop()
	spinner.Spinner.Start()
	return spinner.Spinner
}
