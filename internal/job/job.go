package job

import (
	"context"
	"time"
)

// Source represents the origin of a job.
type Source string

const (
	SourceAny Source = ""    // Unknown origin.
	SourceVDC Source = "vdc" // Virtual Device Cloud
	SourceRDC Source = "rdc" // Real Device Cloud
	SourceAPI Source = "api" // API Fortress
)

// The different states that a job can be in.
const (
	StateNew        = "new"
	StateQueued     = "queued"
	StateInProgress = "in progress"
	StateComplete   = "complete"
	StateError      = "error"
	StateUnknown    = "?"
)

// The following states are only used by RDC.
const (
	StatePassed = "passed"
	StateFailed = "failed"
)

var AllStates = []string{StatePassed, StateComplete, StateFailed, StateError, StateInProgress, StateQueued}

// DoneStates represents states that a job doesn't transition out of, i.e. once the job is in one of these states,
// it's done.
var DoneStates = []string{StateComplete, StateError, StatePassed, StateFailed}

// Job represents test details and metadata of a test run (aka Job), that is
// usually associated with a particular test execution instance (e.g. VM).
type Job struct {
	ID     string
	Name   string
	Passed bool
	Status string
	Error  string

	BrowserName    string
	BrowserVersion string

	DeviceName string

	Framework string

	OS        string
	OSVersion string

	// IsRDC flags a job started as an RDC run.
	IsRDC bool

	// TimedOut flags a job as an unfinished one.
	TimedOut bool
}

// TotalStatus returns the total status of a job, combining the result of fields Status + Passed.
func (j Job) TotalStatus() string {
	if Done(j.Status) {
		if j.Passed {
			return StatePassed
		}
		return StateFailed
	}

	return j.Status
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

// Service represents the interface for Job interactions.
type Service interface {
	// StartJob starts a new Job.
	StartJob(ctx context.Context, opts StartOptions) (jobID string, err error)

	// StopJob stops a running Job.
	StopJob(ctx context.Context, jobID string, realDevice bool) (Job, error)

	// ReadJob returns the job details.
	ReadJob(ctx context.Context, id string, realDevice bool) (Job, error)

	// PollJob polls job details at an interval, until timeout has been reached or until the job has ended, whether successfully or due to an error.
	PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (Job, error)

	// GetJobAssetFileNames returns all assets files available.
	GetJobAssetFileNames(ctx context.Context, jobID string, realDevice bool) ([]string, error)

	// GetJobAssetFileContent returns the job asset file content.
	GetJobAssetFileContent(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error)

	// UploadAsset uploads an asset to the job that matches the given jobID.
	UploadAsset(jobID string, realDevice bool, fileName string, contentType string, content []byte) error

	// DownloadArtifacts downloads artifacts from a job. Returns a list of
	// file paths.
	DownloadArtifacts(job Job, isLastAttempt bool) []string
}
