package report

import "time"

// TestResult represents the test result.
type TestResult struct {
	Name          string        `json:"name"`
	Duration      time.Duration `json:"duration"`
	StartTime     time.Time     `json:"startTime"`
	EndTime       time.Time     `json:"endTime"`
	Status        string        `json:"status"`
	Browser       string        `json:"browser"`
	Platform      string        `json:"platform"`
	DeviceName    string        `json:"deviceName"`
	URL           string        `json:"url"`
	Artifacts     []Artifact    `json:"artifacts"`
	Origin        string        `json:"origin"`
	Attempts      int           `json:"attempts"`
	RDC           bool          `json:"-"`
	TimedOut      bool          `json:"-"`
	PassThreshold bool          `json:"passThreshold"`
}

// ArtifactType represents the type of assets (e.g. a junit report). Semantically similar to Content-Type.
type ArtifactType int

const (
	// JUnitArtifact represents the junit artifact type (https://llg.cubic.org/docs/junit/).
	JUnitArtifact ArtifactType = iota
	// JSONArtifact represents the json artifact type
	JSONArtifact
)

// Artifact represents an artifact (aka asset) that was generated as part of a job.
type Artifact struct {
	FilePath  string       `json:"filePath,omitempty"`
	AssetType ArtifactType `json:"-"`
	Body      []byte       `json:"-"`

	// Error contains optional error information in case the artifact was not retrieved.
	Error error `json:"-"`
}

// Reporter is the interface for rest result reporting.
type Reporter interface {
	// Add adds the TestResult to the reporter. TestResults added this way can then be rendered out by calling Render().
	Add(t TestResult)
	// Render renders the test results. The destination depends on the implementation.
	Render()
	// Reset resets the state of the reporter (e.g. remove any previously reported TestResults).
	Reset()
	// ArtifactRequirements returns a list of artifact types that this reporter requires to create a proper report.
	ArtifactRequirements() []ArtifactType
}

// IsArtifactRequired traverses the list of reporters and validates their requirements against the given artifact type.
// Returns true if at least one of the reporters has a matching requirement.
func IsArtifactRequired(reps []Reporter, at ArtifactType) bool {
	for _, r := range reps {
		for _, ar := range r.ArtifactRequirements() {
			if ar == at {
				return true
			}
		}
	}

	return false
}
