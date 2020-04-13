package runner

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

type ciRunner struct {
	baseRunner
}

func (r ciRunner) Setup() error {
	// copy files from repository into target dir
	for _, pattern := range r.config.Files {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, file := range matches {
			if err := copyFile(file, targetDir); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r ciRunner) Run() (int, error) {
	cmd := exec.Command("npm", "test")

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	if err != nil {
		return 1, err
	}
	return 0, nil
}

func (r ciRunner) Teardown(logDir string) error {
	if logDir != "" {
		return nil
	}

	for _, containerSrcPath := range logFiles {
		file := filepath.Base(containerSrcPath)
		dstPath := filepath.Join(logDir, file)
		if err := copyFile(containerSrcPath, dstPath); err != nil {
			continue
		}
	}

	return nil
}

func copyFile(src string, targetDir string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	srcFile := src
	if !path.IsAbs(srcFile) {
		srcFile = filepath.Join(pwd, src)
	}

	input, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(targetDir+filepath.Base(srcFile), input, 0644)
	if err != nil {
		return err
	}

	return nil
}
