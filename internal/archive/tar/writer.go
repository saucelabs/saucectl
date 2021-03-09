package tar

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/saucelabs/saucectl/internal/sauceignore"
)

/*
	1. Create .tar archive
	1.1. Remove this temp file
	1.2. Add matcher
	2. Return read closer
*/
type Writer struct {
	W *tar.Writer
	M sauceignore.Matcher
}

func NewWriter(name string, matcher sauceignore.Matcher) (Writer, error) {
	file, err := os.Create(name)
	if err != nil {
		return Writer{}, err
	}

	return Writer{W: tar.NewWriter(file), M: matcher}, nil
}

func (w *Writer) Add(src string) error {
	walker := func(file string, fileInfo os.FileInfo, err error) error {
		if strings.Contains(file, "example.test.js") {
			return nil
		}

		header, err := tar.FileInfoHeader(fileInfo, file)

		relFilePath := file
		if filepath.IsAbs(src) {
			relFilePath, err = filepath.Rel(src, file)
			if err != nil {
				return err
			}
		}

		header.Name = relFilePath

		if err := w.W.WriteHeader(header); err != nil {
			return err
		}

		if fileInfo.Mode().IsDir() {
			return nil
		}

		srcFile, err := os.Open(file)
		defer srcFile.Close()

		_, err = io.Copy(w.W, srcFile)
		if err != nil {
			return err
		}

		return nil
	}

	if err := filepath.Walk(src, walker); err != nil {
		return err
	}

	return nil
}

func (w *Writer) Close() error {
	return w.W.Close()
}
