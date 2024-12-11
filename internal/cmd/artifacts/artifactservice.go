package artifacts

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/saucelabs/saucectl/internal/job"
)

// ArtifactService represents artifact service
type ArtifactService struct {
	JobService job.Service
}

// List returns an artifact list
func (s *ArtifactService) List(ctx context.Context, jobID string) (List, error) {
	isRDC, err := s.isRDC(ctx, jobID)
	if err != nil {
		return List{}, err
	}
	items, err := s.JobService.ArtifactNames(
		ctx, jobID, isRDC,
	)
	if err != nil {
		return List{}, err
	}
	return List{
		JobID: jobID,
		Items: items,
	}, nil
}

// Download does download specified artifacts
func (s *ArtifactService) Download(ctx context.Context, jobID, filename string) ([]byte, error) {
	isRDC, err := s.isRDC(ctx, jobID)
	if err != nil {
		return nil, err
	}

	return s.JobService.Artifact(
		ctx, jobID, filename, isRDC,
	)
}

// Upload does upload the specified artifact
func (s *ArtifactService) Upload(ctx context.Context, jobID, filename string, content []byte) error {
	isRDC, err := s.isRDC(ctx, jobID)
	if err != nil {
		return err
	}
	if isRDC {
		return errors.New("uploading file to Real Device job is not supported")
	}

	return s.JobService.UploadArtifact(ctx, jobID, false, filename, http.DetectContentType(content), content)
}

func (s *ArtifactService) isRDC(ctx context.Context, jobID string) (bool, error) {
	_, err := s.JobService.Job(ctx, jobID, false)
	if err != nil {
		_, err = s.JobService.Job(ctx, jobID, true)
		if err != nil {
			return false, fmt.Errorf("failed to get the job: %w", err)
		}
		return true, nil
	}

	return false, nil
}
