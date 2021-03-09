package tar

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/saucelabs/saucectl/internal/sauceignore"
)

// TarResource archives the resource and exclude files and folders based on sauceignore logic.
func TarResource(src string, matcher sauceignore.Matcher) (io.Reader, error) {
	bb := new(bytes.Buffer)
	w := tar.NewWriter(bb)
	defer w.Close()

	walker := func(file string, fileInfo os.FileInfo, err error) error {
		// Only will be applied if we have .sauceignore file and have patterns to exclude files and folders
		if matcher.Match(strings.Split(file, string(os.PathSeparator)), fileInfo.IsDir()) {
			return nil
		}

		header, err := tar.FileInfoHeader(fileInfo, file)
		if err != nil {
			return err
		}

		relFilePath := file
		if filepath.IsAbs(src) {
			// copy temp files
			if strings.Contains(src, os.TempDir()) {
				relFilePath = filepath.Base(file)
			} else {
				relFilePath, err = filepath.Rel(src, file)
				if err != nil {
					return err
				}
			}
		}

		header.Name = relFilePath

		if err := w.WriteHeader(header); err != nil {
			return err
		}

		if fileInfo.Mode().IsDir() {
			return nil
		}

		srcFile, err := os.Open(file)
		defer srcFile.Close()

		_, err = io.Copy(w, srcFile)
		if err != nil {
			return err
		}

		return nil
	}

	if err := filepath.Walk(src, walker); err != nil {
		return nil, err
	}

	return bytes.NewReader(bb.Bytes()), nil
}
