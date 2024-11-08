package hashio

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
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

// HashContent computes a SHA-256 hash of the file content combined with extra content,
// and returns the first 16 characters of the hex-encoded hash.
func HashContent(filename string, extra ...string) (string, error) {
	h := sha256.New()

	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	h.Write([]byte(strings.Join(extra, "")))

	return fmt.Sprintf("%x", h.Sum(nil))[:16], nil
}
