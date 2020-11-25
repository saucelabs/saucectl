package zip

import (
	"archive/zip"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

// Writer is a wrapper around zip.Writer and implements zip archiving for archive.Writer.
type Writer struct {
	W *zip.Writer
}

// NewWrite returns a new Writer that archives files to name.
func NewWriter(name string) (Writer, error) {
	f, err := os.Create(name)
	if err != nil {
		return Writer{}, err
	}

	w := Writer{W: zip.NewWriter(f)}

	return w, nil
}

// Add adds the file at src to the destination dst in the archive.
func (w *Writer) Add(src, dst string) error {
	finfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !finfo.IsDir() {
		w, err := w.W.Create(path.Join(dst, finfo.Name()))
		if err != nil {
			return err
		}
		b, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}

	files, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, f := range files {
		base := filepath.Base(src)
		rebase := path.Join(dst, base)
		fpath := filepath.Join(src, f.Name())
		if err := w.Add(fpath, rebase); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the archive. Adding more files to the archive is not possible after this.
func (w *Writer) Close() error {
	return w.W.Close()
}
