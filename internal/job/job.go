package job

// The different states that a job can be in.
const (
	StateNew        = "new"
	StateQueued     = "queued"
	StateInProgress = "in progress"
	StateComplete   = "complete"
	StateError      = "error"
)

// DoneStates represents states that a job doesn't transition out of, i.e. once the job is in one of these states,
// it's done.
var DoneStates = []string{StateComplete, StateError}

// Job represents test details and metadata of a test run (aka Job), that is usually associated with a particular test
// execution instance (e.g. VM).
type Job struct {
	ID     string `json:"id"`
	Passed bool   `json:"passed"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// Done returns true if the job status is one of DoneStates. False otherwise.
func Done(status string) bool {
	for _, s := range DoneStates {
		if s == status {
			return true
		}
	}

	return false
}
