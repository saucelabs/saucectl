package zip

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/sauceignore"
)

// Writer is a wrapper around zip.Writer and implements zip archiving for archive.Writer.
type Writer struct {
	W       *zip.Writer
	M       sauceignore.Matcher
	ZipFile *os.File
}

// NewFileWriter returns a new Writer that archives files to name.
func NewFileWriter(name string, matcher sauceignore.Matcher) (Writer, error) {
	f, err := os.Create(name)
	if err != nil {
		return Writer{}, err
	}

	w := Writer{W: zip.NewWriter(f), M: matcher, ZipFile: f}

	return w, nil
}

// New returns a new Writer that archives files to the specified io.Writer.
func New(f io.Writer, matcher sauceignore.Matcher) (Writer, error) {
	w := Writer{W: zip.NewWriter(f), M: matcher}
	return w, nil
}

// Add adds the file at src to the destination dst in the archive and returns a count of
// the files added to the archive, as well the length of the longest path.
func (w *Writer) Add(src, dst string) (count int, length int, err error) {
	finfo, err := os.Stat(src)
	if err != nil {
		return 0, 0, err
	}

	// Only will be applied if we have .sauceignore file and have patterns to exclude files and folders
	if w.M.Match(strings.Split(src, string(os.PathSeparator)), finfo.IsDir()) {
		return 0, 0, nil
	}

	if !finfo.IsDir() {
		log.Debug().Str("name", src).Msg("Adding to archive")
		target := path.Join(dst, finfo.Name())
		w, err := w.W.Create(target)
		if err != nil {
			return 0, 0, err
		}
		f, err := os.Open(src)
		if err != nil {
			return 0, 0, err
		}

		if _, err := io.Copy(w, f); err != nil {
			return 0, 0, err
		}

		if err := f.Close(); err != nil {
			return 0, 0, err
		}

		return 1, len(target), err
	}

	files, err := os.ReadDir(src)
	if err != nil {
		return 0, 0, err
	}

	base := filepath.Base(src)
	relBase := filepath.Join(dst, base)
	_, err = w.W.Create(fmt.Sprintf("%s/", relBase))
	if err != nil {
		return 0, 0, err
	}
	count++

	for _, f := range files {
		rebase := path.Join(dst, base)
		fpath := filepath.Join(src, f.Name())
		fileCount, pathLength, err := w.Add(fpath, rebase)
		if err != nil {
			return 0, 0, err
		}

		count += fileCount
		if pathLength > length {
			length = pathLength
		}
	}

	return count, length, nil
}

// Close closes the archive. Adding more files to the archive is not possible after this.
func (w *Writer) Close() error {
	if err := w.W.Close(); err != nil {
		return err
	}
	return w.ZipFile.Close()
}

func Extract(targetDir string, file *zip.File) error {
	fullPath := path.Join(targetDir, file.Name)

	relPath, err := filepath.Rel(targetDir, fullPath)
	if err != nil {
		return err
	}
	if strings.Contains(relPath, "..") {
		return fmt.Errorf("file %s is relative to an outside folder", file.Name)
	}

	folder := path.Dir(fullPath)
	if err := os.MkdirAll(folder, 0755); err != nil {
		return err
	}

	fd, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	rd, err := file.Open()
	if err != nil {
		return err
	}
	defer rd.Close()

	_, err = io.Copy(fd, rd)
	if err != nil {
		return err
	}
	return fd.Close()
}
