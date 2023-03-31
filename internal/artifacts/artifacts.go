package artifacts

// List represents artifact structure
type List struct {
	JobID string   `json:"jobID"`
	Items []string `json:"items"`
}

// Service is the interface for interacting with artifacts
type Service interface {
	List(jobID string) (List, error)
	Download(jobID, filename string) ([]byte, error)
	Upload(jobID, filename string, content []byte) error
	HtoDownload(runID, pattern, targetDir string) error
}
