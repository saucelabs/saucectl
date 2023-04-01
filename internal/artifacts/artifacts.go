package artifacts

const (
	RDCSource = "rdc"
	VDCSource = "vdc"
	HTOSource = "hto"
)

// List represents artifact structure
type List struct {
	JobID string   `json:"jobID,omitempty"`
	RunID string   `json:"runID,omitempty"`
	Items []string `json:"items"`
}

// Service is the interface for interacting with artifacts
type Service interface {
	List(jobID string) (List, error)
	Download(jobID, targetDir, filename string) error
	Upload(jobID, filename string, content []byte) error
	GetSource(ID string) (string, error)
	HtoDownload(ID, pattern, targetDir string) (List, error)
}
