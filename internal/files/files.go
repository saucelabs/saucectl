package files

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// NewSHA256 hashes the given file using crypto.SHA256 and returns the resulting string.
func NewSHA256(filename string) (string, error) {
	h := sha256.New()
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
