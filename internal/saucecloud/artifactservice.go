package saucecloud

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/ryanuber/go-glob"
	szip "github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/artifacts"
	"github.com/saucelabs/saucectl/internal/files"
	"github.com/saucelabs/saucectl/internal/job"
)

// ArtifactService represents artifact service
type ArtifactService struct {
	JobService
	RunnerService ImageRunner
}

// NewArtifactService returns an artifact service
func NewArtifactService(vdcReader job.Reader, rdcReader job.Reader, vdcWriter job.Writer, imgRunner ImageRunner) *ArtifactService {
	return &ArtifactService{
		JobService: JobService{
			VDCReader: vdcReader,
			RDCReader: rdcReader,
			VDCWriter: vdcWriter,
		},
		RunnerService: imgRunner,
	}
}

// List returns an artifact list
func (s *ArtifactService) List(jobID string) (artifacts.List, error) {
	isRDC, err := s.isRDC(jobID)
	if err != nil {
		return artifacts.List{}, err
	}
	items, err := s.GetJobAssetFileNames(context.Background(), jobID, isRDC)
	if err != nil {
		return artifacts.List{}, err
	}
	return artifacts.List{
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

	return s.GetJobAssetFileContent(context.Background(), jobID, filename, isRDC)
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

	return s.UploadAsset(jobID, false, filename, http.DetectContentType(content), content)
}

func (s *ArtifactService) isRDC(jobID string) (bool, error) {
	_, err := s.ReadJob(context.Background(), jobID, false)
	if err != nil {
		_, err = s.ReadJob(context.Background(), jobID, true)
		if err != nil {
			return false, fmt.Errorf("failed to get the job: %w", err)
		}
		return true, nil
	}

	return false, nil
}

func (s *ArtifactService) HtoDownload(runnerID, pattern, targetDir string) ([]string, error) {
	reader, err := s.RunnerService.DownloadArtifacts(context.Background(), runnerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artifacts: %w", err)
	}

	fileName, err := files.SaveToTempFile(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to download artifacts content: %w", err)
	}
	defer os.Remove(fileName)

	zf, err := zip.OpenReader(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer zf.Close()

	files := []string{}
	for _, f := range zf.File {
		if glob.Glob(pattern, f.Name) {
			files = append(files, f.Name)
			if err = szip.Extract(targetDir, f); err != nil {
				return nil, fmt.Errorf("failed to extract file: %w", err)
			}
		}
	}
	return files, nil
}
