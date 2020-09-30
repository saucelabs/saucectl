package fleet

import (
	"context"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/internal/fpath"
)

// TestSuite represents the user's test suite.
type TestSuite struct {
	Name      string   `json:"name"`
	TestFiles []string `json:"testFiles"`
}

// Sequencer is the interface for test sequencing.
type Sequencer interface {
	Register(ctx context.Context, buildID string, testSuites []TestSuite) (string, error)
	NextAssignment(ctx context.Context, fleetID, suiteName string) (string, error)
}

// Register is a convenience function for Sequencer.Register().
func Register(ctx context.Context, seq Sequencer, buildID string, testFiles []string, suites []config.Suite) (string, error) {
	ts := make([]TestSuite, len(suites))

	for i, suite := range suites {
		files, err := fpath.Walk(testFiles, suite.Match)
		if err != nil {
			return "", err
		}
		ts[i] = TestSuite{
			Name:      suite.Name,
			TestFiles: files,
		}
	}

	return seq.Register(ctx, buildID, ts)
}
