package artifacts

// List represents artifact structure
type List struct {
	JobID string   `json:"jobID"`
	IsRDC bool     `json:"realDevice"`
	Items []string `json:"items"`
}

// Service is the interface for interacting with artifacts
type Service interface {
	List() (List, error)
	Download(filename string) ([]byte, error)
	Upload(filename string, content []byte) error
}
