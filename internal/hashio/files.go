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
func HashContent(filePath string, extraContent ...string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	buffer := make([]byte, fileInfo.Size())
	if _, err := file.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	combinedContent := string(buffer) + strings.Join(extraContent, "")

	hash := sha256.Sum256([]byte(combinedContent))
	return fmt.Sprintf("%x", hash)[:15], nil
}
