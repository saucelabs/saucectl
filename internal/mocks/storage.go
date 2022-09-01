package mocks

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/storage"
	"io"
)

// FakeProjectUploader mock struct
type FakeProjectUploader struct {
	UploadSuccess bool
}

func (fpu *FakeProjectUploader) UploadStream(filename string, reader io.Reader) (storage.Item, error) {
	panic("not implemented")
}

func (fpu *FakeProjectUploader) Download(id string) (io.ReadCloser, int64, error) {
	panic("not implemented")
}

func (fpu *FakeProjectUploader) List(opts storage.ListOptions) (storage.List, error) {
	panic("not implemented")
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
