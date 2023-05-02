package hashio

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// SHA256 hashes the given file with crypto.SHA256 and returns the checksum as a
// base-16 (hex) string.
func SHA256(filename string) (string, error) {
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
