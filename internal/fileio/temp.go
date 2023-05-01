package fileio

import (
	"io"
	"os"
)

// SaveToTempFile writes out the contents of the reader to a temp file.
// It's the caller's responsibility to clean up the temp file.
func SaveToTempFile(closer io.ReadCloser) (string, error) {
	defer closer.Close()
	fd, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer fd.Close()

	_, err = io.Copy(fd, closer)
	if err != nil {
		return "", err
	}
	return fd.Name(), fd.Close()
}
