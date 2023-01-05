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
}

// NewArtifactService returns an artifact service
func NewArtifactService(restoClient resto.Client, rdcClient rdc.Client, testcompClient testcomposer.Client) *ArtifactService {
	return &ArtifactService{
		JobService: JobService{
			VDCReader: &restoClient,
			RDCReader: &rdcClient,
			VDCWriter: &testcompClient,
		},
	}
}

// List returns a artifact list
func (s *ArtifactService) List(jobID string, isRDC bool) (artifacts.List, error) {
	items, err := s.GetJobAssetFileNames(context.Background(), jobID, isRDC)
	if err != nil {
		return artifacts.List{}, err
	}
	return artifacts.List{
		Items: items,
	}, nil
}

// Download does download specified artifacts
func (s *ArtifactService) Download(jobID, filename string, isRDC bool) ([]byte, error) {
	return s.GetJobAssetFileContent(context.Background(), jobID, filename, isRDC)
}

// Upload does upload the specified artifact
func (s *ArtifactService) Upload(jobID, filename string, isRDC bool, content []byte) error {
	return s.UploadAsset(jobID, isRDC, filename, "text/plain", content)
}
