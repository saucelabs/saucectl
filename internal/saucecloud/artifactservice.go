package saucecloud

import (
	"context"

	"github.com/saucelabs/saucectl/internal/artifacts"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/testcomposer"
)

// ArtifactService represents artifact service
type ArtifactService struct {
	JobService
	JobID string
	IsRDC bool
}

// NewArtifactService returns an artifact service
func NewArtifactService(restoClient resto.Client, rdcClient rdc.Client, testcompClient testcomposer.Client, jobID string, isRDC bool) *ArtifactService {
	return &ArtifactService{
		JobService: JobService{
			VDCReader: &restoClient,
			RDCReader: &rdcClient,
			VDCWriter: &testcompClient,
		},
		JobID: jobID,
		IsRDC: isRDC,
	}
}

// List returns a artifact list
func (s *ArtifactService) List() (artifacts.List, error) {
	items, err := s.GetJobAssetFileNames(context.Background(), s.JobID, s.IsRDC)
	if err != nil {
		return artifacts.List{}, err
	}
	return artifacts.List{
		JobID: s.JobID,
		IsRDC: s.IsRDC,
		Items: items,
	}, nil
}

// Download does download specified artifacts
func (s *ArtifactService) Download(filename string) ([]byte, error) {
	return s.GetJobAssetFileContent(context.Background(), s.JobID, filename, s.IsRDC)
}

// Upload does upload the specified artifact
func (s *ArtifactService) Upload(filename string, content []byte) error {
	return s.UploadAsset(s.JobID, s.IsRDC, filename, "text/plain", content)
}
