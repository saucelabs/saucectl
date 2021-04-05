package errors

import (
	"github.com/getsentry/sentry-go"
	"time"
)

// Handle capture errrors with sentry
func Handle(err error) {
	sentry.CaptureException(err)
}

// HandleAndFlush capture errors and flush
func HandleAndFlush(err error) {
	Handle(err)
	flush()
}

func flush () {
	// Flush buffered events before the program terminates.
	// Set the timeout to the maximum duration the program can afford to wait.
	defer sentry.Flush(2 * time.Second)
}
