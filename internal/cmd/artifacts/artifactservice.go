package artifacts

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/saucelabs/saucectl/internal/saucecloud"
)

// ArtifactService represents artifact service
type ArtifactService struct {
	JobService saucecloud.JobService
}

// List returns an artifact list
func (s *ArtifactService) List(jobID string) (List, error) {
	isRDC, err := s.isRDC(jobID)
	if err != nil {
		return List{}, err
	}
	items, err := s.JobService.GetJobAssetFileNames(
		context.Background(), jobID, isRDC,
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
func (s *ArtifactService) Download(jobID, filename string) ([]byte, error) {
	isRDC, err := s.isRDC(jobID)
	if err != nil {
		return nil, err
	}

	return s.JobService.GetJobAssetFileContent(
		context.Background(), jobID, filename, isRDC,
	)
}

// Upload does upload the specified artifact
func (s *ArtifactService) Upload(jobID, filename string, content []byte) error {
	isRDC, err := s.isRDC(jobID)
	if err != nil {
		return err
	}
	if isRDC {
		return errors.New("uploading file to Real Device job is not supported")
	}

	return s.JobService.UploadAsset(
		jobID, false, filename, http.DetectContentType(content), content,
	)
}

func (s *ArtifactService) isRDC(jobID string) (bool, error) {
	_, err := s.JobService.ReadJob(context.Background(), jobID, false)
	if err != nil {
		_, err = s.JobService.ReadJob(context.Background(), jobID, true)
		if err != nil {
			return false, fmt.Errorf("failed to get the job: %w", err)
		}
		return true, nil
	}

	return false, nil
}
