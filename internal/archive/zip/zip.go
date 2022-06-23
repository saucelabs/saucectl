package zip

import (
	"archive/zip"
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
	W *zip.Writer
	M sauceignore.Matcher
}

// NewFileWriter returns a new Writer that archives files to name.
func NewFileWriter(name string, matcher sauceignore.Matcher) (Writer, error) {
	f, err := os.Create(name)
	if err != nil {
		return Writer{}, err
	}

	w := Writer{W: zip.NewWriter(f), M: matcher}

	return w, nil
}

// New returns a new Writer that archives files to the specified io.Writer.
func New(f io.Writer, matcher sauceignore.Matcher) (Writer, error) {
	w := Writer{W: zip.NewWriter(f), M: matcher}
	return w, nil
}

// Add adds the file at src to the destination dst in the archive and returns a count of
// the files added to the archive.
func (w *Writer) Add(src, dst string) (int, error) {
	finfo, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	// Only will be applied if we have .sauceignore file and have patterns to exclude files and folders
	if w.M.Match(strings.Split(src, string(os.PathSeparator)), finfo.IsDir()) {
		return 0, nil
	}
	log.Debug().Str("name", src).Msg("Adding to archive")

	if !finfo.IsDir() {
		w, err := w.W.Create(path.Join(dst, finfo.Name()))
		if err != nil {
			return 0, err
		}
		f, err := os.Open(src)
		if err != nil {
			return 0, err
		}
		defer f.Close()

		_, err = io.Copy(w, f)
		return 1, err
	}

	files, err := os.ReadDir(src)
	if err != nil {
		return 0, err
	}

	totalFileCount := 0
	for _, f := range files {
		base := filepath.Base(src)
		rebase := path.Join(dst, base)
		fpath := filepath.Join(src, f.Name())
		fileCount, err := w.Add(fpath, rebase)
		if err != nil {
			return 0, err
		}

		totalFileCount += fileCount
	}

	return totalFileCount, nil
}

// Close closes the archive. Adding more files to the archive is not possible after this.
func (w *Writer) Close() error {
	return w.W.Close()
}
