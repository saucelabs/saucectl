package saucecloud

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"

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
	source, err := s.GetSource(jobID)
	if err != nil {
		return artifacts.List{}, err
	}
	items, err := s.GetJobAssetFileNames(context.Background(), jobID, source == artifacts.RDCSource)
	if err != nil {
		return artifacts.List{}, err
	}
	return artifacts.List{
		JobID: jobID,
		Items: items,
	}, nil
}

// Download does download specified artifacts
func (s *ArtifactService) Download(jobID, targetDir, filename string) error {
	source, err := s.GetSource(jobID)
	if err != nil {
		return err
	}

	body, err := s.GetJobAssetFileContent(context.Background(), jobID, filename, source == artifacts.RDCSource)
	if err != nil {
		return err
	}
	if targetDir != "" {
		if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create target dir: %w", err)
		}
		filename = path.Join(targetDir, filename)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	_, err = file.Write(body)
	if err != nil {
		return fmt.Errorf("failed to write to the file: %w", err)
	}

	return file.Close()
}

// Upload does upload the specified artifact
func (s *ArtifactService) Upload(jobID, filename string, content []byte) error {
	source, err := s.GetSource(jobID)
	if err != nil {
		return err
	}
	if source == artifacts.RDCSource {
		return errors.New("uploading file to Real Device job is not supported")
	}

	return s.UploadAsset(jobID, false, filename, http.DetectContentType(content), content)
}

func (s *ArtifactService) GetSource(ID string) (string, error) {
	var source = artifacts.VDCSource

	switch source {
	case artifacts.VDCSource:
		_, err := s.ReadJob(context.Background(), ID, false)
		if err == nil {
			return artifacts.VDCSource, nil
		}
		fallthrough
	case artifacts.RDCSource:
		_, err := s.ReadJob(context.Background(), ID, true)
		if err == nil {
			return artifacts.RDCSource, nil
		}
		fallthrough
	case artifacts.HTOSource:
		_, err := s.RunnerService.GetStatus(context.Background(), ID)
		if err == nil {
			return artifacts.HTOSource, nil
		}
	}

	return "", fmt.Errorf("job not found")
}

func (s *ArtifactService) HtoDownload(runID, pattern, targetDir string) (artifacts.List, error) {
	reader, err := s.RunnerService.DownloadArtifacts(context.Background(), runID)
	if err != nil {
		return artifacts.List{}, fmt.Errorf("failed to fetch artifacts: %w", err)
	}

	fileName, err := files.SaveToTempFile(reader)
	if err != nil {
		return artifacts.List{}, fmt.Errorf("failed to download artifacts content: %w", err)
	}
	defer os.Remove(fileName)

	zf, err := zip.OpenReader(fileName)
	if err != nil {
		return artifacts.List{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer zf.Close()

	files := []string{}
	for _, f := range zf.File {
		if glob.Glob(pattern, f.Name) {
			files = append(files, f.Name)
			if err = szip.Extract(targetDir, f); err != nil {
				return artifacts.List{}, fmt.Errorf("failed to extract file: %w", err)
			}
		}
	}
	return artifacts.List{
		RunID: runID,
		Items: files,
	}, nil
}
