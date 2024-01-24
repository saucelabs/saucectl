package imagerunner

import "fmt"

type AsyncEventSetupError struct {
	Err error
}

func (e AsyncEventSetupError) Error() string {
	return fmt.Sprintf("streaming setup failed with: %v", e.Err)
}

type AsyncEventFatalError struct {
	Err error
}

func (e AsyncEventFatalError) Error() string {
	return e.Err.Error()
}
