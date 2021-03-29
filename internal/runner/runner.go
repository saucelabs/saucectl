package runner

// Testrunner describes the test runner interface
type Testrunner interface {
	RunProject() (int, error)
}
