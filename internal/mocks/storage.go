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
func (fpu *FakeProjectUploader) Upload(name string) (storage.Item, error) {
	if fpu.UploadSuccess {
		return storage.Item{
			ID: "fake-id",
		}, nil
	}
	return storage.Item{}, errors.New("failed-upload")
}

// Find mock function
func (fpu *FakeProjectUploader) Find(hash string) (storage.Item, error) {
	return storage.Item{}, nil
}
