package progress

import (
	"fmt"
	"io"
	"time"

	"github.com/briandowns/spinner"
	"github.com/schollz/progressbar/v3"
)

var spinnerSpeed = 1 * time.Second
var spinnerInstance = spinner.New(spinner.CharSets[14], spinnerSpeed)

// Show starts showing a progress spinner.
func Show(text string, args ...interface{}) *spinner.Spinner {
	message := " " + fmt.Sprintf(text, args...)
	spinnerInstance.Suffix = message
	spinnerInstance.Stop()
	spinnerInstance.Start()
	return spinnerInstance
}

// Stop stops the progress spinner.
func Stop() {
	spinnerInstance.Stop()
}

// ReadSeeker is a wrapper around io.ReadSeeker that updates a progress bar.
type ReadSeeker struct {
	io.ReadSeeker
	bar *progressbar.ProgressBar
}

// NewReadSeeker returns a new ReadSeeker with the given bar.
func NewReadSeeker(r io.ReadSeeker, bar *progressbar.ProgressBar) ReadSeeker {
	return ReadSeeker{
		ReadSeeker: r,
		bar:        bar,
	}
}

func (r *ReadSeeker) Seek(offset int64, whence int) (int64, error) {
	// Resetting the progress bar is a rather buggy operation at the time of
	// writing. You just end up with multiple bars. Ignore the
	// seek operation in regard to the progress bar.
	return r.ReadSeeker.Seek(offset, whence)
}

func (r *ReadSeeker) Read(p []byte) (n int, err error) {
	n, err = r.ReadSeeker.Read(p)
	_ = r.bar.Add(n)
	return
}

func (r *ReadSeeker) Close() (err error) {
	if closer, ok := r.ReadSeeker.(io.Closer); ok {
		return closer.Close()
	}
	_ = r.bar.Finish()
	return
}
