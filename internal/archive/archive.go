package archive

import (
	"io"
)

type Writer interface {
	io.Closer

	// Add adds the src file to the destination dst in the archive.
	Add(src, dst string) error
}
