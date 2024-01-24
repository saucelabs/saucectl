package imagerunner

import "fmt"

type AsyncEventSetupError struct {
	Err error
}

func (e AsyncEventSetupError) Error() string {
	return fmt.Sprintf("streaming setup failed with: %v", e.Err)
}

type NotAuthorizedError struct {
}

func (e NotAuthorizedError) Error() string {
	return "not authorized"
}
