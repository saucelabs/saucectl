package archive

import (
	"io"
)

// Writer represents an archive writer.
type Writer interface {
	io.Closer

	// Add adds the src file to the destination dst in the archive.
	Add(src, dst string) error
}
