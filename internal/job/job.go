package job

// Job represents test details and metadata of a test run (aka Job), that is usually associated with a particular test
// execution instance (e.g. VM).
type Job struct {
	ID     string `json:"id"`
	Passed bool   `json:"passed"`
	Status string `json:"status"`
	Error  string `json:"error"`
}
