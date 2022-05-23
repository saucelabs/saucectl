package job

import "context"

// Stopper is the interface for stopping jobs.
type Stopper interface {
	StopJob(ctx context.Context, jobID string, realDevice bool) (Job, error)
}
