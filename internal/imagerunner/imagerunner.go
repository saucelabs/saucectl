package imagerunner

import "errors"

// The different states that a runner can be in.
const (
	StateUnknown    = "Unknown"
	StatePending    = "Pending"
	StateRunning    = "Running"
	StateUploading  = "Uploading"
	StateSucceeded  = "Succeeded"
	StateCancelled  = "Cancelled"
	StateFailed     = "Failed"
	StateTerminated = "Terminated"
)

// DoneStates represents states that a runner doesn't transition out of, i.e. once the runner is in one of these states,
// it's done.
var DoneStates = []string{StateSucceeded, StateCancelled, StateFailed, StateTerminated}

// Done returns true if the runner status is one of DoneStates. False otherwise.
func Done(status string) bool {
	for _, s := range DoneStates {
		if s == status {
			return true
		}
	}

	return false
}

var ErrResourceNotFound = errors.New("resource not found")

type RunnerSpec struct {
	Container    Container         `json:"container,omitempty"`
	EntryPoint   string            `json:"entrypoint,omitempty"`
	Env          []EnvItem         `json:"env,omitempty"`
	Files        []FileData        `json:"files,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Artifacts    []string          `json:"artifacts,omitempty"`
	WorkloadType string            `json:"workloadType,omitempty"`
	Tunnel       *Tunnel           `json:"tunnel,omitempty"`
}

type Tunnel struct {
	Name  string `json:"name,omitempty"`
	Owner string `json:"owner,omitempty"`
}

type Container struct {
	Name string `json:"name,omitempty"`
	Auth *Auth  `json:"auth,omitempty"`
}

type Auth struct {
	User  string `json:"user,omitempty"`
	Token string `json:"token,omitempty"`
}

type EnvItem struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type FileData struct {
	Path string `json:"path,omitempty"`
	Data string `json:"data,omitempty"`
}

type Runner struct {
	ID                string `json:"id,omitempty"`
	Status            string `json:"status,omitempty"`
	Image             string `json:"image,omitempty"`
	CreationTime      int64  `json:"creation_time,omitempty"`
	TerminationTime   int64  `json:"termination_time,omitempty"`
	TerminationReason string `json:"termination_reason,omitempty"`
}

type ArtifactList struct {
	ID    string   `json:"id"`
	Items []string `json:"items"`
}
