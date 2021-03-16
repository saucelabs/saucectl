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

// Archive archives the resource and exclude files and folders based on sauceignore logic.
func Archive(src string, matcher sauceignore.Matcher) (io.Reader, error) {
	bb := new(bytes.Buffer)
	w := tar.NewWriter(bb)
	defer w.Close()

	infoSrc, err := os.Stat(src)
	if err != nil {
		return nil, err
	}

	baseDir := ""
	if infoSrc.IsDir() {
		baseDir = filepath.Base(src)
	}

	walker := func(file string, fileInfo os.FileInfo, err error) error {
		// Only will be applied if we have .sauceignore file and have patterns to exclude files and folders
		if matcher.Match(strings.Split(file, string(os.PathSeparator)), fileInfo.IsDir()) {
			return nil
		}

		header, err := tar.FileInfoHeader(fileInfo, file)
		if err != nil {
			return err
		}

		// Elevate permissions.
		header.Uid = 1000
		header.Gid = 1000
		header.Mode = 0777

		if baseDir != "" {
			// Update the name to correctly reflect the desired destination when untaring
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(file, src))
		}

		if err := w.WriteHeader(header); err != nil {
			return err
		}

		if fileInfo.IsDir() {
			return nil
		}

		srcFile, err := os.Open(file)
		if err != nil {
			return err
		}
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
