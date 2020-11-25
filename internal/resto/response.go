package resto

// Details represent API response regarding job.
type Details struct {
	ID     string `json:"id"`
	Passed bool   `json:"passed"`
	Status string `json:"status"`
	Error  string `json:"error"`
}
