package fleet

import "context"

// TestSuite represents the user's test suite.
type TestSuite struct {
	Name      string   `json:"name"`
	TestFiles []string `json:"testFiles"`
}

// Manager is the management interface for a fleet.
type Manager interface {
	CreateFleet(ctx context.Context, buildID string, testSuites []TestSuite) (string, error)
	NextAssignment(ctx context.Context, fleetID, suiteName string) (string, error)
}
