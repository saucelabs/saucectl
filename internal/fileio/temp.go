package fileio

import (
	"io"
	"os"
)

// CreateTemp writes out the contents of the reader to a temp file.
// It's the caller's responsibility to clean up the temp file.
func CreateTemp(r io.Reader) (string, error) {
	fd, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer fd.Close()

	if _, err := io.Copy(fd, r); err != nil {
		return "", err
	}

	return fd.Name(), fd.Close()
}
