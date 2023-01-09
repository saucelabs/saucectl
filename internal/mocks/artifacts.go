package mocks

import "github.com/saucelabs/saucectl/internal/artifacts"

// FakeArtifactService is the mocked struct
type FakeArtifactService struct {
}

// List returns artifact list
func (s *FakeArtifactService) List(jobID string, isRDC bool) (artifacts.List, error) {
	return artifacts.List{}, nil
}

// Download does download specified artifact
func (s *FakeArtifactService) Download(jobID, filename string, isRDC bool) ([]byte, error) {
	return []byte{}, nil
}

// Upload does upload specified artifact
func (s *FakeArtifactService) Upload(jobID, filename string, isRDC bool, content []byte) error {
	return nil
}
