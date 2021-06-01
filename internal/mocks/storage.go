package mocks

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/storage"
)

// FakeProjectUploader mock struct
type FakeProjectUploader struct {
	UploadSuccess bool
}

// Upload mock function
func (fpu *FakeProjectUploader) Upload(name string) (storage.ArtifactMeta, error) {
	if fpu.UploadSuccess {
		return storage.ArtifactMeta{
			ID: "fake-id",
		}, nil
	}
	return storage.ArtifactMeta{}, errors.New("failed-upload")
}

// Locate mock function
func (fpu *FakeProjectUploader) Locate(hash string) (storage.ArtifactMeta, error) {
	return storage.ArtifactMeta{}, nil
}
