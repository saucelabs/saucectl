package mocks

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/storage"
)

type FakeProjectUploader struct {
	UploadSuccess bool
}

func (fpu *FakeProjectUploader) Upload(name string) (storage.ArtifactMeta, error) {
	if fpu.UploadSuccess {
		return storage.ArtifactMeta{
			ID: "fake-id",
		}, nil
	}
	return storage.ArtifactMeta{}, errors.New("failed-upload")
}
