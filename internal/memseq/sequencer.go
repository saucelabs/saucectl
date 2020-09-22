package memseq

import (
	"context"
	"github.com/saucelabs/saucectl/internal/fleet"
)

// Sequencer represents a local, standalone test sequencer.
type Sequencer struct {
	sequence map[string]chan string
}

// Register registers the fleet.
func (m *Sequencer) Register(ctx context.Context, buildID string, testSuites []fleet.TestSuite) (string, error) {
	m.sequence = make(map[string]chan string, len(testSuites))

	for _, suite := range testSuites {
		key := buildID + suite.Name
		m.sequence[key] = make(chan string, len(suite.TestFiles))

		for _, tf := range suite.TestFiles {
			m.sequence[key] <- tf
		}
	}

	// Just reuse the buildID as a fleetID
	return buildID, nil
}

// NextAssignment fetches the next test assignment. Returns an empty string if no assignments left.
func (m *Sequencer) NextAssignment(ctx context.Context, fleetID, suiteName string) (string, error) {
	select {
	case next := <-m.sequence[fleetID + suiteName]:
		return next, nil
	default:
		return "", nil
	}
}
