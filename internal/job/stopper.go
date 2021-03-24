package job

import "context"

type Stopper interface {
	StopJob(ctx context.Context, jobID string) error
}
