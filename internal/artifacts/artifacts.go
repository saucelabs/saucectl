package artifacts

// List represents artifact structure
type List struct {
	JobID string   `json:"jobID"`
	Items []string `json:"items"`
}

// Service is the interface for interacting with artifacts
type Service interface {
	List(jobID string, isRDC bool) (List, error)
	Download(jobID, filename string, isRDC bool) ([]byte, error)
	Upload(jobID, filename string, isRDC bool, content []byte) error
}
