package imagerunner

import "fmt"

// AsyncEventSetupError represents an error that occurs during the setup of the
// asynchronous event handling process.
// This error indicates that the setup process failed and may need to be retried
// or debugged.
type AsyncEventSetupError struct {
	Err error
}

func (e AsyncEventSetupError) Error() string {
	return fmt.Sprintf("streaming setup failed with: %v", e.Err)
}

// AsyncEventFatalError represents an error that occurs during the asynchronous
// event handling process.
// This error is considered fatal, meaning it cannot be recovered from.
type AsyncEventFatalError struct {
	Err error
}

func (e AsyncEventFatalError) Error() string {
	return e.Err.Error()
}
