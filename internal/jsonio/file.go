package jsonio

import (
	"encoding/json"
	"fmt"
	"os"
)

// WriteFile encodes v as JSON and saves it as a file at name.
func WriteFile(name string, v interface{}) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = json.NewEncoder(f).Encode(v); err != nil {
		return err
	}

	// Explicit close to ensure that all bytes have been flushed.
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file stream when serializing '%v': %v", name, err)
	}

	return nil
}
